//
// Copyright (c) 2015-2024 MinIO, Inc.
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

func TestSetMultiplePrivateKeys(t *testing.T) {
	priv1, err := rsa.GenerateKey(crand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	priv2, err := rsa.GenerateKey(crand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	// Write each test stream twice: once encrypted under priv1's key and
	// once under priv2's key. AddKeyEncrypted switches the key for all
	// following streams.
	var buf bytes.Buffer
	w := NewWriter(&buf)
	for _, pub := range []*rsa.PublicKey{&priv1.PublicKey, &priv2.PublicKey} {
		if err := w.AddKeyEncrypted(pub); err != nil {
			t.Fatal(err)
		}
		for name, value := range testStreams {
			st, err := w.AddEncryptedStream(name, []byte(name))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := io.Copy(st, bytes.NewBuffer(value)); err != nil {
				t.Fatal(err)
			}
			if err := st.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	wantStreams := 2 * len(testStreams)
	encoded := buf.Bytes()

	readAll := func(t *testing.T, setup func(r *Reader)) error {
		t.Helper()
		r, err := NewReader(bytes.NewBuffer(encoded))
		if err != nil {
			t.Fatal(err)
		}
		setup(r)
		n := 0
		for {
			st, err := r.NextStream()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			want, ok := testStreams[st.Name]
			if !ok {
				t.Fatalf("unexpected stream name %q", st.Name)
			}
			got, err := io.ReadAll(st)
			if err != nil {
				return err
			}
			if !bytes.Equal(got, want) {
				t.Errorf("stream %d (%s): content mismatch (len %d,%d)", n, st.Name, len(got), len(want))
			}
			n++
		}
		if n != wantStreams {
			t.Errorf("want %d streams, got %d", wantStreams, n)
		}
		return nil
	}

	// Both keys supplied in a single variadic call.
	t.Run("Variadic", func(t *testing.T) {
		if err := readAll(t, func(r *Reader) { r.SetPrivateKey(priv1, priv2) }); err != nil {
			t.Fatal(err)
		}
	})

	// Repeated calls must accumulate keys, not override the previous one.
	t.Run("Appended", func(t *testing.T) {
		if err := readAll(t, func(r *Reader) {
			r.SetPrivateKey(priv1)
			r.SetPrivateKey(priv2)
		}); err != nil {
			t.Fatal(err)
		}
	})

	// Sanity check: a single key cannot decrypt streams under the other key.
	t.Run("SingleKeyInsufficient", func(t *testing.T) {
		if err := readAll(t, func(r *Reader) { r.SetPrivateKey(priv1) }); err == nil {
			t.Fatal("expected failure reading streams encrypted under the second key")
		}
	})
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

func TestStreamRoundtripCompressed(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	w.WithCompression()
	if err := w.AddKeyPlain(); err != nil {
		t.Fatal(err)
	}

	wantStreams := 0
	for name, value := range testStreams {
		for _, enc := range []bool{true, false} {
			var st io.WriteCloser
			var err error
			if enc {
				st, err = w.AddEncryptedStream(name, []byte(name))
			} else {
				st, err = w.AddUnencryptedStream(name, []byte(name))
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = io.Copy(st, bytes.NewBuffer(value)); err != nil {
				t.Fatal(err)
			}
			if err = st.Close(); err != nil {
				t.Fatal(err)
			}
			wantStreams++
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	// Read back...
	b := buf.Bytes()
	r, err := NewReader(bytes.NewBuffer(b))
	if err != nil {
		t.Fatal(err)
	}

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
			t.Fatalf("stream %d (%s): %v", gotStreams, st.Name, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("stream %d (%s): content mismatch (len %d,%d)", gotStreams, st.Name, len(got), len(want))
		}
		gotStreams++
	}
	if gotStreams != wantStreams {
		t.Errorf("want %d streams, got %d", wantStreams, gotStreams)
	}

	// Read back, but Skip() every other stream. This exercises Skip() on the
	// compressed reader wrappers (drainReader / minlz over sio): after a Skip
	// the underlying *streamReader must be reset so the following streams still
	// read correctly.
	r, err = NewReader(bytes.NewBuffer(b))
	if err != nil {
		t.Fatal(err)
	}
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
		if gotStreams%2 == 0 {
			if err := st.Skip(); err != nil {
				t.Fatalf("stream %d (%s): skip: %v", gotStreams, st.Name, err)
			}
			// A read after Skip must not leak data and must not advance the
			// parent reader into the following stream's blocks. The wrapping
			// decompressor/decryptor may surface its own truncation error, but
			// it must never return data; correctness of the next stream below
			// proves the parent position was left intact.
			if n, _ := st.Read(make([]byte, 32)); n != 0 {
				t.Fatalf("stream %d (%s): read after skip returned %d bytes, want 0", gotStreams, st.Name, n)
			}
		} else {
			got, err := io.ReadAll(st)
			if err != nil {
				t.Fatalf("stream %d (%s): %v", gotStreams, st.Name, err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("stream %d (%s): content mismatch (len %d,%d)", gotStreams, st.Name, len(got), len(want))
			}
		}
		gotStreams++
	}
	if gotStreams != wantStreams {
		t.Errorf("want %d streams, got %d", wantStreams, gotStreams)
	}

	// DebugStream must understand the compressed stream block IDs.
	r, err = NewReader(bytes.NewBuffer(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := r.DebugStream(io.Discard); err != nil {
		t.Fatalf("DebugStream: %v", err)
	}
}
