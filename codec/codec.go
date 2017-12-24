/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Sun Dec 24 16:42:12 2017 mstenber
 * Last modified: Sun Dec 24 18:31:01 2017 mstenber
 * Edit time:     58 min
 *
 */

// codec library is responsible for transforming data + additionalData
// to different kind of data. This means in practise either
// encrypting/decrypting, or compressing/uncompressing on case-by-case
// basis.
//
// CodecChain makes it possible to combine multiple Codecs that do the
// particular sub-EncodeBytes/DecodeBytes steps.
package codec

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"log"

	"github.com/minio/sha256-simd"
	"github.com/pierrec/lz4"
	"golang.org/x/crypto/pbkdf2"
)

// Codec
//
// Single transformation of byte slices.
type Codec interface {
	DecodeBytes(data, additionalData []byte) (ret []byte, err error)
	EncodeBytes(data, additionalData []byte) (ret []byte, err error)
}

// EncryptingCodec
//
// AES GCM based encrypting/decrypting (+authenticating) Codec.
//
// TBD: Should # of iterations be parametrizable?
type EncryptingCodec struct {
	gcm cipher.AEAD
	// Main key
	mk []byte
}

func (self EncryptingCodec) Init(password, salt []byte, iter int) *EncryptingCodec {
	self.mk = pbkdf2.Key(password, salt, iter, 32, sha256.New)
	block, err := aes.NewCipher(self.mk)
	if err != nil {
		log.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		log.Fatal(err)
	}
	self.gcm = gcm
	return &self
}

func (self *EncryptingCodec) DecodeBytes(data, additionalData []byte) (ret []byte, err error) {
	var ed EncryptedData
	_, err = ed.UnmarshalMsg(data)
	if err != nil {
		return
	}
	ret, err = self.gcm.Open(nil, ed.Nonce, ed.EncryptedData, additionalData)
	return
}

func (self *EncryptingCodec) EncodeBytes(data, additionalData []byte) (ret []byte, err error) {
	nonce := make([]byte, self.gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return
	}
	ciphertext := self.gcm.Seal(ret, nonce, data, additionalData)
	ed := EncryptedData{Nonce: nonce, EncryptedData: ciphertext}
	ret, err = ed.MarshalMsg(nil)
	return
}

// CompressingCodec
//
// On-the-fly compressing Codec. If the result does not improve, the
// result is marked to be plaintext and passed as-is (at cost of 1
// byte).
type CompressingCodec struct {
	// maximumSize represents the largest decode we have been hit
	// with.  By default we always allocate target buffers of that
	// size when decoding and exponentially grow the # if we are too small.
	maximumSize int
}

const smallestCompressionSize = 1024      // Reasonable initial #
const largestCompressionSize = 1024000000 // Gigabyte at once is madness

func (self *CompressingCodec) DecodeBytes(data, additionalData []byte) (ret []byte, err error) {
	var cd CompressedData
	_, err = cd.UnmarshalMsg(data)
	if err != nil {
		return
	}
	switch cd.CompressionType {
	case CompressionType_PLAIN:
		ret = cd.RawData
	case CompressionType_LZ4:
		maximumSize := self.maximumSize
		if maximumSize < smallestCompressionSize {

			maximumSize = 1024
		}
		ret = make([]byte, maximumSize)
		var n int
		n, err = lz4.UncompressBlock(cd.RawData, ret, 0)
		if err == lz4.ErrShortBuffer {
			self.maximumSize = maximumSize * 2
			if self.maximumSize > largestCompressionSize {
				log.Panic(err)
			}
			return self.DecodeBytes(data, additionalData)
		}
		ret = ret[:n]
	}
	return
}

func (self *CompressingCodec) EncodeBytes(data, additionalData []byte) (ret []byte, err error) {
	rd := make([]byte, len(data))
	var n int
	n, err = lz4.CompressBlock(data, rd, 0)
	if err != nil {
		return
	}
	ct := CompressionType_LZ4
	if n == 0 {
		ct = CompressionType_PLAIN
		rd = data
	} else {
		rd = rd[:n]
	}
	cd := CompressedData{CompressionType: ct, RawData: rd}
	ret, err = cd.MarshalMsg(nil)
	return
}

type CodecChain struct {
	codecs, reverseCodecs []Codec
}

// Init method initializes the codec chain.
//
// codecs are given in decryption order, so e.g.
// encrypting one should be given before compressing one.
func (self CodecChain) Init(codecs []Codec) *CodecChain {
	self.codecs = codecs
	// Reverse the codec slice for decryption purposes
	rc := make([]Codec, len(codecs))
	for i, c := range codecs {
		rc[len(codecs)-i-1] = c
	}
	self.reverseCodecs = rc
	return &self
}

func (self *CodecChain) DecodeBytes(data, additionalData []byte) (ret []byte, err error) {
	ret = data
	for _, c := range self.codecs {
		ret, err = c.DecodeBytes(data, additionalData)
		if err != nil {
			return
		}
		data = ret
	}
	return
}

func (self *CodecChain) EncodeBytes(data, additionalData []byte) (ret []byte, err error) {
	ret = data
	for _, c := range self.reverseCodecs {
		ret, err = c.EncodeBytes(data, additionalData)
		if err != nil {
			return
		}
		data = ret
	}
	return
}
