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
	"strings"
	"testing"
)

// mustClients parses each specification, failing the test on an error.
func mustClients(t *testing.T, specs ...string) []Client {
	t.Helper()
	out := make([]Client, len(specs))
	for i, s := range specs {
		c, err := ParseClient(s)
		if err != nil {
			t.Fatalf("ParseClient(%q): %v", s, err)
		}
		out[i] = c
	}
	return out
}

// specRules is the rule list the spec's samples use.
func specRules(t *testing.T) []Rule {
	t.Helper()
	return []Rule{
		{Clients: mustClients(t, "10.20.9.0/24"), Access: None},
		{Clients: mustClients(t, "10.20.4.7", "10.20.4.8"), Access: RW},
		{Clients: mustClients(t, "*.corp.example.com", "10.20.0.0/16"), Access: RO},
	}
}

// specJSON is the spec's request body for those rules.
const specJSON = `[
  { "clients": ["10.20.9.0/24"],                       "accessType": "none" },
  { "clients": ["10.20.4.7", "10.20.4.8"],             "accessType": "rw"   },
  { "clients": ["*.corp.example.com", "10.20.0.0/16"], "accessType": "ro"   }
]`

// TestParseSpecSample reads the spec's sample rules file, comments and all.
func TestParseSpecSample(t *testing.T) {
	const file = "# carol: lab machines read-only, two build hosts read-write\n" +
		"10.20.9.0/24                        none    # contractor range, carved out first\n" +
		"10.20.4.7,10.20.4.8                 rw\r\n" +
		"\n" +
		"   \t# an indented comment\n" +
		"\t*.corp.example.com,10.20.0.0/16\tro  " // no final newline
	rules, err := Parse(strings.NewReader(file), "rules.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.EqualFunc(rules.All(), specRules(t), Rule.Equal) {
		t.Fatalf("Parse = %v, want %v", rules.All(), specRules(t))
	}
}

