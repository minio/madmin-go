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
// Rules is the zero value. An error reading r is returned as it is.
func Parse(r io.Reader, name string) (Rules, error) {
	br := bufio.NewReader(io.LimitReader(r, MaxFileBytes+1))
	var (
		errs  LineErrors
		rules []Rule
		seen  = make(map[Client]int)
		total int
		line  int
	)
	fail := func(format string, args ...any) {
		errs = append(errs, LineError{File: name, Line: line, Msg: fmt.Sprintf(format, args...)})
	}
	for {
		text, err := br.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return Rules{}, err
		}
		if text == "" && errors.Is(err, io.EOF) {
			break
		}
		total += len(text)
		if total > MaxFileBytes {
			return Rules{}, LineErrors{{File: name, Msg: fmt.Sprintf("the file is larger than %s bytes", thousands(MaxFileBytes))}}
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

// Format prints rules as a rules file that Parse reads back as the same
// rules. header, when not empty, is printed first as a comment, one comment
// line per line of header; a byte a rules file may not hold is printed as
// '?'. Each rule follows on its own line, in evaluation order, with every
// specification in normalized form and the CLIENTS column aligned.
//
// Format takes a []Rule rather than Rules so it prints any list, including
// an empty one, which yields the header alone. A rule that would not pass its
// own checks is an error, and nothing is written.
func Format(w io.Writer, header string, rules []Rule) error {
	clients := make([]string, len(rules))
	width := 0
	for i, rule := range rules {
		if err := rule.check(); err != nil {
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
