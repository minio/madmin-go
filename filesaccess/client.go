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
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

// Kind is the form of a client specification.
type Kind uint8

// The forms a client specification takes. The zero value is no form, and
// only the zero Client carries it.
const (
	// Every is "*", which matches every client.
	Every Kind = iota + 1

	// IPv4Addr is one IPv4 address, such as 10.1.2.7.
	IPv4Addr

	// IPv4Net is an IPv4 network, such as 10.1.0.0/16.
	IPv4Net

	// IPv6Addr is one IPv6 address, such as fd00::7.
	IPv6Addr

	// IPv6Net is an IPv6 network, such as fd00::/64.
	IPv6Net

	// Hostname is a host name, such as build01.example.com. The serving node
	// resolves it by forward DNS when it loads the rules.
	Hostname

	// HostPattern is a host name pattern using *, ? or [...], such as
	// *.lab.example.com. It is matched against the client's address as text,
	// then against its reverse-DNS name, and the match is case-sensitive.
	HostPattern

	// Netgroup is a netgroup, such as @contractors.
	Netgroup
)

// String names the form, for messages.
func (k Kind) String() string {
	switch k {
	case Every:
		return "every client"
	case IPv4Addr:
		return "IPv4 address"
	case IPv4Net:
		return "IPv4 network"
	case IPv6Addr:
		return "IPv6 address"
	case IPv6Net:
		return "IPv6 network"
	case Hostname:
		return "hostname"
	case HostPattern:
		return "hostname pattern"
	case Netgroup:
		return "netgroup"
	}
	return "invalid"
}

// The length limits of the forms that have one.
const (
	maxHostnameLen = 253
	maxLabelLen    = 63
	maxPatternLen  = 64
	maxNetgroupLen = 64
)

// Client is one client specification, held in its normalized form. Only
// ParseClient, and the decoders that call it, build one, so a Client other
// than the zero value is always valid. Two Clients are the same specification
// exactly when they compare equal with ==.
type Client struct {
	kind Kind
	norm string
}

// Kind returns the form of the specification.
func (c Client) Kind() Kind { return c.kind }

// String returns the normalized form:
//
//   - "*" as it is;
//   - an IPv4 address or network as written, with a /32 network written as
//     its address;
//   - an IPv6 address or network in RFC 5952 text, with a /128 network
//     written as its address;
//   - a hostname in lower case;
//   - a hostname pattern or a netgroup as written.
//
// The zero Client returns "".
func (c Client) String() string { return c.norm }

// IsZero reports whether c is the zero Client, which names no client.
func (c Client) IsZero() bool { return c.kind == 0 }

// MarshalJSON encodes the normalized form. The zero Client is an error, so a
// rule naming no client is never sent.
func (c Client) MarshalJSON() ([]byte, error) {
	if c.IsZero() {
		return nil, errors.New("an empty client specification cannot be encoded")
	}
	return json.Marshal(c.norm)
}

// UnmarshalJSON decodes a client specification with the checks of
// ParseClient.
func (c *Client) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("a client specification is a string: %w", err)
	}
	v, err := ParseClient(s)
	if err != nil {
		return err
	}
	*c = v
	return nil
}

// ParseClient reads one client specification and returns it in normalized
// form. It refuses the spellings Ganesha would read differently from what
// they appear to say:
//
//   - 0.0.0.0, which Ganesha reads as every client: write "*";
//   - an IPv4-mapped IPv6 address, which never matches, because Ganesha
//     converts a mapped client address to IPv4 before matching;
//   - a network with host bits set;
//   - an IPv4 octet with a leading zero, which some parsers read as octal;
//   - a hostname that starts with a digit, which Ganesha's config lexer
//     reads as a number.
//
// A hostname is checked for syntax only. Nothing here resolves it.
func ParseClient(s string) (Client, error) {
	switch {
	case s == "":
		return Client{}, errors.New("empty client specification")
	case s == "*":
		return Client{kind: Every, norm: s}, nil
	case s[0] == '@':
		return parseNetgroup(s)
	case strings.ContainsAny(s, "*?["):
		return parsePattern(s)
	case strings.Contains(s, ":"):
		return parseIPv6(s)
	case strings.Trim(s, "0123456789./") == "":
		return parseIPv4(s)
	}
	return parseHostname(s)
}