// TestParseErrors checks that Parse reports every error with its file and
// line, and the file-level ones last.
func TestParseErrors(t *testing.T) {
	const file = "10.20.4.7          rw\n" + // 1
		"# comment\n" + // 2
		"10.1.2.3/16        ro\n" + // 3
		"\n" + // 4
		"10.20.4.7/32       rw\n" + // 5
		"a.example.com      RW\n" + // 6
		"b.example.com\n" + // 7
		"c.example.com, d   rw\n" + // 8
		"e.example.com,     rw\n" + // 9
		"caf\xc3\xa9.com     rw\n" + // 10
		"x\x01y             rw\n" + // 11
		"f.example.com,f.example.com rw\n" // 12
	_, err := Parse(strings.NewReader(file), "rules.txt")
	var errs LineErrors
	if !errors.As(err, &errs) {
		t.Fatalf("Parse error = %v (%T), want LineErrors", err, err)
	}
	want := []string{
		`rules.txt:3: "10.1.2.3/16" has host bits set; did you mean "10.1.0.0/16"?`,
		`rules.txt:5: "10.20.4.7/32" (10.20.4.7) already appears on line 1; this rule can never match it`,
		`rules.txt:6: "RW" is not an access type; want rw, ro or none`,
		`rules.txt:7: want two fields, CLIENTS then ACCESS; found only "b.example.com"`,
		`rules.txt:8: want two fields, CLIENTS then ACCESS; found 3 (client specifications are joined by commas, with no spaces)`,
		`rules.txt:9: empty client specification`,
		`rules.txt:10: byte 0xc3 is not ASCII; a rules file is ASCII text`,
		`rules.txt:11: control character 0x01 is not allowed; a rules file is ASCII text`,
		`rules.txt:12: "f.example.com" already appears on line 12; this rule can never match it`,
	}
	got := strings.Split(err.Error(), "\n")
	if !slices.Equal(got, want) {
		t.Fatalf("Parse errors:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestParseCount checks the 1 to 1,000 bound, and that a file with no rules
// is refused.
func TestParseCount(t *testing.T) {
	for _, tc := range []struct {
		name, file, want string
	}{
		{"empty", "", "rules.txt: the file holds no rules"},
		{"comments only", "# export carol (id 104): no access rules; every client gets rw\n", "rules.txt: the file holds no rules"},
		{"too many", manyRules(1204), "rules.txt: 1,204 rules; at most 1,000 are allowed"},
	} {
		_, err := Parse(strings.NewReader(tc.file), "rules.txt")
		if err == nil || err.Error() != tc.want {
			t.Errorf("%s: Parse error = %v, want %q", tc.name, err, tc.want)
		}
	}
	rules, err := Parse(strings.NewReader(manyRules(MaxRules)), "rules.txt")
	if err != nil || rules.Len() != MaxRules {
		t.Fatalf("Parse of %d rules = %d, %v", MaxRules, rules.Len(), err)
	}
}

// manyRules returns a rules file of n rules, each naming one distinct host.
func manyRules(n int) string {
	var sb strings.Builder
	for i := range n {
		fmt.Fprintf(&sb, "10.%d.%d.%d rw\n", i/65536, i/256%256, i%256)
	}
	return sb.String()
}

// TestParseTooLarge checks that Parse stops at MaxFileBytes.
func TestParseTooLarge(t *testing.T) {
	file := strings.Repeat("#"+strings.Repeat("x", 1023)+"\n", MaxFileBytes/1024+1)
	_, err := Parse(strings.NewReader(file), "big.txt")
	if err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Fatalf("Parse error = %v, want the size refusal", err)
	}
}

// TestParseJSON checks that Parse reads a JSON array of rules, and the object
// mc prints for an export's rules, ignoring its other members.
func TestParseJSON(t *testing.T) {
	for _, tc := range []struct{ name, file string }{
		{"array", specJSON},
		{"array after white space", "\n \t\r\n" + specJSON + "\n"},
		{"mc object", `{"status": "success", "name": "carol", "exportId": 104, "rules": ` + specJSON + `}`},
	} {
		rules, err := Parse(strings.NewReader(tc.file), "rules.json")
		if err != nil {
			t.Errorf("%s: Parse: %v", tc.name, err)
			continue
		}
		if !slices.EqualFunc(rules.All(), specRules(t), Rule.Equal) {
			t.Errorf("%s: Parse = %v, want %v", tc.name, rules.All(), specRules(t))
		}
	}
}

// TestParseJSONRoundTrip checks that Parse reads back what Rules.MarshalJSON
// writes.
func TestParseJSONRoundTrip(t *testing.T) {
	rules, err := NewRules(specRules(t))
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.MarshalIndent(map[string]any{"name": "carol", "rules": rules}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	again, err := Parse(bytes.NewReader(b), "-")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.EqualFunc(again.All(), rules.All(), Rule.Equal) {
		t.Fatalf("Parse = %v, want %v", again.All(), rules.All())
	}
}

// TestParseJSONErrors checks that a JSON file's errors are LineErrors with no
// line, one per error, each naming its rule.
func TestParseJSONErrors(t *testing.T) {
	for _, tc := range []struct {
		name, file string
		want       []string
	}{
		{"empty array", "[]", []string{"f: no rules; a rule list holds at least one rule"}},
		{"mc object with no rules", `{"name": "carol", "rules": []}`, []string{"f: no rules; a rule list holds at least one rule"}},
		{"object without rules", `{"name": "carol"}`, []string{`f: a JSON rules file is an array of rules, or an object with a "rules" member holding one`}},
		{"null rules", `{"rules": null}`, []string{`f: a JSON rules file is an array of rules, or an object with a "rules" member holding one`}},
		{"two values", specJSON + specJSON, []string{"f: the file holds more than one JSON value"}},
		{"truncated", `[{"clients": ["a"]`, []string{"f: the file is not valid JSON: unexpected EOF"}},
		{
			"bad rules",
			`[{"clients": ["10.1.2.3/16", "a.example.com"], "accessType": "RW"}, {"clients": ["a.example.com"], "accessType": "ro"}]`,
			[]string{
				`f: rule 1: "10.1.2.3/16" has host bits set; did you mean "10.1.0.0/16"?`,
				`f: rule 1: "RW" is not an access type; want rw, ro or none`,
			},
		},
		{
			"duplicate",
			`[{"clients": ["a.example.com"], "accessType": "rw"}, {"clients": ["A.example.com"], "accessType": "ro"}]`,
			[]string{`f: rule 2: "a.example.com" already appears in rule 1; this rule can never match it`},
		},
	} {
		_, err := Parse(strings.NewReader(tc.file), "f")
		var errs LineErrors
		if !errors.As(err, &errs) {
			t.Errorf("%s: Parse error = %v (%T), want LineErrors", tc.name, err, err)
			continue
		}
		if got := strings.Split(err.Error(), "\n"); !slices.Equal(got, tc.want) {
			t.Errorf("%s: Parse errors:\n%s\nwant:\n%s", tc.name, strings.Join(got, "\n"), strings.Join(tc.want, "\n"))
		}
	}
}

// TestParseTextStartingWithBracket checks that a text rule starting with a
// hostname pattern class is not taken for JSON.
func TestParseTextStartingWithBracket(t *testing.T) {
	rules, err := Parse(strings.NewReader("[ab]*.corp.example.com rw\n"), "rules.txt")
	if err != nil {
		t.Fatal(err)
	}
	if want := mustClients(t, "[ab]*.corp.example.com"); rules.Len() != 1 || !slices.Equal(rules.All()[0].Clients, want) {
		t.Fatalf("Parse = %v", rules.All())
	}
}

// errReader fails every read.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("disk on fire") }

// TestParseReadError checks that a read error is returned as it is.
func TestParseReadError(t *testing.T) {
	_, err := Parse(errReader{}, "x")
	if err == nil || err.Error() != "disk on fire" {
		t.Fatalf("Parse error = %v", err)
	}
}

// TestFormat checks the printed file against the spec's sample, with the
// header as a comment and the CLIENTS column aligned.
func TestFormat(t *testing.T) {
	var buf bytes.Buffer
	if err := Format(&buf, "export carol (id 104): 3 rules, in match order", specRules(t)); err != nil {
		t.Fatal(err)
	}
	const want = "# export carol (id 104): 3 rules, in match order\n" +
		"10.20.9.0/24                       none\n" +
		"10.20.4.7,10.20.4.8                rw\n" +
		"*.corp.example.com,10.20.0.0/16    ro\n"
	if buf.String() != want {
		t.Fatalf("Format =\n%s\nwant:\n%s", buf.String(), want)
	}
}

// TestFormatEmpty checks that an empty list prints the header alone, and that
// the result is refused by Parse, so a print and apply cannot clear rules.
func TestFormatEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := Format(&buf, "export carol (id 104): no access rules; every client gets rw", nil); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "# export carol (id 104): no access rules; every client gets rw\n" {
		t.Fatalf("Format = %q", buf.String())
	}
	if _, err := Parse(&buf, "-"); err == nil {
		t.Fatal("Parse accepted a file holding no rules")
	}
}

// TestFormatHeader checks that a multi-line header becomes comment lines, and
// that a byte a rules file may not hold is replaced so the output parses.
func TestFormatHeader(t *testing.T) {
	var buf bytes.Buffer
	rules := []Rule{{Clients: mustClients(t, "*"), Access: RW}}
	if err := Format(&buf, "export caf\xc3\xa9\r\nsecond line\n\nfourth\n", rules); err != nil {
		t.Fatal(err)
	}
	const want = "# export caf??\n# second line\n#\n# fourth\n*    rw\n"
	if buf.String() != want {
		t.Fatalf("Format = %q, want %q", buf.String(), want)
	}
	if _, err := Parse(&buf, "-"); err != nil {
		t.Fatalf("Parse of Format output: %v", err)
	}
}

// TestFormatRefuses checks that Format writes nothing for a rule that would
// not pass its own checks.
func TestFormatRefuses(t *testing.T) {
	for _, rule := range []Rule{
		{Access: RW},
		{Clients: []Client{{}}, Access: RW},
		{Clients: mustClients(t, "*")},
	} {
		var buf bytes.Buffer
		if err := Format(&buf, "h", []Rule{rule}); err == nil || buf.Len() != 0 {
			t.Errorf("Format(%v) = %v, wrote %q", rule, err, buf.String())
		}
	}
}

// TestParseFormatRoundTrip checks that Parse(Format(rules)) returns the same
// rules, with every specification in normalized form.
func TestParseFormatRoundTrip(t *testing.T) {
	const file = "Build01.Example.com,FD00:0:0:0::7/128   ro\n" +
		"10.1.2.7/32  none\n" +
		"@contractors,*.Lab.example.com,fd00::/64 rw\n" +
		"* ro\n"
	rules, err := Parse(strings.NewReader(file), "in")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Format(&buf, "round trip", rules.All()); err != nil {
		t.Fatal(err)
	}
	again, err := Parse(bytes.NewReader(buf.Bytes()), "out")
	if err != nil {
		t.Fatalf("Parse of\n%s: %v", buf.String(), err)
	}
	if !slices.EqualFunc(again.All(), rules.All(), Rule.Equal) {
		t.Fatalf("round trip = %v, want %v", again.All(), rules.All())
	}
	if !strings.Contains(buf.String(), "build01.example.com,fd00::7 ") || !strings.Contains(buf.String(), "\n10.1.2.7 ") {
		t.Fatalf("Format did not print the normalized forms:\n%s", buf.String())
	}
}

// TestRulesJSON checks that the spec's request body decodes, and encodes
// back in the same order with the same members.
func TestRulesJSON(t *testing.T) {
	var rules Rules
	if err := json.Unmarshal([]byte(specJSON), &rules); err != nil {
		t.Fatal(err)
	}
	if !slices.EqualFunc(rules.All(), specRules(t), Rule.Equal) {
		t.Fatalf("Unmarshal = %v", rules.All())
	}
	b, err := json.Marshal(rules)
	if err != nil {
		t.Fatal(err)
	}
	const want = `[{"clients":["10.20.9.0/24"],"accessType":"none"},` +
		`{"clients":["10.20.4.7","10.20.4.8"],"accessType":"rw"},` +
		`{"clients":["*.corp.example.com","10.20.0.0/16"],"accessType":"ro"}]`
	if string(b) != want {
		t.Fatalf("Marshal = %s, want %s", b, want)
	}

	// A body field holding Rules decodes and encodes the same way.
	var body struct {
		Rules Rules `json:"rules"`
	}
	if err := json.Unmarshal([]byte(`{"rules":`+specJSON+`}`), &body); err != nil || body.Rules.Len() != 3 {
		t.Fatalf("Unmarshal body = %d rules, %v", body.Rules.Len(), err)
	}
}

// TestRulesJSONNormalizes checks that a decoded list is held in normalized
// form, so 10.1.2.7/32 sent comes back as 10.1.2.7.
func TestRulesJSONNormalizes(t *testing.T) {
	var rules Rules
	if err := json.Unmarshal([]byte(`[{"clients":["10.1.2.7/32","Host.Example.com"],"accessType":"rw"}]`), &rules); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(rules)
	if string(b) != `[{"clients":["10.1.2.7","host.example.com"],"accessType":"rw"}]` {
		t.Fatalf("Marshal = %s", b)
	}
}

// TestRulesJSONErrors checks that decoding reports every error by rule index.
func TestRulesJSONErrors(t *testing.T) {
	for _, tc := range []struct {
		name, in string
		want     []string
	}{
		{"empty", `[]`, []string{"no rules"}},
		{"null", `null`, []string{"no rules"}},
		{"not an array", `{"clients":["*"]}`, []string{"JSON array"}},
		{"bad entries", `[
			{"clients":["10.1.2.3/16","0.0.0.0"],"accessType":"rw"},
			{"clients":["*"],"accessType":"RW"},
			{"clients":[],"accessType":"ro"},
			{"accessType":"ro"},
			{"clients":["a.example.com"]},
			{"clients":["b.example.com"],"accessType":"ro","squash":"all"}
		]`, []string{
			`rule 1: "10.1.2.3/16" has host bits set`,
			`rule 1: "0.0.0.0" is read by Ganesha as every client`,
			`rule 2: "RW" is not an access type`,
			"rule 3: a rule names at least one client",
			"rule 4: a rule names at least one client",
			"rule 5: a rule needs an accessType",
			`rule 6: a rule is {"clients": [...], "accessType": ...}: json: unknown field "squash"`,
		}},
		{"duplicate", `[
			{"clients":["10.1.2.7"],"accessType":"none"},
			{"clients":["*","10.1.2.7/32"],"accessType":"rw"}
		]`, []string{`rule 2: "10.1.2.7" already appears in rule 1; this rule can never match it`}},
		{
			"duplicate within a rule", `[{"clients":["fd00::7","FD00::7"],"accessType":"rw"}]`,
			[]string{`rule 1: "fd00::7" already appears in rule 1`},
		},
	} {
		var rules Rules
		err := json.Unmarshal([]byte(tc.in), &rules)
		if err == nil {
			t.Errorf("%s: Unmarshal succeeded with %v", tc.name, rules.All())
			continue
		}
		for _, w := range tc.want {
			if !strings.Contains(err.Error(), w) {
				t.Errorf("%s: error %q does not say %q", tc.name, err, w)
			}
		}
		if rules.Len() != 0 {
			t.Errorf("%s: a failed decode left %d rules", tc.name, rules.Len())
		}
	}
}

// TestRulesJSONDuplicateBesideBadRule checks that a rule that fails to decode
// does not hide a duplicate among the others, and that every error comes in
// rule order.
func TestRulesJSONDuplicateBesideBadRule(t *testing.T) {
	const in = `[
		{"clients":["10.1.2.7"],"accessType":"none"},
		{"clients":["a.example.com"],"accessType":"RW"},
		{"clients":["10.1.2.7/32"],"accessType":"rw"},
		{"clients":["b.example.com","0.0.0.0"],"accessType":"ro"},
		{"clients":["B.example.com"],"accessType":"ro"}
	]`
	var rules Rules
	err := json.Unmarshal([]byte(in), &rules)
	if err == nil {
		t.Fatalf("Unmarshal succeeded with %v", rules.All())
	}
	want := []string{
		`rule 2: "RW" is not an access type; want rw, ro or none`,
		`rule 3: "10.1.2.7" already appears in rule 1; this rule can never match it`,
		`rule 4: "0.0.0.0" is read by Ganesha as every client; write "*"`,
	}
	if got := strings.Split(err.Error(), "\n"); !slices.Equal(got, want) {
		t.Fatalf("errors:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if rules.Len() != 0 {
		t.Fatalf("a failed decode left %d rules", rules.Len())
	}
}

// TestRulesJSONTooMany checks the upper bound on a decoded list.
func TestRulesJSONTooMany(t *testing.T) {
	entries := make([]string, MaxRules+1)
	for i := range entries {
		entries[i] = fmt.Sprintf(`{"clients":["10.%d.%d.%d"],"accessType":"rw"}`, i/65536, i/256%256, i%256)
	}
	var rules Rules
	err := json.Unmarshal([]byte("["+strings.Join(entries, ",")+"]"), &rules)
	if err == nil || !strings.Contains(err.Error(), "1,001 rules; at most 1,000 are allowed") {
		t.Fatalf("Unmarshal error = %v", err)
	}
}

// TestRuleJSON checks that one Rule decodes on its own with the per-entry
// checks only, so a rule repeating a client is left to the list check.
func TestRuleJSON(t *testing.T) {
	var rule Rule
	if err := json.Unmarshal([]byte(`{"clients":["a.example.com","A.example.com"],"accessType":"ro"}`), &rule); err != nil {
		t.Fatal(err)
	}
	if len(rule.Clients) != 2 || rule.Access != RO {
		t.Fatalf("Unmarshal = %v", rule)
	}
	if _, err := json.Marshal(Rule{Access: RW}); err == nil {
		t.Fatal("encoding a rule with no client succeeded")
	}
}

// TestRuleJSONIgnoresUnknownMember checks that one Rule, which is what a
// reply carries, decodes past a member a newer server adds, while a Rules,
// which is a policy to send, still refuses it.
func TestRuleJSONIgnoresUnknownMember(t *testing.T) {
	const rule = `{"clients":["10.1.2.0/24"],"accessType":"ro","squash":"all"}`
	var got Rule
	if err := json.Unmarshal([]byte(rule), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(got.Clients) != 1 || got.Access != RO {
		t.Fatalf("Unmarshal = %v", got)
	}

	var rules Rules
	if err := json.Unmarshal([]byte("["+rule+"]"), &rules); err == nil {
		t.Fatal("a Rules with an unknown member decoded")
	}
}

// TestNewRules checks the list checks on rules built in code, and that the
// result does not share the caller's slices.
func TestNewRules(t *testing.T) {
	in := specRules(t)
	rules, err := NewRules(in)
	if err != nil {
		t.Fatal(err)
	}
	in[0].Access = RW
	in[1].Clients[0] = mustClients(t, "*")[0]
	if !slices.EqualFunc(rules.All(), specRules(t), Rule.Equal) {
		t.Fatalf("NewRules shares the caller's slices: %v", rules.All())
	}
	out := rules.All()
	out[2].Clients[0] = mustClients(t, "*")[0]
	if !slices.EqualFunc(rules.All(), specRules(t), Rule.Equal) {
		t.Fatalf("All shares the list's slices: %v", rules.All())
	}

	for _, tc := range []struct {
		name  string
		rules []Rule
		want  string
	}{
		{"none", nil, "no rules"},
		{"no clients", []Rule{{Access: RW}}, "rule 1: a rule names at least one client"},
		{"zero client", []Rule{{Clients: []Client{{}}, Access: RW}}, "rule 1: a rule names an empty client specification"},
		{"zero access", []Rule{{Clients: mustClients(t, "*")}}, "rule 1: invalid access type 0"},
		{"duplicate", []Rule{
			{Clients: mustClients(t, "@ops"), Access: RW},
			{Clients: mustClients(t, "@ops"), Access: RO},
		}, `rule 2: "@ops" already appears in rule 1`},
	} {
		if _, err := NewRules(tc.rules); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: NewRules error = %v, want %q", tc.name, err, tc.want)
		}
	}
	if _, err := json.Marshal(Rules{}); err == nil {
		t.Fatal("encoding the zero Rules succeeded")
	}
}

// TestThousands checks the count formatting the messages use.
func TestThousands(t *testing.T) {
	for n, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", 1204: "1,204", 1234567: "1,234,567", -1204: "-1,204"} {
		if got := thousands(n); got != want {
			t.Errorf("thousands(%d) = %q, want %q", n, got, want)
		}
	}
}
