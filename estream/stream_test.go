//
// Copyright (c) 2015-2022 MinIO, Inc.
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

package estream

import (
	"bytes"
	crand "crypto/rand"
	"crypto/rsa"
	"io"
	"os"
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
	wantDecStreams := 0
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
		wantDecStreams += 2
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
		wantDecStreams++
	}
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Read back...
	b := buf.Bytes()
	r, err := NewReader(bytes.NewBuffer(b))
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

	// Read back, but skip encrypted streams.
	r, err = NewReader(bytes.NewBuffer(b))
	if err != nil {
		t.Fatal(err)
	}
	r.SkipEncrypted(true)

	gotStreams = 0
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
	if gotStreams != wantDecStreams {
		t.Errorf("want %d streams, got %d", wantStreams, gotStreams)
	}

	gotStreams = 0
	r, err = NewReader(bytes.NewBuffer(b))
	if err != nil {
		t.Fatal(err)
	}
	r.SkipEncrypted(true)
	for {
		st, err := r.NextStream()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("stream %d: %v", gotStreams, err)
		}
		_, ok := testStreams[st.Name]
		if !ok {
			t.Fatal("unexpected stream name", st.Name)
		}
		if !bytes.Equal(st.Extra, []byte(st.Name)) {
			t.Fatal("unexpected stream extra:", st.Extra)
		}
		err = st.Skip()
		if err != nil {
			t.Fatalf("stream %d: %v", gotStreams, err)
		}
		gotStreams++
	}
	if gotStreams != wantDecStreams {
		t.Errorf("want %d streams, got %d", wantDecStreams, gotStreams)
	}

	if false {
		r, err = NewReader(bytes.NewBuffer(b))
		if err != nil {
			t.Fatal(err)
		}
		r.SkipEncrypted(true)

		err = r.DebugStream(os.Stdout)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestReplaceKeys(t *testing.T) {
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

	priv2, err := rsa.GenerateKey(crand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	var replaced bytes.Buffer
	err = ReplaceKeys(&replaced, &buf, func(key *rsa.PublicKey) (*rsa.PrivateKey, *rsa.PublicKey) {
		if key == nil {
			return nil, &priv2.PublicKey
		}
		if key.Equal(&priv.PublicKey) {
			return priv, &priv2.PublicKey
		}
		t.Fatal("unknown key\n", *key, "\nwant\n", priv.PublicKey)
		return nil, nil
	}, ReplaceKeysOptions{EncryptAll: true})
	if err != nil {
		t.Fatal(err)
	}

	// Read back...
	r, err := NewReader(&replaced)
	if err != nil {
		t.Fatal(err)
	}

	// Use key provider.
	r.PrivateKeyProvider(func(key *rsa.PublicKey) *rsa.PrivateKey {
		if key.Equal(&priv2.PublicKey) {
			return priv2
		}
		t.Fatal("unexpected public key")
		return nil
	})

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
		if st.SentEncrypted != (gotStreams&1 == 0) {
			t.Errorf("stream %d was sent with unexpected encryption %v", gotStreams, st.SentEncrypted)
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

func TestError(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.AddKeyPlain(); err != nil {
		t.Fatal(err)
	}
	want := "an error message!"
	if err := w.AddError(want); err != nil {
		t.Fatal(err)
	}
	w.Close()

	// Read back...
	r, err := NewReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	st, err := r.NextStream()
	if err == nil {
		t.Fatalf("did not receive error, got %v, err: %v", st, err)
	}
	if err.Error() != want {
		t.Errorf("Expected %q, got %q", want, err.Error())
	}
}

func TestStreamReturnNonDecryptable(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.AddKeyPlain(); err != nil {
		t.Fatal(err)
	}

	priv, err := rsa.GenerateKey(crand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	err = w.AddKeyEncrypted(&priv.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	wantStreams := len(testStreams)
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
	}
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Read back...
	b := buf.Bytes()
	r, err := NewReader(bytes.NewBuffer(b))
	if err != nil {
		t.Fatal(err)
	}
	r.ReturnNonDecryptable(true)
	gotStreams := 0
	for {
		st, err := r.NextStream()
		if err == io.EOF {
			break
		}
		if err != ErrNoKey {
			t.Fatalf("stream %d: %v", gotStreams, err)
		}
		_, ok := testStreams[st.Name]
		if !ok {
			t.Fatal("unexpected stream name", st.Name)
		}
		if !bytes.Equal(st.Extra, []byte(st.Name)) {
			t.Fatal("unexpected stream extra:", st.Extra)
		}
		if !st.SentEncrypted {
			t.Fatal("stream not marked as encrypted:", st.SentEncrypted)
		}
		err = st.Skip()
		if err != nil {
			t.Fatalf("stream %d: %v", gotStreams, err)
		}
		gotStreams++
	}
	if gotStreams != wantStreams {
		t.Errorf("want %d streams, got %d", wantStreams, gotStreams)
	}
}
