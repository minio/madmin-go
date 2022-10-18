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
	"crypto/rand"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"

	"github.com/cespare/xxhash/v2"
	"github.com/secure-io/sio-go"
	"github.com/tinylib/msgp/msgp"
)

// ReplaceFn provides key replacement.
//
// When a key is found on stream, the function is called with the public key.
// The function must then return a private key to decrypt matching the key sent.
// The public key must then be specified that should be used to re-encrypt the stream.
//
// If no private key is sent and the public key matches the one sent to the function
// the key will be kept as is. Other returned values will cause an error.
//
// For encrypting unencrypted keys on stream a nil key will be sent.
// If a public key is returned the key will be encrypted with the public key.
// No private key should be returned for this.
type ReplaceFn func(key *rsa.PublicKey) (*rsa.PrivateKey, *rsa.PublicKey)

// ReplaceKeysOptions allows passing additional options to ReplaceKeys.
type ReplaceKeysOptions struct {
	// If EncryptAll set all unencrypted keys will be encrypted.
	EncryptAll bool

	// PassErrors will pass through error an error packet,
	// and not return an error.
	PassErrors bool
}

// ReplaceKeys will replace the keys in a stream.
//
// A replace function must be provided. See ReplaceFn for functionality.
// If encryptAll is set.
func ReplaceKeys(w io.Writer, r io.Reader, replace ReplaceFn, o ReplaceKeysOptions) error {
	var ver [2]byte
	if _, err := io.ReadFull(r, ver[:]); err != nil {
		return err
	}
	switch ver[0] {
	case 2:
	default:
		return fmt.Errorf("unknown stream version: 0x%x", ver[0])
	}
	if _, err := w.Write(ver[:]); err != nil {
		return err
	}
	// Input
	mr := msgp.NewReader(r)
	mw := msgp.NewWriter(w)

	// Temporary block storage.
	block := make([]byte, 1024)

	// Write a block.
	writeBlock := func(id blockID, sz uint32, content []byte) error {
		if err := mw.WriteInt8(int8(id)); err != nil {
			return err
		}
		if err := mw.WriteUint32(sz); err != nil {
			return err
		}
		_, err := mw.Write(content)
		return err
	}

	for {
		// Read block ID.
		n, err := mr.ReadInt8()
		if err != nil {
			return err
		}
		id := blockID(n)

		// Read size
		sz, err := mr.ReadUint32()
		if err != nil {
			return err
		}
		if cap(block) < int(sz) {
			block = make([]byte, sz)
		}
		block = block[:sz]
		_, err = io.ReadFull(mr, block)
		if err != nil {
			return err
		}

		switch id {
		case blockEncryptedKey:
			ogBlock := block
			// Read public key
			publicKey, block, err := msgp.ReadBytesZC(block)
			if err != nil {
				return err
			}

			pk, err := x509.ParsePKCS1PublicKey(publicKey)
			if err != nil {
				return err
			}

			private, public := replace(pk)
			if private == nil && public == pk {
				if err := writeBlock(id, sz, ogBlock); err != nil {
					return err
				}
			}
			if private == nil {
				return errors.New("no private key provided, unable to re-encrypt")
			}

			// Read cipher key
			cipherKey, _, err := msgp.ReadBytesZC(block)
			if err != nil {
				return err
			}

			// Decrypt stream key
			key, err := rsa.DecryptOAEP(sha512.New(), crand.Reader, private, cipherKey, nil)
			if err != nil {
				return err
			}

			if len(key) != 32 {
				return fmt.Errorf("unexpected key length: %d", len(key))
			}

			cipherKey, err = rsa.EncryptOAEP(sha512.New(), crand.Reader, public, key[:], nil)
			if err != nil {
				return err
			}

			// Write Public key
			tmp := msgp.AppendBytes(nil, x509.MarshalPKCS1PublicKey(public))
			// Write encrypted cipher key
			tmp = msgp.AppendBytes(tmp, cipherKey)
			if err := writeBlock(blockEncryptedKey, uint32(len(tmp)), tmp); err != nil {
				return err
			}
		case blockPlainKey:
			if !o.EncryptAll {
				if err := writeBlock(id, sz, block); err != nil {
					return err
				}
				continue
			}
			_, public := replace(nil)
			if public == nil {
				if err := writeBlock(id, sz, block); err != nil {
					return err
				}
				continue
			}
			key, _, err := msgp.ReadBytesZC(block)
			if err != nil {
				return err
			}
			if len(key) != 32 {
				return fmt.Errorf("unexpected key length: %d", len(key))
			}
			cipherKey, err := rsa.EncryptOAEP(sha512.New(), crand.Reader, public, key[:], nil)
			if err != nil {
				return err
			}

			// Write Public key
			tmp := msgp.AppendBytes(nil, x509.MarshalPKCS1PublicKey(public))
			// Write encrypted cipher key
			tmp = msgp.AppendBytes(tmp, cipherKey)
			if err := writeBlock(blockEncryptedKey, uint32(len(tmp)), tmp); err != nil {
				return err
			}
		case blockEOF:
			if err := writeBlock(id, sz, block); err != nil {
				return err
			}
			return mw.Flush()
		case blockError:
			if o.PassErrors {
				if err := writeBlock(id, sz, block); err != nil {
					return err
				}
				return mw.Flush()
			}
			// Return error
			msg, _, err := msgp.ReadStringBytes(block)
			if err != nil {
				return err
			}
			return errors.New(msg)
		default:
			if err := writeBlock(id, sz, block); err != nil {
				return err
			}
		}
	}
}

