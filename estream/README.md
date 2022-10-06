# Encrypted Stream

This package provides a flexible way to merge multiple streams with controlled encryption.

## Features

* Allows encrypted and unencrypted streams.
* Any number of keys can be used on a streams.
* Each key can be encrypted by a (different) public key.
* Each stream is identified by a string "name".
* A stream has optional (unencrypted) metadata slice.
* Keys can be re-encrypted with another public key without needing data re-encryption given the private key.
* Streams are checksummed.
* Streams cannot be truncated by early EOF.
* Format is extensible with skippable blocks.
* Allows signaling errors while writing streams.
* Nonce per stream (of course).
* Messagepack for platform independent type safety.

# Format

## Header

Format starts with 2 version bytes.

| Field         | Type   |
|---------------|--------|
| Major Version | 1 byte |
| Minor Version | 1 byte |

Unknown major versions should be rejected by the decoder, 
however minor versions are assumed to be compatible, 
but may contain data that will be ignored by older versions.

## Blocks

Each block is preceded by a messagepack encoded int8 indicating the block type.

Positive types must be parsed by the decoder. Negative types are *skippable* blocks.

Skippable blocks must have their length encoded as a messagepack unsigned integer following the block ID.
This indicates the number of bytes to skip after the length to reach the next block ID.

The skippable block size must be representable by a 32 bit unsigned integer.

All block content is messagepack encoded.

### id 1: Plain Key

This block contains an unencrypted key that is used for all following streams.

Multiple keys can be sent, but only the latest key should be used to decrypt a stream. 

| Field         | Type      | Contents      |
|---------------|-----------|---------------|
| Stream Key    | bin array | 32 byte key   |

### id 2: RSA Encrypted Key

This block contains an RSA encrypted key that is used for all following streams.

Multiple keys can be sent, but only the latest key should be used to decrypt a stream.

| Field      | Type      | Contents                                    |
|------------|-----------|---------------------------------------------|
| Public Key | bin array | RSA public key to PKCS #1 in ASN.1 DER form |
| Cipher Key | bin array | 32 byte key encrypted with public key above |

The cipher key is encrypted with RSA-OAEP using SHA-512.


### id 3: SIO Encrypted Stream

Start of stream encrypted using [sio-go](github.com/secure-io/sio-go).

Stream will be encrypted using `AES_256_GCM` using the last key provided on stream.

| Field    | Type      | Contents                      |
|----------|-----------|-------------------------------|
| Name     | string    | Identifier of the stream      |
| Extra    | bin array | Optional extra data           |
| Checksum | uint8     | Checksum type used for stream |
| Nonce    | bin array | 32 byte nonce used for stream |

The stream consists of all data blocks following until "End Of Stream" block is sent.

### id 4: Plain Stream

Start of unencrypted stream.

| Field    | Type      | Contents                      |
|----------|-----------|-------------------------------|
| Name     | string    | Identifier of the stream      |
| Extra    | bin array | Optional extra data           |
| Checksum | uint8     | Checksum type used for stream |

The stream consists of all data blocks following until "End Of Stream" block is sent.

### id 5: Data Block

Data contains a data block.

| Field | Type      | Contents                 |
|-------|-----------|--------------------------|
| Data  | bin array | Data to append to stream |

If block is part of an encrypted stream it should be sent to the stream decrypter as is.

### id 6: End Of Stream

Indicates successful end of individual stream. 

| Field         | Type      | Contents  |
|---------------|-----------|-----------|
| Checksum      | bin array | Checksum  |

No more data blocks should be expected before new stream information is sent.

### id 7: EOF

Indicates successful end of all streams.

### id 8: Error

An error block can be sent to indicate an error occurred while generating the stream.

It is expected that the parser returns the message and stops processing.

| Field   | Type   | Contents                            |
|---------|--------|-------------------------------------|
| Message | string | Error message that will be returned |

## Checksum types

| ID  | Type                  | Bytes     |
|-----|-----------------------|-----------|
| 0   | No checksum           | (ignored) | 
| 1   | 64 bit xxhash (XXH64) | 8         | 