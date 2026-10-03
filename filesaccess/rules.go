// Copyright (c) 2015-2026 MinIO, Inc.
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package filesaccess

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

// MaxRules is the most rules one export may hold.
const MaxRules = 1000

// Rule grants Access to the clients it names. On the wire it is
// {"clients": [...], "accessType": "rw"}.
type Rule struct {
	Clients []Client
	Access  Access
}

// Equal reports whether r and o name the same clients, in the same order, and
// grant the same access.
func (r Rule) Equal(o Rule) bool {
	return r.Access == o.Access && slices.Equal(r.Clients, o.Clients)
}

// check is the per-rule check: at least one client, none of them zero, and a
// valid access type. Whether a client appears twice is a check on a whole
// list, made by NewRules.
func (r Rule) check() error {
	if len(r.Clients) == 0 {
		return errors.New("a rule names at least one client")
	}
	if slices.ContainsFunc(r.Clients, Client.IsZero) {
		return errors.New("a rule names an empty client specification")
	}
	if r.Access.String() == "" {
		return fmt.Errorf("invalid access type %d", uint8(r.Access))
	}
	return nil
}

// ruleJSON is the wire form of a Rule.
type ruleJSON struct {
	Clients    []Client `json:"clients"`
	AccessType Access   `json:"accessType"`
}

// MarshalJSON encodes the rule with each client in its normalized form. A
// rule that would not pass its own checks is an error.
func (r Rule) MarshalJSON() ([]byte, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	return json.Marshal(ruleJSON{Clients: r.Clients, AccessType: r.Access})
}

// UnmarshalJSON decodes one rule with the per-entry checks: every client
// specification must parse, and the access type must be valid. It reports
// every client that does not parse. An unknown member is refused, because a
// policy field this version does not know would otherwise be dropped
// silently.
func (r *Rule) UnmarshalJSON(b []byte) error {
	var raw struct {
		Clients    []string         `json:"clients"`
		AccessType *json.RawMessage `json:"accessType"`
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return fmt.Errorf("a rule is {\"clients\": [...], \"accessType\": ...}: %w", err)
	}

	var errs []error
	clients := make([]Client, 0, len(raw.Clients))
	for _, s := range raw.Clients {
		c, err := ParseClient(s)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		clients = append(clients, c)
	}
	if len(raw.Clients) == 0 {
		errs = append(errs, errors.New("a rule names at least one client"))
	}

	var access Access
	if raw.AccessType == nil {
		errs = append(errs, errors.New("a rule needs an accessType"))
	} else if err := access.UnmarshalJSON(*raw.AccessType); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	*r = Rule{Clients: clients, Access: access}
	return nil
}

// Rules is an export's access rule list: 1 to MaxRules rules, in evaluation
// order, naming no client specification twice. Specifications compare in
// normalized form, so 10.1.2.7 and 10.1.2.7/32 are the same one.
//
// Only NewRules, Parse and UnmarshalJSON build a Rules, so one that is not
// the zero value always passes these checks. The zero value holds no rules,
// and is refused by MarshalJSON.
type Rules struct {
	rules []Rule
}

// NewRules checks rules as a whole list and returns it. The error names each
// offending rule by its 1-based position. The rules are copied, so a later
// change to the caller's slice does not reach the returned list.
func NewRules(rules []Rule) (Rules, error) {
	var errs []error
	seen := make(map[Client]int)
	for i, rule := range rules {
		if err := rule.check(); err != nil {
			errs = append(errs, fmt.Errorf("rule %d: %w", i+1, err))
			continue
		}
		for _, c := range rule.Clients {
			if first, dup := seen[c]; dup {
				errs = append(errs, fmt.Errorf("rule %d: %q already appears in rule %d; this rule can never match it", i+1, c.String(), first))
				continue
			}
			seen[c] = i + 1
		}
	}
	if err := checkCount(len(rules)); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return Rules{}, errors.Join(errs...)
	}
	return Rules{rules: cloneRules(rules)}, nil
}

// checkCount checks the number of rules in a list.
func checkCount(n int) error {
	switch {
	case n == 0:
		return errors.New("no rules; a rule list holds at least one rule")
	case n > MaxRules:
		return fmt.Errorf("%s rules; at most %s are allowed", thousands(n), thousands(MaxRules))
	}
	return nil
}

// All returns the rules in evaluation order. The slice is a copy.
func (r Rules) All() []Rule {
	return cloneRules(r.rules)
}

// Len returns the number of rules.
func (r Rules) Len() int { return len(r.rules) }

// MarshalJSON encodes the rules as a JSON array, in evaluation order. The
// zero Rules is an error: a list with no rules is not a request anyone means
// to send.
func (r Rules) MarshalJSON() ([]byte, error) {
	if len(r.rules) == 0 {
		return nil, errors.New("no rules; a rule list holds at least one rule")
	}
	return json.Marshal(r.rules)
}

// UnmarshalJSON decodes a JSON array of rules with the checks of NewRules,
// and the per-entry checks of Rule. It reports every error, each naming the
// rule by its 1-based position.
func (r *Rules) UnmarshalJSON(b []byte) error {
	var raws []json.RawMessage
	if err := json.Unmarshal(b, &raws); err != nil {
		return fmt.Errorf("access rules are a JSON array: %w", err)
	}
	var errs []error
	rules := make([]Rule, 0, len(raws))
	for i, raw := range raws {
		var rule Rule
		if err := rule.UnmarshalJSON(raw); err != nil {
			errs = append(errs, prefixEach(fmt.Sprintf("rule %d", i+1), err)...)
			continue
		}
		rules = append(rules, rule)
	}
	if len(errs) > 0 {
		if err := checkCount(len(raws)); err != nil {
			errs = append(errs, err)
		}
		return errors.Join(errs...)
	}
	v, err := NewRules(rules)
	if err != nil {
		return err
	}
	*r = v
	return nil
}

// prefixEach labels err with prefix, and each error inside it separately when
// it joins several, so every line of the message names where it comes from.
func prefixEach(prefix string, err error) []error {
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		return []error{fmt.Errorf("%s: %w", prefix, err)}
	}
	var out []error
	for _, e := range joined.Unwrap() {
		out = append(out, prefixEach(prefix, e)...)
	}
	return out
}

// cloneRules copies rules and each rule's client list.
func cloneRules(rules []Rule) []Rule {
	if rules == nil {
		return nil
	}
	out := make([]Rule, len(rules))
	for i, rule := range rules {
		out[i] = Rule{Clients: slices.Clone(rule.Clients), Access: rule.Access}
	}
	return out
}

// thousands writes n with a comma between each group of three digits.
func thousands(n int) string {
	s := fmt.Sprint(n)
	if n < 0 {
		return "-" + thousands(-n)
	}
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
