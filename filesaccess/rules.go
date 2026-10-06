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
	"maps"
	"slices"
)

// MaxRules is the most rules one export may hold.
const MaxRules = 1000

// Rule grants Access to the clients it names. On the wire it is
// {"clients": [...], "accessType": "rw"}.
type Rule struct {
	Clients []Client
	Access  Access

	// extra holds the members of a rule decoded from a reply that this
	// version does not know, as they were sent, so encoding the rule sends
	// them back unchanged.
	extra map[string]json.RawMessage
}

// Equal reports whether r and o name the same clients, in the same order,
// grant the same access, and carry the same members this version does not
// know.
func (r Rule) Equal(o Rule) bool {
	return r.Access == o.Access && slices.Equal(r.Clients, o.Clients) &&
		maps.EqualFunc(r.extra, o.extra, func(a, b json.RawMessage) bool { return bytes.Equal(a, b) })
}

// check is the per-rule check: at least one client, none of them zero, and an
// access type. Whether a client appears twice is a check on a whole list, made
// by NewRules. A value this version does not know passes, since only a reply
// carries one, and AIStor checks it.
func (r Rule) check() error {
	if len(r.Clients) == 0 {
		return errors.New("a rule names at least one client")
	}
	if slices.ContainsFunc(r.Clients, Client.IsZero) {
		return errors.New("a rule names an empty client specification")
	}
	if r.Access == "" {
		return errors.New("a rule needs an access type")
	}
	return nil
}

// checkKnown refuses a rule carrying a value this version does not know: an
// Unrecognized client, an access type other than RW, RO and None, or an
// unknown member.
func (r Rule) checkKnown() error {
	var errs []error
	for _, c := range r.Clients {
		if c.kind == Unrecognized {
			errs = append(errs, fmt.Errorf("client specification %q is of a form this version does not know", c.norm))
		}
	}
	if !r.Access.Known() {
		errs = append(errs, fmt.Errorf("access type %q is one this version does not know", string(r.Access)))
	}
	for _, k := range slices.Sorted(maps.Keys(r.extra)) {
		errs = append(errs, fmt.Errorf("member %q is one this version does not know", k))
	}
	return errors.Join(errs...)
}

// ruleJSON is the wire form of a Rule.
type ruleJSON struct {
	Clients    []Client `json:"clients"`
	AccessType Access   `json:"accessType"`
}

// The members of a rule this version knows.
const (
	clientsMember    = "clients"
	accessTypeMember = "accessType"
)

// MarshalJSON encodes the rule with each client in its normalized form, and
// with every member it was decoded with that this version does not know. A
// rule that would not pass its own checks is an error.
func (r Rule) MarshalJSON() ([]byte, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	if len(r.extra) == 0 {
		return json.Marshal(ruleJSON{Clients: r.Clients, AccessType: r.Access})
	}
	members := make(map[string]any, len(r.extra)+2)
	for k, v := range r.extra {
		members[k] = v
	}
	members[clientsMember] = r.Clients
	members[accessTypeMember] = r.Access
	return json.Marshal(members)
}

// UnmarshalJSON decodes one rule of a reply. It refuses a rule without clients
// or an access type, or whose members are not of their JSON types, and reports
// every such error. It refuses no value for being one this version does not
// know: a newer server may send one, and an export holding it must stay
// readable. An access type it does not know is kept as it is, a client
// specification ParseClient refuses is kept as an Unrecognized Client, and an
// unknown member is kept with the rule. Encoding the rule sends each back
// unchanged.
//
// Rules.UnmarshalJSON refuses all three, since a Rules is a policy about to be
// sent.
func (r *Rule) UnmarshalJSON(b []byte) error {
	return r.decode(b, false)
}

