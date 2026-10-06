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
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// MaxFileBytes is the largest rules file Parse reads. A file of MaxRules
// rules, each with a long client list and a comment, fits well inside it.
const MaxFileBytes = 4 << 20

// LineError is one error in a rules file. Line is 1-based, and zero for an
// error about the whole file, such as the number of rules.
type LineError struct {
	File string
	Line int
	Msg  string
}

// Error returns "FILE:LINE: MSG", or "FILE: MSG" for an error about the whole
// file. An empty File is left out.
func (e LineError) Error() string {
	var where string
	switch {
	case e.File != "" && e.Line > 0:
		where = fmt.Sprintf("%s:%d: ", e.File, e.Line)
	case e.File != "":
		where = e.File + ": "
	case e.Line > 0:
		where = fmt.Sprintf("line %d: ", e.Line)
	}
	return where + e.Msg
}

// LineErrors is every error Parse found, in file order, with any error about
// the whole file last.
type LineErrors []LineError

// Error returns one error per line.
func (e LineErrors) Error() string {
	lines := make([]string, len(e))
	for i, le := range e {
		lines[i] = le.Error()
	}
	return strings.Join(lines, "\n")
}

// Parse reads a rules file and checks the whole of it. name labels the
// errors, typically the file's path, or "-" for standard input.
//
// The file is either the text form below or JSON. JSON is either an array of
// rules, as Rules.UnmarshalJSON reads, or an object whose "rules" member is
// that array; its other members are ignored, so the JSON mc prints for an
// export's rules can be edited and read back. JSON is told from text by its
// first byte other than white space: '{', or '[' followed by '{' or ']'. A
// text rule may start with '[', as a hostname pattern does, but no pattern
// class starts with '{' or ']'.
//
// A rule line holds two fields separated by spaces or tabs: CLIENTS, one or
// more client specifications joined by commas, then ACCESS, which is rw, ro
// or none. '#' starts a comment that runs to the end of the line. Blank lines
// and comment-only lines are skipped, and so are leading and trailing spaces
// and tabs. Lines end in LF or CRLF. The text is ASCII, and any other byte,
// or a control character other than a tab, is refused.
//
// The rules are in evaluation order: the first rule line is rule 1. The file
// holds 1 to MaxRules rules, and no client specification appears twice,
// compared in normalized form. A file with no rules is refused, because that
// is what a wrong path or a truncated file looks like.
//
// On failure the error is a LineErrors holding every error found, and the
// Rules is the zero value. An error in a JSON file carries no line; it names
// the rule by its 1-based position instead. An error reading r is returned as
// it is.
func Parse(r io.Reader, name string) (Rules, error) {
	b, err := io.ReadAll(io.LimitReader(r, MaxFileBytes+1))
	if err != nil {
		return Rules{}, err
	}
	if len(b) > MaxFileBytes {
		return Rules{}, LineErrors{{File: name, Msg: fmt.Sprintf("the file is larger than %s bytes", thousands(MaxFileBytes))}}
	}
	if isJSON(b) {
		return parseJSON(b, name)
	}
	return parseText(b, name)
}

// isJSON reports whether a rules file is JSON rather than text: its first
// byte other than white space is '{', or '[' followed by '{' or ']'.
func isJSON(b []byte) bool {
	b = bytes.TrimLeft(b, " \t\r\n")
	if len(b) == 0 {
		return false
	}
	switch b[0] {
	case '{':
		return true
	case '[':
		b = bytes.TrimLeft(b[1:], " \t\r\n")
		return len(b) > 0 && (b[0] == '{' || b[0] == ']')
	}
	return false
}

// parseJSON reads a JSON rules file: an array of rules, or an object whose
// "rules" member is one.
func parseJSON(b []byte, name string) (Rules, error) {
	fail := func(err error) (Rules, error) {
		flat := flatten(err)
		errs := make(LineErrors, len(flat))
		for i, e := range flat {
			errs[i] = LineError{File: name, Msg: e.Error()}
		}
		return Rules{}, errs
	}

	dec := json.NewDecoder(bytes.NewReader(b))
	var v json.RawMessage
	if err := dec.Decode(&v); err != nil {
		return fail(fmt.Errorf("the file is not valid JSON: %w", err))
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return fail(errors.New("the file holds more than one JSON value"))
	}

	if v[0] == '{' {
		var obj struct {
			Rules json.RawMessage `json:"rules"`
		}
		if err := json.Unmarshal(v, &obj); err != nil {
			return fail(fmt.Errorf("the file is not valid JSON: %w", err))
		}
		if len(obj.Rules) == 0 || string(obj.Rules) == "null" {
			return fail(errors.New(`a JSON rules file is an array of rules, or an object with a "rules" member holding one`))
		}
		v = obj.Rules
	}
	var rules Rules
	if err := rules.UnmarshalJSON(v); err != nil {
		return fail(err)
	}
	return rules, nil
}

