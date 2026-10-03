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
	"encoding/json"
	"strings"
	"testing"
)

// TestParseClientAccepts checks each form the rules file accepts, and the
// normalized form each comes back in.
func TestParseClientAccepts(t *testing.T) {
	for _, tc := range []struct {
		in   string
		kind Kind
		norm string
	}{
		{"*", Every, "*"},

		{"10.1.2.7", IPv4Addr, "10.1.2.7"},
		{"0.0.0.1", IPv4Addr, "0.0.0.1"},
		{"255.255.255.255", IPv4Addr, "255.255.255.255"},
		{"10.1.2.7/32", IPv4Addr, "10.1.2.7"},
		{"10.1.0.0/16", IPv4Net, "10.1.0.0/16"},
		{"10.20.9.0/24", IPv4Net, "10.20.9.0/24"},
		{"0.0.0.0/0", IPv4Net, "0.0.0.0/0"},

		{"fd00::7", IPv6Addr, "fd00::7"},
		{"FD00:0:0:0::7", IPv6Addr, "fd00::7"},
		{"FD00:0:0:0::7/128", IPv6Addr, "fd00::7"},
		{"2001:db8:0:0:1:0:0:1", IPv6Addr, "2001:db8::1:0:0:1"},
		{"fd00::/64", IPv6Net, "fd00::/64"},
		{"FD00:0000::/8", IPv6Net, "fd00::/8"},
		{"::/0", IPv6Net, "::/0"},

		{"build01.example.com", Hostname, "build01.example.com"},
		{"Build01.Example.COM", Hostname, "build01.example.com"},
		{"localhost", Hostname, "localhost"},
		{"a-b.c-d", Hostname, "a-b.c-d"},

		{"*.lab.example.com", HostPattern, "*.lab.example.com"},
		{"*.Lab.Example.com", HostPattern, "*.Lab.Example.com"},
		{"build??.example.com", HostPattern, "build??.example.com"},
		{"node[0-9].example.com", HostPattern, "node[0-9].example.com"},
		{"node[!a].example_x.com", HostPattern, "node[!a].example_x.com"},
		{"10.1.*", HostPattern, "10.1.*"},

		{"@contractors", Netgroup, "@contractors"},
		{"@_ops.team-1", Netgroup, "@_ops.team-1"},
		{"@" + strings.Repeat("a", 63), Netgroup, "@" + strings.Repeat("a", 63)},
	} {
		c, err := ParseClient(tc.in)
		if err != nil {
			t.Errorf("ParseClient(%q): %v", tc.in, err)
			continue
		}
		if c.Kind() != tc.kind || c.String() != tc.norm {
			t.Errorf("ParseClient(%q) = %v %q, want %v %q", tc.in, c.Kind(), c.String(), tc.kind, tc.norm)
		}
		again, err := ParseClient(c.String())
		if err != nil || again != c {
			t.Errorf("ParseClient(%q) of the normalized form = %v, %v; want it unchanged", c.String(), again, err)
		}
	}
}