// parseIPv4 reads an IPv4 address, or a network when s carries a prefix
// length.
func parseIPv4(s string) (Client, error) {
	addrText, bitsText, isNet := strings.Cut(s, "/")
	addr, err := parseIPv4Addr(addrText)
	if err != nil {
		return Client{}, fmt.Errorf("%q is not an IPv4 address: %w", s, err)
	}
	if !isNet {
		return ipv4Addr(s, addr)
	}
	bits, err := parsePrefixLen(bitsText, 32)
	if err != nil {
		return Client{}, fmt.Errorf("%q is not an IPv4 network: %w", s, err)
	}
	if bits == 32 {
		return ipv4Addr(s, addr)
	}
	prefix := netip.PrefixFrom(addr, bits)
	if masked := prefix.Masked(); masked.Addr() != addr {
		return Client{}, fmt.Errorf("%q has host bits set; did you mean %q?", s, masked.String())
	}
	return Client{kind: IPv4Net, norm: prefix.String()}, nil
}

// ipv4Addr returns the client for one IPv4 address, refusing 0.0.0.0. s is
// the specification as written, for the message.
func ipv4Addr(s string, addr netip.Addr) (Client, error) {
	if addr.IsUnspecified() {
		return Client{}, fmt.Errorf("%q is read by Ganesha as every client; write \"*\"", s)
	}
	return Client{kind: IPv4Addr, norm: addr.String()}, nil
}

// parseIPv4Addr reads four decimal octets, each 0 to 255 and without a
// leading zero. It is stricter than netip.ParseAddr only in the messages it
// gives.
func parseIPv4Addr(s string) (netip.Addr, error) {
	fields := strings.Split(s, ".")
	if len(fields) != 4 {
		return netip.Addr{}, errors.New("want four decimal octets")
	}
	var octets [4]byte
	for i, f := range fields {
		switch {
		case f == "":
			return netip.Addr{}, errors.New("an octet is empty")
		case len(f) > 1 && f[0] == '0':
			return netip.Addr{}, fmt.Errorf("octet %q has a leading zero, which some parsers read as octal", f)
		case len(f) > 3 || strings.Trim(f, "0123456789") != "":
			return netip.Addr{}, fmt.Errorf("octet %q is not a number from 0 to 255", f)
		}
		n, _ := strconv.Atoi(f)
		if n > 255 {
			return netip.Addr{}, fmt.Errorf("octet %q is not a number from 0 to 255", f)
		}
		octets[i] = byte(n)
	}
	return netip.AddrFrom4(octets), nil
}

// parsePrefixLen reads a prefix length from 0 to max, in decimal without a
// leading zero.
func parsePrefixLen(s string, maxBits int) (int, error) {
	if s == "" || strings.Trim(s, "0123456789") != "" || (len(s) > 1 && s[0] == '0') || len(s) > 3 {
		return 0, fmt.Errorf("prefix length %q is not a number from 0 to %d", s, maxBits)
	}
	n, _ := strconv.Atoi(s)
	if n > maxBits {
		return 0, fmt.Errorf("prefix length %q is not a number from 0 to %d", s, maxBits)
	}
	return n, nil
}

// parseIPv6 reads an IPv6 address, or a network when s carries a prefix
// length.
func parseIPv6(s string) (Client, error) {
	addrText, bitsText, isNet := strings.Cut(s, "/")
	if strings.Contains(addrText, "%") {
		return Client{}, fmt.Errorf("%q carries a zone; a client specification takes none", s)
	}
	addr, err := netip.ParseAddr(addrText)
	if err != nil || !addr.Is6() {
		return Client{}, fmt.Errorf("%q is not an IPv6 address", s)
	}
	if addr.Is4In6() {
		return Client{}, fmt.Errorf("%q is an IPv4-mapped IPv6 address, which never matches because Ganesha matches a mapped client as IPv4; write %q",
			s, ipv4Form(addr, bitsText, isNet))
	}
	if !isNet {
		return Client{kind: IPv6Addr, norm: addr.String()}, nil
	}
	bits, err := parsePrefixLen(bitsText, 128)
	if err != nil {
		return Client{}, fmt.Errorf("%q is not an IPv6 network: %w", s, err)
	}
	if bits == 128 {
		return Client{kind: IPv6Addr, norm: addr.String()}, nil
	}
	prefix := netip.PrefixFrom(addr, bits)
	if masked := prefix.Masked(); masked.Addr() != addr {
		return Client{}, fmt.Errorf("%q has host bits set; did you mean %q?", s, masked.String())
	}
	return Client{kind: IPv6Net, norm: prefix.String()}, nil
}

// ipv4Form suggests the IPv4 spelling of an IPv4-mapped address or network.
func ipv4Form(addr netip.Addr, bitsText string, isNet bool) string {
	v4 := addr.Unmap().String()
	if !isNet {
		return v4
	}
	if bits, err := parsePrefixLen(bitsText, 128); err == nil && bits >= 96 {
		return v4 + "/" + strconv.Itoa(bits-96)
	}
	return v4
}

