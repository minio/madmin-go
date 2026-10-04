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
// The same checks run wherever rules enter: Parse reads a rules file,
// Rules.UnmarshalJSON reads a request or a reply, and NewRules takes rules
// built in code. So a file mc accepts is a request AIStor accepts.
package filesaccess

import (
	"encoding/json"
	"fmt"
)

// Access is the access a rule grants to the clients it matches. The zero
// value is no access type at all, and is refused wherever one is required.
type Access uint8

// The access types a rule can grant.
const (
	// RW grants read and write access.
	RW Access = iota + 1

	// RO grants read-only access.
	RO

	// None grants no access: a client it matches cannot mount the export.
	None
)

// String returns the access type as it is written in a rules file and on the
// wire: "rw", "ro" or "none". An invalid value returns "".
func (a Access) String() string {
	switch a {
	case RW:
		return "rw"
	case RO:
		return "ro"
	case None:
		return "none"
	}
	return ""
}

// ParseAccess reads an access type. Only the lower-case spellings are
// accepted.
func ParseAccess(s string) (Access, error) {
	switch s {
	case "rw":
		return RW, nil
	case "ro":
		return RO, nil
	case "none":
		return None, nil
	}
	return 0, fmt.Errorf("%q is not an access type; want rw, ro or none", s)
}

// MarshalJSON encodes the access type as its string. An invalid value is an
// error rather than an empty string, so a zero Access is never sent.
func (a Access) MarshalJSON() ([]byte, error) {
	s := a.String()
	if s == "" {
		return nil, fmt.Errorf("invalid access type %d", uint8(a))
	}
	return json.Marshal(s)
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
