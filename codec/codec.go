/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Sun Dec 24 16:42:12 2017 mstenber
 * Last modified: Mon Dec 25 00:58:09 2017 mstenber
 * Edit time:     75 min
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
	"bytes"
	"compress/zlib"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"io"
	"io/ioutil"
	"log"

	"github.com/golang/snappy"
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
	CompressionType CompressionType
}

func (self *CompressingCodec) DecodeBytes(data, additionalData []byte) (ret []byte, err error) {
	var cd CompressedData
	_, err = cd.UnmarshalMsg(data)
	if err != nil {
		return
	}
	switch cd.CompressionType {
	case CompressionType_PLAIN:
		ret = cd.RawData
	case CompressionType_SNAPPY:
		ret, err = snappy.Decode(nil, cd.RawData)
	case CompressionType_ZLIB:
		br := bytes.NewReader(cd.RawData)
		var r io.Reader
		r, err = zlib.NewReader(br)
		if err != nil {
			return
		}
		ret, err = ioutil.ReadAll(r)
	}
	return
}

func (self *CompressingCodec) EncodeBytes(data, additionalData []byte) (ret []byte, err error) {
	var rd []byte
	var ct CompressionType
	switch self.CompressionType {
	case CompressionType_ZLIB:
		var b bytes.Buffer
		w := zlib.NewWriter(&b)
		w.Write(data)
		w.Close()
		rd = b.Bytes()
		ct = CompressionType_ZLIB

	case CompressionType_UNSET:
		fallthrough
	case CompressionType_SNAPPY:
		rd = snappy.Encode(nil, data)
		ct = CompressionType_SNAPPY

	case CompressionType_PLAIN:
		ct = CompressionType_PLAIN
		rd = data
	}
	if ct != CompressionType_PLAIN && len(rd) >= len(data) {
		ct = CompressionType_PLAIN
		rd = data
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
func (self CodecChain) Init(codecs ...Codec) *CodecChain {
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
