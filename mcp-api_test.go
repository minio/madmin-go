//
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
// along with this program. If not, see <http://www.gnu.org/licenses/>.
//

package madmin

import (
	"testing"
)

func TestMCPPermissionBitLayout(t *testing.T) {
	// Verify the bit positions match the documented table.
	tests := []struct {
		perm MCPPermission
		bit  uint
	}{
		{MCPPermObjectsRead, 0},
		{MCPPermObjectsWrite, 1},
		{MCPPermObjectsDelete, 2},
		{MCPPermAdminRead, 3},
		{MCPPermAdminWrite, 4},
		{MCPPermAdminDelete, 5},
		{MCPPermTablesRead, 6},
		{MCPPermTablesWrite, 7},
		{MCPPermTablesDelete, 8},
		{MCPPermBucketsRead, 9},
		{MCPPermBucketsWrite, 10},
		{MCPPermBucketsDelete, 11},
	}
	for _, tc := range tests {
		if want := MCPPermission(1 << tc.bit); tc.perm != want {
			t.Errorf("bit %d: got %d, want %d", tc.bit, tc.perm, want)
		}
	}
}

func TestMCPPermissionGroups(t *testing.T) {
	if MCPPermObjects != MCPPermObjectsRead|MCPPermObjectsWrite|MCPPermObjectsDelete {
		t.Error("MCPPermObjects mismatch")
	}
	if MCPPermAdmin != MCPPermAdminRead|MCPPermAdminWrite|MCPPermAdminDelete {
		t.Error("MCPPermAdmin mismatch")
	}
	if MCPPermTables != MCPPermTablesRead|MCPPermTablesWrite|MCPPermTablesDelete {
		t.Error("MCPPermTables mismatch")
	}
	if MCPPermBuckets != MCPPermBucketsRead|MCPPermBucketsWrite|MCPPermBucketsDelete {
		t.Error("MCPPermBuckets mismatch")
	}
	if MCPPermAll != MCPPermObjects|MCPPermAdmin|MCPPermTables|MCPPermBuckets {
		t.Error("MCPPermAll mismatch")
	}
	if MCPPermRead != MCPPermObjectsRead|MCPPermAdminRead|MCPPermTablesRead|MCPPermBucketsRead {
		t.Error("MCPPermRead mismatch")
	}
	if MCPPermWrite != MCPPermObjectsWrite|MCPPermAdminWrite|MCPPermTablesWrite|MCPPermBucketsWrite {
		t.Error("MCPPermWrite mismatch")
	}
	if MCPPermDelete != MCPPermObjectsDelete|MCPPermAdminDelete|MCPPermTablesDelete|MCPPermBucketsDelete {
		t.Error("MCPPermDelete mismatch")
	}
}

func TestMCPPermissionHas(t *testing.T) {
	p := MCPPermObjectsRead | MCPPermAdminWrite
	if !p.Has(MCPPermObjectsRead) {
		t.Error("should have objects:read")
	}
	if !p.Has(MCPPermAdminWrite) {
		t.Error("should have admin:write")
	}
	if p.Has(MCPPermObjectsDelete) {
		t.Error("should not have objects:delete")
	}
	// Has checks all bits present
	if p.Has(MCPPermObjectsRead | MCPPermObjectsWrite) {
		t.Error("should not have objects:read+write combined")
	}
	if !MCPPermAll.Has(MCPPermObjectsRead | MCPPermBucketsDelete) {
		t.Error("all should have any combo")
	}
}

func TestMCPPermissionString(t *testing.T) {
	tests := []struct {
		perm MCPPermission
		want string
	}{
		{MCPPermAll, "all"},
		{MCPPermObjectsRead, "objects:r"},
		{MCPPermObjectsRead | MCPPermObjectsWrite, "objects:rw"},
		{MCPPermAdminRead | MCPPermAdminWrite | MCPPermAdminDelete, "admin:rwd"},
		{MCPPermObjectsRead | MCPPermTablesWrite, "objects:r,tables:w"},
		{MCPPermBucketsRead | MCPPermBucketsDelete, "buckets:rd"},
		{0, ""},
	}
	for _, tc := range tests {
		if got := tc.perm.String(); got != tc.want {
			t.Errorf("(%d).String() = %q, want %q", tc.perm, got, tc.want)
		}
	}
}

func TestParseMCPPermissions(t *testing.T) {
	tests := []struct {
		input string
		want  MCPPermission
	}{
		{"all", MCPPermAll},
		{"objects:r", MCPPermObjectsRead},
		{"objects:rw", MCPPermObjectsRead | MCPPermObjectsWrite},
		{"objects:rwd", MCPPermObjects},
		{"admin:r,tables:wd", MCPPermAdminRead | MCPPermTablesWrite | MCPPermTablesDelete},
		{"buckets:d", MCPPermBucketsDelete},
		{" Objects:R , Admin:W ", MCPPermObjectsRead | MCPPermAdminWrite},
		{"", 0},
	}
	for _, tc := range tests {
		got, err := ParseMCPPermissions(tc.input)
		if err != nil {
			t.Errorf("ParseMCPPermissions(%q) error: %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseMCPPermissions(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestParseMCPPermissionsErrors(t *testing.T) {
	bad := []string{
		"bogus:r",
		"objects:x",
		"objects",
		"read",
	}
	for _, s := range bad {
		if _, err := ParseMCPPermissions(s); err == nil {
			t.Errorf("ParseMCPPermissions(%q) should have failed", s)
		}
	}
}

func TestMCPPermissionRoundTrip(t *testing.T) {
	perms := []MCPPermission{
		MCPPermAll,
		MCPPermObjectsRead | MCPPermAdminWrite | MCPPermBucketsDelete,
		MCPPermTables,
		MCPPermObjectsRead,
	}
	for _, p := range perms {
		s := p.String()
		got, err := ParseMCPPermissions(s)
		if err != nil {
			t.Errorf("round-trip failed for %d -> %q: %v", p, s, err)
			continue
		}
		if got != p {
			t.Errorf("round-trip: %d -> %q -> %d", p, s, got)
		}
	}
}

func TestMCPPermissionNoBitOverlap(t *testing.T) {
	all := []MCPPermission{
		MCPPermObjectsRead, MCPPermObjectsWrite, MCPPermObjectsDelete,
		MCPPermAdminRead, MCPPermAdminWrite, MCPPermAdminDelete,
		MCPPermTablesRead, MCPPermTablesWrite, MCPPermTablesDelete,
		MCPPermBucketsRead, MCPPermBucketsWrite, MCPPermBucketsDelete,
	}
	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			if all[i]&all[j] != 0 {
				t.Errorf("bit overlap between index %d (%d) and %d (%d)", i, all[i], j, all[j])
			}
		}
	}
}
