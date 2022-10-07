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
	"fmt"
	"io"

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