// decode decodes one rule as UnmarshalJSON does. strict refuses a value this
// version does not know: a client specification ParseClient refuses, an access
// type ParseAccess refuses, and an unknown member.
func (r *Rule) decode(b []byte, strict bool) error {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(b, &members); err != nil {
		return fmt.Errorf("a rule is {\"clients\": [...], \"accessType\": ...}: %w", err)
	}

	var errs []error
	var rawClients []string
	if v, ok := members[clientsMember]; ok {
		if err := json.Unmarshal(v, &rawClients); err != nil {
			errs = append(errs, fmt.Errorf("a rule's clients are an array of strings: %w", err))
		}
		delete(members, clientsMember)
	}
	clients := make([]Client, 0, len(rawClients))
	for _, s := range rawClients {
		c, err := ParseClient(s)
		switch {
		case err == nil:
			clients = append(clients, c)
		case strict:
			errs = append(errs, err)
		default:
			clients = append(clients, Client{kind: Unrecognized, norm: s})
		}
	}
	if len(rawClients) == 0 {
		errs = append(errs, errors.New("a rule names at least one client"))
	}

	var access Access
	if v, ok := members[accessTypeMember]; !ok {
		errs = append(errs, errors.New("a rule needs an accessType"))
	} else if strict {
		if err := access.UnmarshalJSON(v); err != nil {
			errs = append(errs, err)
		}
	} else {
		// Decode a string, since Access.UnmarshalJSON refuses a value this
		// version does not know.
		var s string
		if err := json.Unmarshal(v, &s); err != nil || s == "" {
			errs = append(errs, errors.New("a rule's accessType is a non-empty string"))
		}
		access = Access(s)
	}
	delete(members, accessTypeMember)

	if strict {
		for _, k := range slices.Sorted(maps.Keys(members)) {
			errs = append(errs, fmt.Errorf("unknown member %q; a rule is {\"clients\": [...], \"accessType\": ...}", k))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	*r = Rule{Clients: clients, Access: access}
	if len(members) > 0 {
		r.extra = members
	}
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
//
// A value this version does not know, which only a rule decoded from a reply
// carries, is let through for AIStor to check, so the rules of an export read
// from a newer server can be edited and sent back.
func NewRules(rules []Rule) (Rules, error) {
	var errs []error
	seen := make(map[Client]int)
	for i, rule := range rules {
		if err := rule.check(); err != nil {
			errs = append(errs, fmt.Errorf("rule %d: %w", i+1, err))
			continue
		}
		errs = append(errs, checkDuplicates(rule, i+1, seen)...)
	}
	if err := checkCount(len(rules)); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return Rules{}, errors.Join(errs...)
	}
	return Rules{rules: cloneRules(rules)}, nil
}

// checkDuplicates reports each client of rule, the rule at 1-based position
// pos, that an earlier rule or an earlier entry of this one already names.
// seen maps each specification already read to its rule's position, and gains
// this rule's.
func checkDuplicates(rule Rule, pos int, seen map[Client]int) []error {
	var errs []error
	for _, c := range rule.Clients {
		if first, dup := seen[c]; dup {
			errs = append(errs, fmt.Errorf("rule %d: %q already appears in rule %d; this rule can never match it", pos, c.String(), first))
			continue
		}
		seen[c] = pos
	}
	return errs
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
// and the per-entry checks of Rule. Unlike Rule.UnmarshalJSON, it refuses a
// value this version does not know: an unknown member, an access type
// ParseAccess refuses, and a client specification ParseClient refuses, because
// the list is a policy to send, checked before it is sent. It reports every error in rule order, each naming the rule
// by its 1-based position. A rule that does not decode still leaves the others
// checked for duplicates among themselves.
func (r *Rules) UnmarshalJSON(b []byte) error {
	var raws []json.RawMessage
	if err := json.Unmarshal(b, &raws); err != nil {
		return fmt.Errorf("access rules are a JSON array: %w", err)
	}
	var errs []error
	rules := make([]Rule, 0, len(raws))
	seen := make(map[Client]int)
	for i, raw := range raws {
		var rule Rule
		if err := rule.decode(raw, true); err != nil {
			errs = append(errs, prefixEach(fmt.Sprintf("rule %d", i+1), err)...)
			continue
		}
		errs = append(errs, checkDuplicates(rule, i+1, seen)...)
		rules = append(rules, rule)
	}
	if err := checkCount(len(raws)); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	*r = Rules{rules: rules}
	return nil
}

// prefixEach labels err with prefix, and each error inside it separately when
// it joins several, so every line of the message names where it comes from.
func prefixEach(prefix string, err error) []error {
	out := flatten(err)
	for i, e := range out {
		out[i] = fmt.Errorf("%s: %w", prefix, e)
	}
	return out
}

// flatten returns the errors err joins, each of them flattened in turn, or
// err alone when it joins none.
func flatten(err error) []error {
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		return []error{err}
	}
	var out []error
	for _, e := range joined.Unwrap() {
		out = append(out, flatten(e)...)
	}
	return out
}

// cloneRules copies rules, and each rule's client list and unknown members.
func cloneRules(rules []Rule) []Rule {
	if rules == nil {
		return nil
	}
	out := make([]Rule, len(rules))
	for i, rule := range rules {
		out[i] = Rule{Clients: slices.Clone(rule.Clients), Access: rule.Access, extra: maps.Clone(rule.extra)}
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
