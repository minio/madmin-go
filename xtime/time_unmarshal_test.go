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
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/tinylib/msgp/msgp"
	"gopkg.in/yaml.v3"
)

type testDuration struct {
	A               string    `yaml:"a" json:"a"`
	Dur             Duration  `yaml:"dur" json:"dur"`
	DurationPointer *Duration `yaml:"durationPointer" json:"durationPointer"`
}

func TestDuration_Unmarshal(t *testing.T) {
	jsonData := []byte(`{"a":"1s","dur":"1w1s","durationPointer":"7d1s"}`)
	yamlData := []byte(`a: 1s
dur: 1w1s
durationPointer: 7d1s`)
	yamlTest := testDuration{}
	if err := yaml.Unmarshal(yamlData, &yamlTest); err != nil {
		t.Fatal(err)
	}
	jsonTest := testDuration{}
	if err := json.Unmarshal(jsonData, &jsonTest); err != nil {
		t.Fatal(err)
	}

	jsonData = []byte(`{"a":"1s","dur":"1w1s"}`)
	yamlData = []byte(`a: 1s
dur: 1w1s`)

	if err := yaml.Unmarshal(yamlData, &yamlTest); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(jsonData, &jsonTest); err != nil {
		t.Fatal(err)
	}
}

func TestMarshalUnmarshalDuration(t *testing.T) {
	v := Duration(time.Hour)
	var vn Duration
	bts, err := v.MarshalMsg(nil)
	if err != nil {
		t.Fatal(err)
	}
	left, err := vn.UnmarshalMsg(bts)
	if err != nil {
		t.Fatal(err)
	}
	if len(left) > 0 {
		t.Errorf("%d bytes left over after UnmarshalMsg(): %q", len(left), left)
	}
	if vn != v {
		t.Errorf("v=%#v; want=%#v", vn, v)
	}

	left, err = msgp.Skip(bts)
	if err != nil {
		t.Fatal(err)
	}
	if len(left) > 0 {
		t.Errorf("%d bytes left over after Skip(): %q", len(left), left)
	}
}

func TestEncodeDecodeDuration(t *testing.T) {
	v := Duration(time.Hour)
	var buf bytes.Buffer
	msgp.Encode(&buf, &v)

	m := v.Msgsize()
	if buf.Len() > m {
		t.Log("WARNING: TestEncodeDecodeDuration Msgsize() is inaccurate")
	}

	var vn Duration
	err := msgp.Decode(&buf, &vn)
	if err != nil {
		t.Error(err)
	}
	if vn != v {
		t.Errorf("v=%#v; want=%#v", vn, v)
	}
	buf.Reset()
	msgp.Encode(&buf, &v)
	err = msgp.NewReader(&buf).Skip()
	if err != nil {
		t.Error(err)
	}
}

func TestDuration_Marshal(t *testing.T) {
	type testDuration struct {
		A               Duration  `json:"a" yaml:"a"`
		Dur             Duration  `json:"dur" yaml:"dur"`
		DurationPointer *Duration `json:"durationPointer,omitempty" yaml:"durationPointer,omitempty"`
	}

	d1 := Duration(time.Second)
	d2 := Duration(0)
	d3 := Duration(time.Hour*24*7 + time.Second)

	testData := testDuration{
		A:               d1,
		Dur:             d2,
		DurationPointer: &d3,
	}

	yamlData, err := yaml.Marshal(&testData)
	if err != nil {
		t.Fatalf("Failed to marshal YAML: %v", err)
	}

	expected := `a: 1s
dur: 0s
durationPointer: 168h0m1s
`
	if string(yamlData) != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, string(yamlData))
	}
}
