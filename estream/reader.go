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
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/cespare/xxhash/v2"
	"github.com/secure-io/sio-go"
	"github.com/tinylib/msgp/msgp"
)

type Reader struct {
	mr            *msgp.Reader
	majorV        uint8
	minorV        uint8
	err           error
	inStream      bool
	key           *[32]byte
	private       *rsa.PrivateKey
	privateFn     func(key *rsa.PublicKey) *rsa.PrivateKey
	skipEncrypted bool
}

// NewReader will return a Reader that will split streams.
func NewReader(r io.Reader) (*Reader, error) {
	var ver [2]byte
	if _, err := io.ReadFull(r, ver[:]); err != nil {
		return nil, err
	}
	switch ver[0] {
	case 2:
	default:
		return nil, fmt.Errorf("unknown stream version: 0x%x", ver[0])
	}

	return &Reader{mr: msgp.NewReader(r), majorV: ver[0], minorV: ver[1]}, nil
}

// SetPrivateKey will set the private key to allow stream decryption.
// This overrides any function set by PrivateKeyProvider.
func (r *Reader) SetPrivateKey(k *rsa.PrivateKey) {
	r.privateFn = nil
	r.private = k
}

// PrivateKeyProvider will ask for a private key matching the public key.
// If the function returns a nil private key the stream key will not be decrypted.
// This overrides any key set by SetPrivateKey.
func (r *Reader) PrivateKeyProvider(fn func(key *rsa.PublicKey) *rsa.PrivateKey) {
	r.privateFn = fn
	r.private = nil
}

// SkipEncrypted will skip encrypted streams if no private key has been set.
func (r *Reader) SkipEncrypted() {
	r.skipEncrypted = true
}

// Stream returns the next stream.
type Stream struct {
	io.Reader
	Name  string
	Extra []byte
}

// NextStream will return the next stream.
// Before calling this the previous stream must be read until EOF.
// Will return nil, io.EOF when there are no more streams.
func (r *Reader) NextStream() (*Stream, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.inStream {
		return nil, errors.New("previous stream not read until EOF")
	}
	for {
		// Read block ID.
		n, err := r.mr.ReadInt8()
		if err != nil {
			return nil, r.setErr(err)
		}
		id := blockID(n)
		switch id {
		case blockPlainKey:
			key, err := r.mr.ReadBytes(nil)
			if err != nil {
				return nil, r.setErr(err)
			}
			if len(key) != 32 {
				return nil, r.setErr(fmt.Errorf("unexpected key length: %d", len(key)))
			}
			r.key = (*[32]byte)(key)
		case blockEncryptedKey:
			// Read public key
			publicKey, err := r.mr.ReadBytes(nil)
			if err != nil {
				return nil, r.setErr(err)
			}

			// Request private key if we have a custom function.
			if r.privateFn != nil {
				pk, err := x509.ParsePKCS1PublicKey(publicKey)
				if err != nil {
					return nil, r.setErr(err)
				}
				r.private = r.privateFn(pk)
				if r.private == nil {
					return nil, r.setErr(errors.New("nil private key returned"))
				}
			}

			// Read cipher key
			cipherKey, err := r.mr.ReadBytes(nil)
			if err != nil {
				return nil, r.setErr(err)
			}
			if r.private == nil {
				if r.skipEncrypted {
					continue
				}
				return nil, r.setErr(errors.New("private key has not been set"))
			}

			// Decrypt stream key
			key, err := rsa.DecryptOAEP(sha512.New(), rand.Reader, r.private, cipherKey, nil)
			if err != nil {
				return nil, err
			}

			if len(key) != 32 {
				return nil, r.setErr(fmt.Errorf("unexpected key length: %d", len(key)))
			}
			r.key = (*[32]byte)(key)
		case blockPlainStream, blockEncStream:
			name, err := r.mr.ReadString()
			if err != nil {
				return nil, r.setErr(err)
			}
			extra, err := r.mr.ReadBytes(nil)
			if err != nil {
				return nil, r.setErr(err)
			}
			if id == blockPlainStream {
				return &Stream{
					Reader: r.newStreamReader(),
					Name:   name,
					Extra:  extra,
				}, nil
			}
			if r.key == nil {
				if r.skipEncrypted {
					if err := r.skipDataBlocks(); err != nil {
						return nil, r.setErr(err)
					}
					continue
				}
				return nil, r.setErr(errors.New("key has not been received"))
			}
			// Read stream nonce
			nonce, err := r.mr.ReadBytes(nil)
			if err != nil {
				return nil, r.setErr(err)
			}
			stream, err := sio.AES_256_GCM.Stream(r.key[:])
			if err != nil {
				return nil, r.setErr(err)
			}

			// Check if nonce is expected length.
			if len(nonce) != stream.NonceSize() {
				return nil, r.setErr(fmt.Errorf("unexpected nonce length: %d", len(nonce)))
			}

			encr := stream.DecryptReader(r.newStreamReader(), nonce, nil)
			return &Stream{
				Reader: encr,
				Name:   name,
				Extra:  extra,
			}, nil

		case blockEOF:
			return nil, io.EOF
		case blockError:
			msg, err := r.mr.ReadString()
			if err != nil {
				return nil, r.setErr(err)
			}
			return nil, r.setErr(errors.New(msg))
		default:
			if err := r.skipBlock(id); err != nil {
				return nil, r.setErr(err)
			}
		}
	}
}

