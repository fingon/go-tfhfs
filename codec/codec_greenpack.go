/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Sun Dec 24 16:42:58 2017 mstenber
 * Last modified: Thu Feb  7 09:47:10 2019 mstenber
 * Edit time:     5 min
 *
 */

package codec

/////////////////////////////////////////////////////////////////////////////

// Codec layer

// This is responsible for hiding (and compressing) bytes in plain
// sight, so to speak.

type EncryptedData struct {
	Nonce         []byte `zid:"0"`
	EncryptedData []byte `zid:"1"`
	// nonce used for AES GCM
	// EncryptedData is AES GCM encrypted CompressedData
}

type CompressionType byte

const (
	CompressionType_UNSET CompressionType = iota

	// The data has not been compressed.
	CompressionType_PLAIN

	// The data is compressed with Snappy.
	CompressionType_SNAPPY

	// Golang built-in zlib
	CompressionType_ZLIB
)

type CompressedData struct {
	// CompressionType describes how the data has been compressed.
	CompressionType CompressionType `zid:"0"`

	// RawData is the raw data of the client (whatever it is)
	RawData []byte `zid:"1"`
}
