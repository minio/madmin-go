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

# Usage

Create a writer that will write the stream. 

You must provide an `io.Writer` to which the output is written. 
Once all streams have been written it should be closed to indicate end of payload. 

```Go
    w := estream.NewWriter(output)
    defer w.Close()
```

It is possible to signal an error to the receiver using `w.AddError(msg string)`.
This will return the error to the receiver.  

## Adding keys

Keys for streams must be added. The keys themselves are 32 bytes of random data, 
but it must be specified how they are stored. 

They can be added as plain text, which isn't secure, 
but allows later encryption using a public key.
To add a key without encryption use `w.AddKeyPlain()` 
which will add the keys to the stream.

To add an encrypted key provide a 2048 bit public RSA key.
Use `w.AddKeyEncrypted(publicKey)` to add a key to the stream.

Once a key has been sent on the stream it will be used for all subsequent encrypted streams.
This means that different keys with different private/public keys can be sent for different streams.

## Sending streams

Streams are added using either `w.AddEncryptedStream` or `w.AddUnencryptedStream`.

A string identifier can be used to identify each stream when reading.
An optional byte block can also be sent.

Note that neither the name nor the byte block is encrypted, 
so they should not contain sensitive data.

The functions above return an `io.WriteCloser`.
Data for this stream should be written to this interface
and `Close()` should be called before another stream can be added.

# Reading Streams

To read back data `r, err := estream.NewReader(input)` can be used for create a Reader.

To set a private key, use `r.SetPrivateKey(key)` to set a single private key.

For multiple keys a key provider can be made to return the appropriate key:

```Go
    var key1, key2 *rsa.PrivateKey
    // (read keys)
    r.PrivateKeyProvider(func(key *rsa.PublicKey) *rsa.PrivateKey {
        if key.Equal(&key1.PublicKey) {
            return key1
        }
        if key.Equal(&key2.PublicKey) {
            return key2
        }
        // Unknown key :(
        return nil
    })
```

It is possible to skip streams that cannot be decrypted using `r.SkipEncrypted(true)`.

A simple for loop can be used to get all streams:

```Go
    for {
        stream, err := r.NextStream()
        if err == io.EOF {
            // All streams read
            break
        }
        // Metadata:
        fmt.Println(stream.Name)
        fmt.Println(stream.Extra)
		
        // Stream content is a standard io.Reader
        io.Copy(os.StdOut, stream)
    }
```

## Replacing keys

It is possible to replace public keys needed for decryption using `estream.ReplaceKeys()`.

For encrypted keys the private key must be provided and optionally unencrypted keys can also be 
encrypted using a public key.

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


| Field  | Type         | Contents                 |
|--------|--------------|--------------------------|
| id     | integer      | Block ID                 |
| length | unsigned int | Length of block in bytes |

Each block is preceded by a messagepack encoded int8 indicating the block type.

Positive types must be parsed by the decoder. Negative types are *skippable* blocks.

Blocks have their length encoded as a messagepack unsigned integer following the block ID.
This indicates the number of bytes to skip after the length to reach the next block ID.

Maximum block size is 2^32-1 (4294967295) bytes.

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

Checksum is of encrypted data.
There is no checksum for decrypted data.

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

# Version History

| Major | Minor | Changes         |
|-------|-------|-----------------|
| 2     | 1     | Initial Version | 