func (r *Reader) skipDataBlocks() error {
	for {
		// Read block ID.
		n, err := r.mr.ReadInt8()
		if err != nil {
			return err
		}
		id := blockID(n)
		switch id {
		case blockDatablock:
			// Skip data
			if err := r.mr.Skip(); err != nil {
				return err
			}
		case blockEOS:
			// Skip hash
			t, err := r.mr.ReadUint8()
			if err != nil {
				return err
			}
			r.inStream = false
			if t != checksumTypeNone {
				return r.mr.Skip()
			}
			return nil
		case blockError:
			msg, err := r.mr.ReadString()
			if err != nil {
				return err
			}
			return errors.New(msg)
		default:
			if err := r.skipBlock(id); err != nil {
				return err
			}
		}
	}
}

func (r *Reader) skipBlock(id blockID) error {
	if id >= 0 {
		return fmt.Errorf("unknown block type: %d", id)
	}

	// Negative is a skippable block. Read size, skip it.
	skip, err := r.mr.ReadUint32()
	if err != nil {
		return err
	}
	_, err = io.Copy(io.Discard, io.LimitReader(r.mr, int64(skip)))
	return err
}

func (r *Reader) setErr(err error) error {
	if r.err != nil {
		return r.err
	}
	if err == nil {
		return err
	}
	if errors.Is(err, io.EOF) {
		r.err = io.ErrUnexpectedEOF
	}
	r.err = err
	return err
}

type streamReader struct {
	up    *Reader
	h     xxhash.Digest
	buf   bytes.Buffer
	tmp   []byte
	isEOF bool
}

func (r *Reader) newStreamReader() *streamReader {
	sr := &streamReader{up: r}
	sr.h.Reset()
	r.inStream = true
	return sr
}

func (r *streamReader) Read(b []byte) (int, error) {
	if r.isEOF {
		return 0, io.EOF
	}
	if r.up.err != nil {
		return 0, r.up.err
	}
	for {
		// If we have anything in the buffer return that first.
		if r.buf.Len() > 0 {
			n, err := r.buf.Read(b)
			if err == io.EOF {
				err = nil
			}
			return n, r.up.setErr(err)
		}

		// Read block
		n, err := r.up.mr.ReadInt8()
		if err != nil {
			return 0, r.up.setErr(err)
		}
		id := blockID(n)
		switch id {
		case blockDatablock:
			// Read block
			buf, err := r.up.mr.ReadBytes(r.tmp[:0])
			if err != nil {
				return 0, r.up.setErr(err)
			}

			// Write to buffer and checksum
			r.h.Write(buf)
			r.tmp = buf
			r.buf.Write(buf)
		case blockEOS:
			// Verify stream checksum if any.
			checksum, err := r.up.mr.ReadUint8()
			if err != nil {
				return 0, r.up.setErr(err)
			}
			switch checksum {
			case checksumTypeXxhash:
				hash, err := r.up.mr.ReadBytes(nil)
				if err != nil {
					return 0, r.up.setErr(err)
				}
				got := r.h.Sum(nil)
				if !bytes.Equal(hash, got) {
					return 0, r.up.setErr(fmt.Errorf("checksum mismatch, want %s, got %s", hex.EncodeToString(hash), hex.EncodeToString(got)))
				}
			case checksumTypeNone:
			default:
				return 0, r.up.setErr(fmt.Errorf("unknown checksum id %d", checksum))
			}
			r.isEOF = true
			r.up.inStream = false
			return 0, io.EOF
		case blockError:
			msg, err := r.up.mr.ReadString()
			if err != nil {
				return 0, r.up.setErr(err)
			}
			return 0, r.up.setErr(errors.New(msg))
		default:
			if err := r.up.skipBlock(id); err != nil {
				return 0, r.up.setErr(err)
			}
		}
	}
}
