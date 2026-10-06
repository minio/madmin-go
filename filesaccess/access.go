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

// Package filesaccess parses, validates, encodes and prints the access rules
// of an AIStor Files export.
//
// An export's access rules are an ordered list. The first rule that matches a
// client decides its access, so the order is the policy: nothing in this
// package sorts a list or removes a duplicate from one. A list that names a
// client specification twice is refused instead, because the second
// appearance can never match.
//
// The same checks run wherever a policy is written: Parse reads a rules file,
// and Rules.UnmarshalJSON a request. So a file mc accepts is a request AIStor
// accepts.
//
// A reply is read with Rule.UnmarshalJSON instead, which never refuses a value
// for being one this version does not know. A newer server may add an access
// type, a client form or a rule member, and an export holding one must still
// be readable, and writable without losing it. Such a value is kept as it was
// sent: an access type for which Access.Known is false, a Client of kind
// Unrecognized, or a member kept with its rule. Encoding the rule sends each
// back unchanged, and NewRules lets them through, leaving their check to
// AIStor.
package filesaccess

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Access is the access a rule grants to the clients it matches, as it is
// written in a rules file and on the wire. RW, RO and None are the values this
// version knows. A rule decoded from a reply keeps any other value as it was
// sent, so a value a newer server adds is written back unchanged; Known tells
// the two apart. The empty value is no access type at all, and is refused
// wherever one is required.
type Access string

// The access types a rule can grant.
const (
	// RW grants read and write access.
	RW Access = "rw"

	// RO grants read-only access.
	RO Access = "ro"

	// None grants no access: a client it matches cannot mount the export.
	None Access = "none"
)

// String returns the access type as it is written.
func (a Access) String() string { return string(a) }

// Known reports whether a is RW, RO or None.
func (a Access) Known() bool {
	switch a {
	case RW, RO, None:
		return true
	}
	return false
}

// ParseAccess reads an access type. Only the lower-case spellings of RW, RO
// and None are accepted.
func ParseAccess(s string) (Access, error) {
	if a := Access(s); a.Known() {
		return a, nil
	}
	return "", fmt.Errorf("%q is not an access type; want rw, ro or none", s)
}

// MarshalJSON encodes the access type as its string. The empty value is an
// error rather than an empty string, so a rule without an access type is never
// sent. A value this version does not know is encoded as it is.
func (a Access) MarshalJSON() ([]byte, error) {
	if a == "" {
		return nil, errors.New("a rule needs an access type")
	}
	return json.Marshal(string(a))
}

// UnmarshalJSON decodes an access type with the checks of ParseAccess.
func (a *Access) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("an access type is a string: %w", err)
	}
	v, err := ParseAccess(s)
	if err != nil {
		return err
	}
	*a = v
	return nil
}