// parseHostname reads a host name: dot-separated labels of letters, digits
// and '-', starting with a letter. Its normalized form is lower case.
func parseHostname(s string) (Client, error) {
	if isDigit(s[0]) {
		return Client{}, fmt.Errorf("%q is not a client specification: a hostname that starts with a digit is read by Ganesha's config lexer as a number", s)
	}
	if !isLetter(s[0]) {
		return Client{}, fmt.Errorf("%q is not a client specification: a hostname starts with a letter", s)
	}
	if len(s) > maxHostnameLen {
		return Client{}, fmt.Errorf("hostname %q is longer than %d characters", s, maxHostnameLen)
	}
	for label := range strings.SplitSeq(s, ".") {
		if err := checkLabel(label); err != nil {
			return Client{}, fmt.Errorf("%q is not a hostname: %w", s, err)
		}
	}
	return Client{kind: Hostname, norm: strings.ToLower(s)}, nil
}

// checkLabel checks one label of a host name.
func checkLabel(label string) error {
	switch {
	case label == "":
		return errors.New("a label is empty")
	case len(label) > maxLabelLen:
		return fmt.Errorf("label %q is longer than %d characters", label, maxLabelLen)
	case label[0] == '-' || label[len(label)-1] == '-':
		return fmt.Errorf("label %q starts or ends with '-'", label)
	}
	for i := 0; i < len(label); i++ {
		if c := label[i]; !isLetter(c) && !isDigit(c) && c != '-' {
			return fmt.Errorf("label %q holds %q; a hostname holds letters, digits, '-' and '.'", label, string(c))
		}
	}
	return nil
}

// parsePattern reads a host name pattern: letters, digits, '.', '-' and '_',
// with at least one of '*', '?', "[...]" or "[!...]". It is kept as written,
// because matching is case-sensitive.
func parsePattern(s string) (Client, error) {
	if len(s) > maxPatternLen {
		return Client{}, fmt.Errorf("hostname pattern %q is longer than %d characters", s, maxPatternLen)
	}
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '*' || c == '?' || isPatternChar(c):
		case c == '[':
			end := strings.IndexByte(s[i+1:], ']')
			if end < 0 {
				return Client{}, fmt.Errorf("hostname pattern %q opens a '[' it does not close", s)
			}
			class := strings.TrimPrefix(s[i+1:i+1+end], "!")
			if class == "" {
				return Client{}, fmt.Errorf("hostname pattern %q has an empty [...] class", s)
			}
			for j := 0; j < len(class); j++ {
				if !isPatternChar(class[j]) {
					return Client{}, fmt.Errorf("hostname pattern %q holds %q in a [...] class; a class holds letters, digits, '.', '-' and '_'",
						s, string(class[j]))
				}
			}
			i += 1 + end
		case c == ']':
			return Client{}, fmt.Errorf("hostname pattern %q closes a ']' it did not open", s)
		default:
			return Client{}, fmt.Errorf("hostname pattern %q holds %q; a pattern holds letters, digits, '.', '-', '_', '*', '?' and [...]",
				s, string(c))
		}
	}
	return Client{kind: HostPattern, norm: s}, nil
}

// parseNetgroup reads a netgroup: '@', then a letter or '_', then letters,
// digits, '_', '.' and '-'. It is kept as written.
func parseNetgroup(s string) (Client, error) {
	if len(s) > maxNetgroupLen {
		return Client{}, fmt.Errorf("netgroup %q is longer than %d characters", s, maxNetgroupLen)
	}
	if len(s) < 2 || (!isLetter(s[1]) && s[1] != '_') {
		return Client{}, fmt.Errorf("netgroup %q does not start with a letter or '_' after the '@'", s)
	}
	for i := 2; i < len(s); i++ {
		if c := s[i]; !isLetter(c) && !isDigit(c) && c != '_' && c != '.' && c != '-' {
			return Client{}, fmt.Errorf("netgroup %q holds %q; a netgroup holds letters, digits, '_', '.' and '-'", s, string(c))
		}
	}
	return Client{kind: Netgroup, norm: s}, nil
}

func isLetter(c byte) bool { return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') }

func isDigit(c byte) bool { return '0' <= c && c <= '9' }

// isPatternChar reports whether c may appear in a hostname pattern as
// itself, outside the wildcards.
func isPatternChar(c byte) bool {
	return isLetter(c) || isDigit(c) || c == '.' || c == '-' || c == '_'
}