// DebugStream will print stream block information to w.
func (r *Reader) DebugStream(w io.Writer) error {
	if r.err != nil {
		return r.err
	}
	if r.inStream {
		return errors.New("previous stream not read until EOF")
	}
	fmt.Fprintf(w, "stream major: %v, minor: %v\n", r.majorV, r.minorV)

	// Temp storage for blocks.
	block := make([]byte, 1024)
	hashers := []hash.Hash{nil, xxhash.New()}
	for {
		// Read block ID.
		n, err := r.mr.ReadInt8()
		if err != nil {
			return r.setErr(fmt.Errorf("reading block id: %w", err))
		}
		id := blockID(n)

		// Read block size
		sz, err := r.mr.ReadUint32()
		if err != nil {
			return r.setErr(fmt.Errorf("reading block size: %w", err))
		}
		fmt.Fprintf(w, "block type: %v, size: %d bytes, in stream: %v\n", id, sz, r.inStream)

		// Read block data
		if cap(block) < int(sz) {
			block = make([]byte, sz)
		}
		block = block[:sz]
		_, err = io.ReadFull(r.mr, block)
		if err != nil {
			return r.setErr(fmt.Errorf("reading block data: %w", err))
		}

		// Parse block
		switch id {
		case blockPlainKey:
			// Read plaintext key.
			key, _, err := msgp.ReadBytesBytes(block, make([]byte, 0, 32))
			if err != nil {
				return r.setErr(fmt.Errorf("reading key: %w", err))
			}
			if len(key) != 32 {
				return r.setErr(fmt.Errorf("unexpected key length: %d", len(key)))
			}

			// Set key for following streams.
			r.key = (*[32]byte)(key)
			fmt.Fprintf(w, "plain key read\n")

		case blockEncryptedKey:
			// Read public key
			publicKey, block, err := msgp.ReadBytesZC(block)
			if err != nil {
				return r.setErr(fmt.Errorf("reading public key: %w", err))
			}

			// Request private key if we have a custom function.
			if r.privateFn != nil {
				fmt.Fprintf(w, "requesting private key from privateFn\n")
				pk, err := x509.ParsePKCS1PublicKey(publicKey)
				if err != nil {
					return r.setErr(fmt.Errorf("parse public key: %w", err))
				}
				r.private = r.privateFn(pk)
				if r.private == nil {
					fmt.Fprintf(w, "privateFn did not provide private key\n")
					if r.skipEncrypted || r.returnNonDec {
						fmt.Fprintf(w, "continuing. skipEncrypted: %v, returnNonDec: %v\n", r.skipEncrypted, r.returnNonDec)
						r.key = nil
						continue
					}
					return r.setErr(errors.New("nil private key returned"))
				}
			}

			// Read cipher key
			cipherKey, _, err := msgp.ReadBytesZC(block)
			if err != nil {
				return r.setErr(fmt.Errorf("reading cipherkey: %w", err))
			}
			if r.private == nil {
				if r.skipEncrypted || r.returnNonDec {
					fmt.Fprintf(w, "no private key, continuing due to skipEncrypted: %v, returnNonDec: %v\n", r.skipEncrypted, r.returnNonDec)
					r.key = nil
					continue
				}
				return r.setErr(errors.New("private key has not been set"))
			}

			// Decrypt stream key
			key, err := rsa.DecryptOAEP(sha512.New(), rand.Reader, r.private, cipherKey, nil)
			if err != nil {
				if r.returnNonDec {
					fmt.Fprintf(w, "no private key, continuing due to returnNonDec: %v\n", r.returnNonDec)
					r.key = nil
					continue
				}
				return fmt.Errorf("decrypting key: %w", err)
			}

			if len(key) != 32 {
				return r.setErr(fmt.Errorf("unexpected key length: %d", len(key)))
			}
			r.key = (*[32]byte)(key)
			fmt.Fprintf(w, "stream key decoded\n")

		case blockPlainStream, blockEncStream:
			// Read metadata
			name, block, err := msgp.ReadStringBytes(block)
			if err != nil {
				return r.setErr(fmt.Errorf("reading name: %w", err))
			}
			extra, block, err := msgp.ReadBytesBytes(block, nil)
			if err != nil {
				return r.setErr(fmt.Errorf("reading extra: %w", err))
			}
			c, block, err := msgp.ReadUint8Bytes(block)
			if err != nil {
				return r.setErr(fmt.Errorf("reading checksum: %w", err))
			}
			checksum := checksumType(c)
			if !checksum.valid() {
				return r.setErr(fmt.Errorf("unknown checksum type %d", checksum))
			}
			fmt.Fprintf(w, "new stream. name: %v, extra size: %v, checksum type: %v\n", name, len(extra), checksum)

			for _, h := range hashers {
				if h != nil {
					h.Reset()
				}
			}

			// Return plaintext stream
			if id == blockPlainStream {
				r.inStream = true
				continue
			}

			// Handle encrypted streams.
			if r.key == nil {
				if r.skipEncrypted {
					fmt.Fprintf(w, "nil key, skipEncrypted: %v\n", r.skipEncrypted)
					r.inStream = true
					continue
				}
				return ErrNoKey
			}
			// Read stream nonce
			nonce, _, err := msgp.ReadBytesZC(block)
			if err != nil {
				return r.setErr(fmt.Errorf("reading nonce: %w", err))
			}

			stream, err := sio.AES_256_GCM.Stream(r.key[:])
			if err != nil {
				return r.setErr(fmt.Errorf("initializing sio: %w", err))
			}

			// Check if nonce is expected length.
			if len(nonce) != stream.NonceSize() {
				return r.setErr(fmt.Errorf("unexpected nonce length: %d", len(nonce)))
			}
			fmt.Fprintf(w, "nonce len: %v\n", len(nonce))
			r.inStream = true
		case blockEOS:
			if !r.inStream {
				return errors.New("end-of-stream without being in stream")
			}
			h, _, err := msgp.ReadBytesZC(block)
			if err != nil {
				return r.setErr(fmt.Errorf("reading block data: %w", err))
			}
			fmt.Fprintf(w, "end-of-stream. stream hash: %s. data hashes: ", hex.EncodeToString(h))
			for i, h := range hashers {
				if h != nil {
					fmt.Fprintf(w, "%s:%s. ", checksumType(i), hex.EncodeToString(h.Sum(nil)))
				}
			}
			fmt.Fprint(w, "\n")
			r.inStream = false
		case blockEOF:
			if r.inStream {
				return errors.New("end-of-file without finishing stream")
			}
			fmt.Fprintf(w, "end-of-file\n")
			return nil
		case blockError:
			msg, _, err := msgp.ReadStringBytes(block)
			if err != nil {
				return r.setErr(fmt.Errorf("reading error string: %w", err))
			}
			fmt.Fprintf(w, "error recorded on stream: %v\n", msg)
			return nil
		case blockDatablock:
			buf, _, err := msgp.ReadBytesZC(block)
			if err != nil {
				return r.setErr(fmt.Errorf("reading block data: %w", err))
			}
			for _, h := range hashers {
				if h != nil {
					h.Write(buf)
				}
			}
			fmt.Fprintf(w, "data block, length: %v\n", len(buf))
		default:
			fmt.Fprintf(w, "skipping block\n")
			if id >= 0 {
				return fmt.Errorf("unknown block type: %d", id)
			}
		}
	}
}
