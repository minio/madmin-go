//
// MinIO Object Storage (c) 2022 MinIO, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package estream

import (
	"bytes"
	crand "crypto/rand"
	"crypto/rsa"
	"io"
	"testing"
)

var testStreams = map[string][]byte{
	"stream1": bytes.Repeat([]byte("a"), 2000),
	"stream2": bytes.Repeat([]byte("b"), 1<<20),
	"stream3": bytes.Repeat([]byte("b"), 5),
	"empty":   {},
}

func TestStreamRoundtrip(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.AddKeyPlain(); err != nil {
		t.Fatal(err)
	}
	wantStreams := 0
	for name, value := range testStreams {
		st, err := w.AddEncryptedStream(name, []byte(name))
		if err != nil {
			t.Fatal(err)
		}
		_, err = io.Copy(st, bytes.NewBuffer(value))
		if err != nil {
			t.Fatal(err)
		}
		st.Close()
		st, err = w.AddUnencryptedStream(name, []byte(name))
		if err != nil {
			t.Fatal(err)
		}
		_, err = io.Copy(st, bytes.NewBuffer(value))
		if err != nil {
			t.Fatal(err)
		}
		st.Close()
		wantStreams += 2
	}

	priv, err := rsa.GenerateKey(crand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	err = w.AddKeyEncrypted(&priv.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range testStreams {
		st, err := w.AddEncryptedStream(name, []byte(name))
		if err != nil {
			t.Fatal(err)
		}
		_, err = io.Copy(st, bytes.NewBuffer(value))
		if err != nil {
			t.Fatal(err)
		}
		st.Close()
		st, err = w.AddUnencryptedStream(name, []byte(name))
		if err != nil {
			t.Fatal(err)
		}
		_, err = io.Copy(st, bytes.NewBuffer(value))
		if err != nil {
			t.Fatal(err)
		}
		st.Close()
		wantStreams += 2
	}
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Read back...
	r, err := NewReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	r.SetPrivateKey(priv)
	var gotStreams int
	for {
		st, err := r.NextStream()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("stream %d: %v", gotStreams, err)
		}
		want, ok := testStreams[st.Name]
		if !ok {
			t.Fatal("unexpected stream name", st.Name)
		}
		if !bytes.Equal(st.Extra, []byte(st.Name)) {
			t.Fatal("unexpected stream extra:", st.Extra)
		}
		got, err := io.ReadAll(st)
		if err != nil {
			t.Fatalf("stream %d: %v", gotStreams, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("stream %d: content mismatch (len %d,%d)", gotStreams, len(got), len(want))
		}
		gotStreams++
	}
	if gotStreams != wantStreams {
		t.Errorf("want %d streams, got %d", wantStreams, gotStreams)
	}
}
