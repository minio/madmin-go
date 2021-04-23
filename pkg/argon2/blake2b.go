//
// MinIO Object Storage (c) 2021 MinIO, Inc.
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

package argon2

import (
	"encoding/binary"
	"hash"

	"golang.org/x/crypto/blake2b"
)

// blake2bHash computes an arbitrary long hash value of in
// and writes the hash to out.
func blake2bHash(out []byte, in []byte) {
	var b2 hash.Hash
	if n := len(out); n < blake2b.Size {
		b2, _ = blake2b.New(n, nil)
	} else {
		b2, _ = blake2b.New512(nil)
	}

	var buffer [blake2b.Size]byte
	binary.LittleEndian.PutUint32(buffer[:4], uint32(len(out)))
	b2.Write(buffer[:4])
	b2.Write(in)

	if len(out) <= blake2b.Size {
		b2.Sum(out[:0])
		return
	}

	outLen := len(out)
	b2.Sum(buffer[:0])
	b2.Reset()
	copy(out, buffer[:32])
	out = out[32:]
	for len(out) > blake2b.Size {
		b2.Write(buffer[:])
		b2.Sum(buffer[:0])
		copy(out, buffer[:32])
		out = out[32:]
		b2.Reset()
	}

	if outLen%blake2b.Size > 0 { // outLen > 64
		r := ((outLen + 31) / 32) - 2 // ⌈τ /32⌉-2
		b2, _ = blake2b.New(outLen-32*r, nil)
	}
	b2.Write(buffer[:])
	b2.Sum(out[:0])
}
