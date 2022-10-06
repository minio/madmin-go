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

	block := make([]byte, 1024)
	for {
		// Read block ID.
		n, err := r.mr.ReadInt8()
		if err != nil {
			return nil, r.setErr(err)
		}
		id := blockID(n)
		sz, err := r.mr.ReadUint32()
		if err != nil {
			return nil, r.setErr(err)
		}
		if cap(block) < int(sz) {
			block = make([]byte, sz)
		}
		block = block[:sz]
		_, err = io.ReadFull(r.mr, block)
		if err != nil {
			return nil, r.setErr(err)
		}

		switch id {
		case blockPlainKey:
			key, _, err := msgp.ReadBytesZC(block)
			if err != nil {
				return nil, r.setErr(err)
			}
			if len(key) != 32 {
				return nil, r.setErr(fmt.Errorf("unexpected key length: %d", len(key)))
			}
			r.key = (*[32]byte)(key)
		case blockEncryptedKey:
			// Read public key
			publicKey, block, err := msgp.ReadBytesZC(block)
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
			cipherKey, _, err := msgp.ReadBytesZC(block)
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
			name, block, err := msgp.ReadStringBytes(block)
			if err != nil {
				return nil, r.setErr(err)
			}
			extra, block, err := msgp.ReadBytesZC(block)
			if err != nil {
				return nil, r.setErr(err)
			}
			c, block, err := msgp.ReadUint8Bytes(block)
			if err != nil {
				return nil, r.setErr(err)
			}
			checksum := checksumType(c)
			if !checksum.valid() {
				return nil, r.setErr(fmt.Errorf("unknown checksum type %d", checksum))
			}

			if id == blockPlainStream {
				return &Stream{
					Reader: r.newStreamReader(checksum),
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
			nonce, _, err := msgp.ReadBytesZC(block)
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

			encr := stream.DecryptReader(r.newStreamReader(checksum), nonce, nil)
			return &Stream{
				Reader: encr,
				Name:   name,
				Extra:  extra,
			}, nil

		case blockEOF:
			return nil, io.EOF
		case blockError:
			msg, _, err := msgp.ReadStringBytes(block)
			if err != nil {
				return nil, r.setErr(err)
			}
			return nil, r.setErr(errors.New(msg))
		default:
			if id >= 0 {
				return nil, fmt.Errorf("unknown block type: %d", id)
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
		sz, err := r.mr.ReadUint32()
		if err != nil {
			return err
		}
		if id == blockError {
			msg, err := r.mr.ReadString()
			if err != nil {
				return err
			}
			return errors.New(msg)
		}
		// Discard data
		_, err = io.CopyN(io.Discard, r.mr, int64(sz))
		if err != nil {
			return err
		}
		switch id {
		case blockDatablock:
			// Skip data
		case blockEOS:
			// Done
			r.inStream = false
			return nil
		default:
			if id >= 0 {
				return fmt.Errorf("unknown block type: %d", id)
			}
		}
	}
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
	check checksumType
}

func (r *Reader) newStreamReader(ct checksumType) *streamReader {
	sr := &streamReader{up: r, check: ct}
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

		// Read size...
		sz, err := r.up.mr.ReadUint32()
		if err != nil {
			return 0, r.up.setErr(err)
		}

		switch id {
		case blockDatablock:
			// Read block
			buf, err := r.up.mr.ReadBytes(r.tmp[:0])
			if err != nil {
				return 0, r.up.setErr(err)
			}

			// Write to buffer and checksum
			if r.check == checksumTypeXxhash {
				r.h.Write(buf)
			}
			r.tmp = buf
			r.buf.Write(buf)
		case blockEOS:
			// Verify stream checksum if any.
			hash, err := r.up.mr.ReadBytes(nil)
			if err != nil {
				return 0, r.up.setErr(err)
			}
			switch r.check {
			case checksumTypeXxhash:
				got := r.h.Sum(nil)
				if !bytes.Equal(hash, got) {
					return 0, r.up.setErr(fmt.Errorf("checksum mismatch, want %s, got %s", hex.EncodeToString(hash), hex.EncodeToString(got)))
				}
			case checksumTypeNone:
			default:
				return 0, r.up.setErr(fmt.Errorf("unknown checksum id %d", r.check))
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
			if id >= 0 {
				return 0, fmt.Errorf("unexpected block type: %d", id)
			}
			// Skip block...
			_, err := io.CopyN(io.Discard, r.up.mr, int64(sz))
			if err != nil {
				return 0, r.up.setErr(err)
			}
		}
	}
}
