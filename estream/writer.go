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
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/x509"
	"errors"
	"io"
	"net/http"

	"github.com/cespare/xxhash/v2"
	"github.com/secure-io/sio-go"
	"github.com/tinylib/msgp/msgp"
)

// Writer provides a stream writer.
// Streams can optionally be encrypted.
// All streams have checksum verification.
type Writer struct {
	mw  *msgp.Writer
	up  io.Writer
	err error
	key *[32]byte
}

const (
	writerMajorVersion = 2
	writerMinorVersion = 1
)

// NewWriter will return a writer that allows to add encrypted and non-encrypted data streams.
func NewWriter(w io.Writer) *Writer {
	_, err := w.Write([]byte{writerMajorVersion, writerMinorVersion})
	mw := msgp.NewWriter(w)
	return &Writer{mw: mw, err: err, up: w}
}

// Close will flush and close the output stream.
func (w *Writer) Close() error {
	if w.err != nil {
		return w.err
	}
	if err := w.addBlock(blockEOF); err != nil {
		return w.setErr(err)
	}
	return w.mw.Flush()
}

// AddKeyEncrypted will create a new encryption key and add it to the stream.
// The key will be encrypted with the public key provided.
// All following files will be encrypted with this key.
func (w *Writer) AddKeyEncrypted(publicKey *rsa.PublicKey) error {
	if w.err != nil {
		return w.err
	}
	var key [32]byte
	_, err := io.ReadFull(crand.Reader, key[:])
	if err != nil {
		return w.setErr(err)
	}
	w.key = &key
	cipherKey, err := rsa.EncryptOAEP(sha512.New(), crand.Reader, publicKey, key[:], nil)
	if err != nil {
		return w.setErr(err)
	}

	if err = w.addBlock(blockEncryptedKey); err != nil {
		return w.setErr(err)
	}

	// Write public key...
	if err := w.mw.WriteBytes(x509.MarshalPKCS1PublicKey(publicKey)); err != nil {
		return w.setErr(err)
	}

	// Write encrypted cipher key
	return w.setErr(w.mw.WriteBytes(cipherKey))
}

// AddKeyPlain will create a new encryption key and add it to the stream.
// The key will be stored without any encryption.
// All calls to AddEncryptedStream will use this key
func (w *Writer) AddKeyPlain() error {
	if w.err != nil {
		return w.err
	}
	var key [32]byte
	_, err := io.ReadFull(crand.Reader, key[:])
	if err != nil {
		return w.setErr(err)
	}
	w.key = &key

	// Write key directly to stream
	if err = w.addBlock(blockPlainKey); err != nil {
		return w.setErr(err)
	}

	return w.setErr(w.mw.WriteBytes(key[:]))
}

// AddError will indicate the writer encountered an error
// and the reader should abort the stream.
// The message will be returned as an error.
func (w *Writer) AddError(msg string) error {
	if w.err != nil {
		return w.err
	}
	if err := w.addBlock(blockError); err != nil {
		return w.setErr(err)
	}
	return w.mw.WriteString(msg)
}

// AddUnencryptedStream adds a named stream.
// Extra data can be added, which is added without encryption or checksums.
func (w *Writer) AddUnencryptedStream(name string, extra []byte) (io.WriteCloser, error) {
	if w.err != nil {
		return nil, w.err
	}

	if err := w.addBlock(blockPlainStream); err != nil {
		return nil, w.setErr(err)
	}

	// Write metadata...
	if err := w.mw.WriteString(name); err != nil {
		return nil, err
	}
	if err := w.mw.WriteBytes(extra); err != nil {
		return nil, err
	}
	if err := w.mw.WriteUint8(uint8(checksumTypeXxhash)); err != nil {
		return nil, err
	}
	return w.newStreamWriter(), nil
}

// AddEncryptedStream adds a named encrypted stream.
// AddKeyEncrypted must have been called before this, but
// multiple streams can safely use the same key.
// Extra data can be added, which is added without encryption or checksums.
func (w *Writer) AddEncryptedStream(name string, extra []byte) (io.WriteCloser, error) {
	if w.err != nil {
		return nil, w.err
	}

	if w.key == nil {
		return nil, errors.New("AddEncryptedStream: No key on stream")
	}
	if err := w.addBlock(blockEncStream); err != nil {
		return nil, w.setErr(err)
	}
	// Write metadata...
	if err := w.mw.WriteString(name); err != nil {
		return nil, err
	}
	if err := w.mw.WriteBytes(extra); err != nil {
		return nil, err
	}
	if err := w.mw.WriteUint8(uint8(checksumTypeXxhash)); err != nil {
		return nil, err
	}
	stream, err := sio.AES_256_GCM.Stream(w.key[:])
	if err != nil {
		return nil, w.setErr(err)
	}

	// Make nonce for stream.
	nonce := make([]byte, stream.NonceSize())
	if _, err := io.ReadFull(crand.Reader, nonce); err != nil {
		return nil, w.setErr(err)
	}

	// Write nonce as bin array.
	if err := w.mw.WriteBytes(nonce); err != nil {
		return nil, w.setErr(err)
	}

	// Send output as blocks.
	sw := w.newStreamWriter()
	encw := stream.EncryptWriter(sw, nonce, nil)

	return &closeWrapper{
		up: encw,
		after: func() error {
			return sw.Close()
		},
	}, nil
}

// Flush the currently written data.
func (w *Writer) Flush() error {
	if err := w.setErr(w.mw.Flush()); err != nil {
		return err
	}

	// Flush upstream if we can
	if f, ok := w.up.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}

func (w *Writer) addBlock(id blockID) error {
	return w.setErr(w.mw.WriteInt8(int8(id)))
}

func (w *Writer) newStreamWriter() *streamWriter {
	sw := &streamWriter{w: w}
	sw.h.Reset()
	return sw
}

func (w *Writer) setErr(err error) error {
	if w.err != nil {
		return w.err
	}
	if err == nil {
		return err
	}
	w.err = err
	return err
}

type streamWriter struct {
	w      *Writer
	closer io.Closer
	h      xxhash.Digest
}

func (w *streamWriter) Write(b []byte) (int, error) {
	if err := w.w.addBlock(blockDatablock); err != nil {
		return 0, err
	}
	// Update hash.
	w.h.Write(b)

	// Write data as binary array.
	return len(b), w.w.setErr(w.w.mw.WriteBytes(b))
}

// Flush adds http.Flusher support.
func (w *streamWriter) Flush() {
	_ = w.w.setErr(w.w.Flush())
}

// Close satisfies the io.Closer interface.
func (w *streamWriter) Close() error {
	if w.closer != nil {
		return w.w.setErr(w.closer.Close())
	}

	err := w.w.addBlock(blockEOS)
	if err != nil {
		return err
	}

	sum := w.h.Sum(nil)
	return w.w.setErr(w.w.mw.WriteBytes(sum))
}

type closeWrapper struct {
	before, after func() error
	up            io.WriteCloser
}

func (w *closeWrapper) Write(b []byte) (int, error) {
	return w.up.Write(b)
}

// Close satisfies the io.Closer interface.
func (w *closeWrapper) Close() error {
	if w.before != nil {
		if err := w.before(); err != nil {
			return err
		}
	}
	if err := w.up.Close(); err != nil {
		return err
	}
	if w.after != nil {
		if err := w.after(); err != nil {
			return err
		}
	}
	return nil
}
