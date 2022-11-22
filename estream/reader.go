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
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"runtime"

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
	returnNonDec  bool
}

// ErrNoKey is returned when a stream cannot be decrypted.
// The Skip function on the stream can be called to skip to the next.
var ErrNoKey = errors.New("no valid private key found")

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
// If the function returns a nil private key the stream key will not be decrypted
// and if SkipEncrypted has been set any streams with this key will be silently skipped.
// This overrides any key set by SetPrivateKey.
func (r *Reader) PrivateKeyProvider(fn func(key *rsa.PublicKey) *rsa.PrivateKey) {
	r.privateFn = fn
	r.private = nil
}

// SkipEncrypted will skip encrypted streams if no private key has been set.
func (r *Reader) SkipEncrypted(b bool) {
	r.skipEncrypted = b
}

// ReturnNonDecryptable will return non-decryptable stream headers.
// Streams are returned with ErrNoKey error.
// Streams with this error cannot be read, but the Skip function can be invoked.
// SkipEncrypted overrides this.
func (r *Reader) ReturnNonDecryptable(b bool) {
	r.returnNonDec = b
}

// Stream returns the next stream.
type Stream struct {
	io.Reader
	Name          string
	Extra         []byte
	SentEncrypted bool

	parent *Reader
}

// NextStream will return the next stream.
// Before calling this the previous stream must be read until EOF,
// or Skip() should have been called.
// Will return nil, io.EOF when there are no more streams.
func (r *Reader) NextStream() (*Stream, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.inStream {
		return nil, errors.New("previous stream not read until EOF")
	}

	// Temp storage for blocks.
	block := make([]byte, 1024)
	for {
		// Read block ID.
		n, err := r.mr.ReadInt8()
		if err != nil {
			return nil, r.setErr(err)
		}
		id := blockID(n)

		// Read block size
		sz, err := r.mr.ReadUint32()
		if err != nil {
			return nil, r.setErr(err)
		}

		// Read block data
		if cap(block) < int(sz) {
			block = make([]byte, sz)
		}
		block = block[:sz]
		_, err = io.ReadFull(r.mr, block)
		if err != nil {
			return nil, r.setErr(err)
		}

		// Parse block
		switch id {
		case blockPlainKey:
			// Read plaintext key.
			key, _, err := msgp.ReadBytesBytes(block, make([]byte, 0, 32))
			if err != nil {
				return nil, r.setErr(err)
			}
			if len(key) != 32 {
				return nil, r.setErr(fmt.Errorf("unexpected key length: %d", len(key)))
			}

			// Set key for following streams.
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
					if r.skipEncrypted || r.returnNonDec {
						r.key = nil
						continue
					}
					return nil, r.setErr(errors.New("nil private key returned"))
				}
			}

			// Read cipher key
			cipherKey, _, err := msgp.ReadBytesZC(block)
			if err != nil {
				return nil, r.setErr(err)
			}
			if r.private == nil {
				if r.skipEncrypted || r.returnNonDec {
					r.key = nil
					continue
				}
				return nil, r.setErr(errors.New("private key has not been set"))
			}

			// Decrypt stream key
			key, err := rsa.DecryptOAEP(sha512.New(), rand.Reader, r.private, cipherKey, nil)
			if err != nil {
				if r.returnNonDec {
					r.key = nil
					continue
				}
				return nil, err
			}

			if len(key) != 32 {
				return nil, r.setErr(fmt.Errorf("unexpected key length: %d", len(key)))
			}
			r.key = (*[32]byte)(key)

		case blockPlainStream, blockEncStream:
			// Read metadata
			name, block, err := msgp.ReadStringBytes(block)
			if err != nil {
				return nil, r.setErr(err)
			}
			extra, block, err := msgp.ReadBytesBytes(block, nil)
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

			// Return plaintext stream
			if id == blockPlainStream {
				return &Stream{
					Reader: r.newStreamReader(checksum),
					Name:   name,
					Extra:  extra,
					parent: r,
				}, nil
			}

			// Handle encrypted streams.
			if r.key == nil {
				if r.skipEncrypted {
					if err := r.skipDataBlocks(); err != nil {
						return nil, r.setErr(err)
					}
					continue
				}
				return &Stream{
					SentEncrypted: true,
					Reader:        nil,
					Name:          name,
					Extra:         extra,
					parent:        r,
				}, ErrNoKey
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
				SentEncrypted: true,
				Reader:        encr,
				Name:          name,
				Extra:         extra,
				parent:        r,
			}, nil
		case blockEOS:
			return nil, errors.New("end-of-stream without being in stream")
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

// skipDataBlocks reads data blocks until end.
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

// setErr sets a stateful error.
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
	if false {
		_, file, line, ok := runtime.Caller(1)
		if ok {
			err = fmt.Errorf("%s:%d: %w", file, line, err)
		}
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

// newStreamReader creates a stream reader that can be read to get all data blocks.
func (r *Reader) newStreamReader(ct checksumType) *streamReader {
	sr := &streamReader{up: r, check: ct}
	sr.h.Reset()
	r.inStream = true
	return sr
}

// Skip the remainder of the stream.
func (s *Stream) Skip() error {
	if sr, ok := s.Reader.(*streamReader); ok {
		sr.isEOF = true
		sr.buf.Reset()
	}
	return s.parent.skipDataBlocks()
}

// Read will return data blocks as on stream.
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
