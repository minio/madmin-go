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
	"crypto/sha512"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"hash"
	"io"

	"github.com/cespare/xxhash/v2"
	"github.com/minio/minlz"
	"github.com/secure-io/sio-go"
	"github.com/tinylib/msgp/msgp"
	"github.com/zeebo/xxh3"
)

// Writer provides a stream writer.
// Streams can optionally be encrypted.
// All streams have checksum verification.
type Writer struct {
	up       io.Writer
	err      error
	key      *[32]byte
	bw       blockWriter
	nonce    uint64
	checksum checksumType
	comp     []minlz.WriterOption
}

const (
	writerMajorVersion = 2
	writerMinorVersion = 1
)

// NewWriter will return a writer that allows to add encrypted and non-encrypted data streams.
func NewWriter(w io.Writer) *Writer {
	_, err := w.Write([]byte{writerMajorVersion, writerMinorVersion})
	writer := &Writer{err: err, up: w, checksum: checksumTypeXxhash}
	writer.bw.init(w)
	return writer
}

// Close will flush and close the output stream.
func (w *Writer) Close() error {
	if w.err != nil {
		return w.err
	}
	w.addBlock(blockEOF)
	return w.sendBlock()
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

	mw := w.addBlock(blockEncryptedKey)

	// Write public key...
	if err := mw.WriteBytes(x509.MarshalPKCS1PublicKey(publicKey)); err != nil {
		return w.setErr(err)
	}

	// Write encrypted cipher key
	w.setErr(mw.WriteBytes(cipherKey))
	return w.sendBlock()
}

// WithCompression enables compression on streams.
func (w *Writer) WithCompression(opts ...minlz.WriterOption) {
	// Always disable the index. Doesn't make sense here.
	opts = append(opts, minlz.WriterCreateIndex(false))
	w.comp = opts
	// We have checksum, no need for double checksum.
	w.checksum = checksumTypeNone
}

// WithXXH3 enables xxh3 checksums on streams.
func (w *Writer) WithXXH3() {
	w.checksum = checksumTypeXxhash3
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

	mw := w.addBlock(blockPlainKey)
	w.setErr(mw.WriteBytes(key[:]))

	return w.sendBlock()
}

// AddError will indicate the writer encountered an error
// and the reader should abort the stream.
// The message will be returned as an error.
func (w *Writer) AddError(msg string) error {
	if w.err != nil {
		return w.err
	}
	mw := w.addBlock(blockError)
	w.setErr(mw.WriteString(msg))
	return w.sendBlock()
}

// AddUnencryptedStream adds a named stream.
// Extra data can be added, which is added without encryption or checksums.
func (w *Writer) AddUnencryptedStream(name string, extra []byte) (io.WriteCloser, error) {
	if w.err != nil {
		return nil, w.err
	}

	var mw *msgp.Writer
	if w.comp == nil {
		mw = w.addBlock(blockPlainStream)
	} else {
		mw = w.addBlock(blockPlainCompressedStream)
	}

	// Write metadata...
	w.setErr(mw.WriteString(name))
	w.setErr(mw.WriteBytes(extra))
	w.setErr(mw.WriteUint8(uint8(w.checksum)))
	if err := w.sendBlock(); err != nil {
		return nil, err
	}
	sw := w.newStreamWriter()
	if w.comp == nil {
		return sw, nil
	}
	// minlz.Writer.Close only flushes; it does not close sw, so wrap it to
	// ensure the EOS/checksum block is written after the compressed data.
	return &closeWrapper{
		up:    minlz.NewWriter(sw, w.comp...),
		after: sw.Close,
	}, nil
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
	var mw *msgp.Writer
	if w.comp == nil {
		mw = w.addBlock(blockEncStream)
	} else {
		mw = w.addBlock(blockEncCompressedStream)
	}

	// Write metadata...
	w.setErr(mw.WriteString(name))
	w.setErr(mw.WriteBytes(extra))
	w.setErr(mw.WriteUint8(uint8(w.checksum)))

	stream, err := sio.AES_256_GCM.Stream(w.key[:])
	if err != nil {
		return nil, w.setErr(err)
	}

	// Get nonce for stream.
	nonce := make([]byte, stream.NonceSize())
	binary.LittleEndian.PutUint64(nonce, w.nonce)
	w.nonce++

	// Write nonce as bin array.
	w.setErr(mw.WriteBytes(nonce))

	if err := w.sendBlock(); err != nil {
		return nil, err
	}

	// Send output as blocks.
	sw := w.newStreamWriter()
	after := sw.Close

	encw := stream.EncryptWriter(sw, nonce, nil)
	wc := io.WriteCloser(encw)
	if w.comp != nil {
		mz := minlz.NewWriter(encw, w.comp...)
		after = func() error {
			err := encw.Close()
			if err != nil {
				sw.Close()
				return err
			}
			return sw.Close()
		}
		wc = mz
	}
	return &closeWrapper{
		up:    wc,
		after: after,
	}, nil
}