// TestParseClientRefuses checks the refusals the spec lists, and that each
// message says why.
func TestParseClientRefuses(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{"", "empty"},
		{"0.0.0.0", `write "*"`},
		{"0.0.0.0/32", `write "*"`},
		{"::ffff:10.1.2.7", `IPv4-mapped IPv6 address, which never matches because Ganesha matches a mapped client as IPv4; write "10.1.2.7"`},
		{"::ffff:10.1.0.0/112", `write "10.1.0.0/16"`},
		{"10.1.2.3/16", `"10.1.2.3/16" has host bits set; did you mean "10.1.0.0/16"?`},
		{"fd00::7/64", `did you mean "fd00::/64"?`},
		{"010.1.2.7", "leading zero"},
		{"10.1.2.07", "leading zero"},
		{"1build.example.com", "starts with a digit"},
		{"10.1.2", "four decimal octets"},
		{"10.1.2.3.4", "four decimal octets"},
		{"10.1..3", "octet is empty"},
		{"10.1.2.256", "from 0 to 255"},
		{"10.1.2.1000", "from 0 to 255"},
		{"10.1.0.0/33", "from 0 to 32"},
		{"10.1.0.0/", "from 0 to 32"},
		{"10.1.0.0/08", "from 0 to 32"},
		{"10.1.0.0/16/16", "from 0 to 32"},
		{"fd00::/129", "from 0 to 128"},
		{"fe80::1%eth0", "zone"},
		{"fd00:::7", "not an IPv6 address"},
		{"1.2.3.4:80", "not an IPv6 address"},
		{"-host", "starts with a letter"},
		{"host_name", "holds \"_\""},
		{"host.", "label is empty"},
		{"host..example", "label is empty"},
		{"host-.example", "starts or ends with '-'"},
		{"a." + strings.Repeat("b", 64), "longer than 63"},
		{"a" + strings.Repeat(".bb", 90), "longer than 253"},
		{"host name", "holds \" \""},
		{"*.lab/x", `holds "/"`},
		{"node[0-9.example.com", "does not close"},
		{"node]*", "did not open"},
		{"node[]*", "empty [...] class"},
		{"node[!]*", "empty [...] class"},
		{"node[*]", "in a [...] class"},
		{"*" + strings.Repeat("a", 64), "longer than 64"},
		{"@", "after the '@'"},
		{"@1group", "after the '@'"},
		{"@group/x", `holds "/"`},
		{"@" + strings.Repeat("a", 64), "longer than 64"},
	} {
		c, err := ParseClient(tc.in)
		if err == nil {
			t.Errorf("ParseClient(%q) = %v %q, want an error", tc.in, c.Kind(), c.String())
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("ParseClient(%q) error %q does not say %q", tc.in, err, tc.want)
		}
	}
}

// TestClientZero checks that the zero Client names nothing and cannot be
// encoded.
func TestClientZero(t *testing.T) {
	var c Client
	if !c.IsZero() || c.String() != "" || c.Kind() != 0 {
		t.Fatalf("zero Client = %v %q", c.Kind(), c.String())
	}
	if _, err := json.Marshal(c); err == nil {
		t.Fatal("encoding the zero Client succeeded")
	}
}

// TestClientJSON checks that a Client encodes in normalized form and decodes
// with ParseClient's checks.
func TestClientJSON(t *testing.T) {
	var c Client
	if err := json.Unmarshal([]byte(`"10.1.2.7/32"`), &c); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(c)
	if err != nil || string(b) != `"10.1.2.7"` {
		t.Fatalf("Marshal = %s, %v; want \"10.1.2.7\"", b, err)
	}
	for _, in := range []string{`"0.0.0.0"`, `7`, `null`, `""`} {
		var c Client
		if err := json.Unmarshal([]byte(in), &c); err == nil {
			t.Errorf("Unmarshal(%s) succeeded", in)
		}
	}
}

// TestAccess checks the access types' spellings, and that only the
// lower-case ones are read.
func TestAccess(t *testing.T) {
	for _, a := range []Access{RW, RO, None} {
		got, err := ParseAccess(a.String())
		if err != nil || got != a {
			t.Errorf("ParseAccess(%q) = %v, %v", a.String(), got, err)
		}
		b, err := json.Marshal(a)
		if err != nil || string(b) != `"`+a.String()+`"` {
			t.Errorf("Marshal(%v) = %s, %v", a, b, err)
		}
	}
	for _, s := range []string{"", "RW", "Ro", "read", "rw "} {
		if _, err := ParseAccess(s); err == nil {
			t.Errorf("ParseAccess(%q) succeeded", s)
		}
	}
	if _, err := json.Marshal(Access(0)); err == nil {
		t.Error("encoding the zero Access succeeded")
	}
	var a Access
	if err := json.Unmarshal([]byte(`"RO"`), &a); err == nil {
		t.Error(`Unmarshal("RO") succeeded`)
	}
}
