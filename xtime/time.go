// Copyright (c) 2015-2024 MinIO, Inc.
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package xtime

import (
	"fmt"
	"time"

	"github.com/tinylib/msgp/msgp"
	"gopkg.in/yaml.v3"
)

// Additional durations, a day is considered to be 24 hours
const (
	Day  time.Duration = time.Hour * 24
	Week               = Day * 7
)

var unitMap = map[string]int64{
	"ns": int64(time.Nanosecond),
	"us": int64(time.Microsecond),
	"µs": int64(time.Microsecond), // U+00B5 = micro symbol
	"μs": int64(time.Microsecond), // U+03BC = Greek letter mu
	"ms": int64(time.Millisecond),
	"s":  int64(time.Second),
	"m":  int64(time.Minute),
	"h":  int64(time.Hour),
	"d":  int64(Day),
	"w":  int64(Week),
}

// ParseDuration parses a duration string.
// The following code is borrowed from time.ParseDuration
// https://cs.opensource.google/go/go/+/refs/tags/go1.22.5:src/time/format.go;l=1589
// This function extends this function by allowing support for days and weeks.
// This function must only be used when days and weeks are necessary inputs
// in all other cases it is preferred that a user uses Go's time.ParseDuration
func ParseDuration(s string) (time.Duration, error) {
	dur, err := time.ParseDuration(s) // Parse via standard Go, if success return right away.
	if err == nil {
		return dur, nil
	}
	return parseDuration(s)
}

// Duration is a wrapper around time.Duration that supports YAML and JSON
type Duration time.Duration

// D will return as a time.Duration.
func (d Duration) D() time.Duration {
	return time.Duration(d)
}

// UnmarshalYAML implements yaml.Unmarshaler
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		dur, err := ParseDuration(value.Value)
		if err != nil {
			return err
		}
		*d = Duration(dur)
		return nil
	}
	return fmt.Errorf("unable to unmarshal %s", value.Tag)
}

// UnmarshalJSON implements json.Unmarshaler
func (d *Duration) UnmarshalJSON(bs []byte) error {
	if len(bs) <= 2 {
		return nil
	}
	dur, err := ParseDuration(string(bs[1 : len(bs)-1]))
	if err != nil {
		return err
	}
	*d = Duration(dur)
	return nil
}

// MarshalMsg appends the marshaled form of the object to the provided
// byte slice, returning the extended slice and any errors encountered.
func (d Duration) MarshalMsg(bytes []byte) ([]byte, error) {
	return msgp.AppendInt64(bytes, int64(d)), nil
}

// UnmarshalMsg unmarshals the object from binary,
// returing any leftover bytes and any errors encountered.
func (d *Duration) UnmarshalMsg(b []byte) ([]byte, error) {
	i, rem, err := msgp.ReadInt64Bytes(b)
	*d = Duration(i)
	return rem, err
}

// EncodeMsg writes itself as MessagePack using a *msgp.Writer.
func (d Duration) EncodeMsg(w *msgp.Writer) error {
	return w.WriteInt64(int64(d))
}

// DecodeMsg decodes itself as MessagePack using a *msgp.Reader.
func (d *Duration) DecodeMsg(reader *msgp.Reader) error {
	i, err := reader.ReadInt64()
	*d = Duration(i)
	return err
}

// Msgsize returns the maximum serialized size in bytes.
func (d Duration) Msgsize() int {
	return msgp.Int64Size
}

// MarshalYAML implements yaml.Marshaler - Converts duration to human-readable format (e.g., "2h", "30m")
func (d Duration) MarshalYAML() (interface{}, error) {
	return time.Duration(d).String(), nil
}