// parseText reads a rules file in the text form.
func parseText(b []byte, name string) (Rules, error) {
	br := bufio.NewReader(bytes.NewReader(b))
	var (
		errs  LineErrors
		rules []Rule
		seen  = make(map[Client]int)
		line  int
	)
	fail := func(format string, args ...any) {
		errs = append(errs, LineError{File: name, Line: line, Msg: fmt.Sprintf(format, args...)})
	}
	for {
		text, err := br.ReadString('\n')
		if text == "" && errors.Is(err, io.EOF) {
			break
		}
		line++

		text = strings.TrimSuffix(strings.TrimSuffix(text, "\n"), "\r")
		if bad := badByte(text); bad >= 0 {
			if text[bad] >= 0x80 {
				fail("byte 0x%02x is not ASCII; a rules file is ASCII text", text[bad])
			} else {
				fail("control character 0x%02x is not allowed; a rules file is ASCII text", text[bad])
			}
		} else if rule, ok := parseLine(text, line, seen, fail); ok {
			rules = append(rules, rule)
		}
		if errors.Is(err, io.EOF) {
			break
		}
	}

	line = 0
	if len(errs) == 0 && len(rules) == 0 {
		fail("the file holds no rules")
	} else if len(rules) > MaxRules {
		fail("%s rules; at most %s are allowed", thousands(len(rules)), thousands(MaxRules))
	}
	if len(errs) > 0 {
		return Rules{}, errs
	}
	return Rules{rules: rules}, nil
}

// badByte returns the index of the first byte a rules file may not hold, or
// -1. A tab is the only control character allowed.
func badByte(s string) int {
	for i := 0; i < len(s); i++ {
		if c := s[i]; c >= 0x7f || (c < 0x20 && c != '\t') {
			return i
		}
	}
	return -1
}

// parseLine reads one line of a rules file. ok is false for a line holding no
// rule, and for a rule line with an error, which it reports through fail.
// seen maps each specification already read to its line, and gains this
// line's.
func parseLine(text string, line int, seen map[Client]int, fail func(string, ...any)) (rule Rule, ok bool) {
	if i := strings.IndexByte(text, '#'); i >= 0 {
		text = text[:i]
	}
	fields := strings.FieldsFunc(text, func(r rune) bool { return r == ' ' || r == '\t' })
	switch len(fields) {
	case 0:
		return Rule{}, false
	case 1:
		fail("want two fields, CLIENTS then ACCESS; found only %q", fields[0])
		return Rule{}, false
	case 2:
	default:
		fail("want two fields, CLIENTS then ACCESS; found %d (client specifications are joined by commas, with no spaces)", len(fields))
		return Rule{}, false
	}

	valid := true
	access, err := ParseAccess(fields[1])
	if err != nil {
		fail("%v", err)
		valid = false
	}
	var clients []Client
	for spec := range strings.SplitSeq(fields[0], ",") {
		c, err := ParseClient(spec)
		if err != nil {
			fail("%v", err)
			valid = false
			continue
		}
		if first, dup := seen[c]; dup {
			fail("%s already appears on line %d; this rule can never match it", describe(spec, c), first)
			valid = false
			continue
		}
		seen[c] = line
		clients = append(clients, c)
	}
	if !valid {
		return Rule{}, false
	}
	return Rule{Clients: clients, Access: access}, true
}

// describe quotes a specification as written, adding its normalized form
// when the two differ, so a duplicate spelled two ways is plain.
func describe(spec string, c Client) string {
	if spec == c.String() {
		return fmt.Sprintf("%q", spec)
	}
	return fmt.Sprintf("%q (%s)", spec, c.String())
}

// ruleGap is the space between the CLIENTS and ACCESS columns of a printed
// rules file.
const ruleGap = 4

// Format prints rules as a rules file. header, when not empty, is printed
// first as a comment, one comment line per line of header; a byte a rules
// file may not hold is printed as '?'. Each rule follows on its own line, in
// evaluation order, with every specification in normalized form and the
// CLIENTS column aligned.
//
// Format takes a []Rule rather than Rules so it prints any list, including
// an empty one, which yields the header alone. It checks each rule on its
// own: a rule that would not pass its own checks is an error, and nothing is
// written. So is a rule carrying a value this version does not know, which
// only a reply holds: a rules file cannot express it, and printing the rule
// without it would drop it. It does not check the list as a whole, so Parse reads the output
// back as the same rules only when the list is one NewRules accepts: 1 to
// MaxRules rules, naming no client specification twice. Parse refuses the
// header alone that an empty list prints.
func Format(w io.Writer, header string, rules []Rule) error {
	clients := make([]string, len(rules))
	width := 0
	for i, rule := range rules {
		if err := rule.check(); err != nil {
			return fmt.Errorf("rule %d: %w", i+1, err)
		}
		if err := rule.checkKnown(); err != nil {
			return fmt.Errorf("rule %d: %w", i+1, err)
		}
		names := make([]string, len(rule.Clients))
		for j, c := range rule.Clients {
			names[j] = c.String()
		}
		clients[i] = strings.Join(names, ",")
		width = max(width, len(clients[i]))
	}

	var sb strings.Builder
	if header != "" {
		for l := range strings.SplitSeq(strings.TrimRight(header, "\r\n"), "\n") {
			l = strings.TrimSuffix(l, "\r")
			sb.WriteString(strings.TrimRight("# "+sanitize(l), " \t"))
			sb.WriteByte('\n')
		}
	}
	for i, rule := range rules {
		fmt.Fprintf(&sb, "%-*s%*s%s\n", width, clients[i], ruleGap, "", rule.Access)
	}
	_, err := io.WriteString(w, sb.String())
	return err
}

// sanitize replaces each byte a rules file may not hold with '?'.
func sanitize(s string) string {
	if badByte(s) < 0 {
		return s
	}
	b := []byte(s)
	for i, c := range b {
		if c >= 0x7f || (c < 0x20 && c != '\t') {
			b[i] = '?'
		}
	}
	return string(b)
}