// addBlock initializes a new block.
// Block content should be written to the returned writer.
// When done call sendBlock.
func (w *Writer) addBlock(id blockID) *msgp.Writer {
	return w.bw.newBlock(id)
}

// sendBlock sends the queued block.
func (w *Writer) sendBlock() error {
	if w.err != nil {
		return w.err
	}
	return w.setErr(w.bw.send())
}

// newStreamWriter creates a new stream writer
func (w *Writer) newStreamWriter() *streamWriter {
	sw := &streamWriter{w: w}
	return sw.init()
}

// setErr will set a stateful error on w.
// If an error has already been set that is returned instead.
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

// streamWriter will send each individual write as a block on stream.
// Close must be called when writes have completed to send hashes.
type streamWriter struct {
	w          *Writer
	h          hash.Hash
	eosWritten bool
}

func (w *streamWriter) init() *streamWriter {
	switch w.w.checksum {
	case checksumTypeXxhash:
		w.h = xxhash.New()
	case checksumTypeXxhash3:
		w.h = xxh3.New()
	default:
		w.h = &zeroHasher{}
	}
	w.h.Reset()
	return w
}

// Write satisfies the io.Writer interface.
// Each write is sent as a separate block.
func (w *streamWriter) Write(b []byte) (int, error) {
	mw := w.w.addBlock(blockDatablock)

	// Update hash.
	w.h.Write(b)

	// Write as messagepack bin array.
	if err := mw.WriteBytes(b); err != nil {
		return 0, w.w.setErr(err)
	}
	// Write data as binary array.
	return len(b), w.w.sendBlock()
}

// Close satisfies the io.Closer interface.
func (w *streamWriter) Close() error {
	// Write EOS only once.
	if !w.eosWritten {
		mw := w.w.addBlock(blockEOS)
		sum := w.h.Sum(nil)
		w.w.setErr(mw.WriteBytes(sum))
		w.eosWritten = true
		return w.w.sendBlock()
	}
	return nil
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
		w.before = nil
	}
	if w.up != nil {
		if err := w.up.Close(); err != nil {
			return err
		}
		w.up = nil
	}
	if w.after != nil {
		if err := w.after(); err != nil {
			return err
		}
		w.after = nil
	}
	return nil
}

type blockWriter struct {
	id  blockID
	w   io.Writer
	wr  *msgp.Writer
	buf bytes.Buffer
	hdr [8 + 5]byte
}

// init the blockwriter
// blocks will be written to w.
func (b *blockWriter) init(w io.Writer) {
	b.w = w
	b.buf.Grow(1 << 10)
	b.buf.Reset()
	b.wr = msgp.NewWriter(&b.buf)
}

// newBlock starts a new block with the specified id.
// Content should be written to the returned writer.
func (b *blockWriter) newBlock(id blockID) *msgp.Writer {
	b.id = id
	b.buf.Reset()
	b.wr.Reset(&b.buf)
	return b.wr
}

func (b *blockWriter) send() error {
	if b.id == 0 {
		return errors.New("blockWriter: no block started")
	}

	// Flush block data into b.buf
	if err := b.wr.Flush(); err != nil {
		return err
	}
	// Add block id
	hdr := msgp.AppendInt8(b.hdr[:0], int8(b.id))
	blockLen := uint32(b.buf.Len())
	if blockLen > (1<<32 - 1) {
		return errors.New("max block size exceeded")
	}
	// Add block length.
	hdr = msgp.AppendUint32(hdr, blockLen)
	if _, err := b.w.Write(hdr); err != nil {
		return err
	}
	// Write block.
	_, err := b.w.Write(b.buf.Bytes())

	// Reset for new block.
	b.buf.Reset()
	b.id = 0
	return err
}
