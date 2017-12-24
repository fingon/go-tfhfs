/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Sun Dec 24 17:15:30 2017 mstenber
 * Last modified: Sun Dec 24 21:46:18 2017 mstenber
 * Edit time:     50 min
 *
 */

package codec

import (
	"crypto/rand"
	"fmt"
	"log"
	"testing"

	"github.com/stvp/assert"
)

const compressible = "123456789123456789123456789123456789123456789123456789123456789123456789123456789123456789123456789"

func ProdCodecOnce(text string, c Codec, t *testing.T) {
	p := []byte(text)
	enc, err := c.EncodeBytes(p, nil)
	assert.Nil(t, err)
	dec, err := c.DecodeBytes(enc, nil)
	assert.Nil(t, err)
	assert.Equal(t, p, dec)

}

func ProdCodec(c Codec, t *testing.T) {
	ProdCodecOnce("foo", c, t)
	ProdCodecOnce(compressible, c, t)
}

func TestEncryptingCodec(t *testing.T) {
	p := []byte("data")
	ad := []byte("ad")

	c := EncryptingCodec{}.Init([]byte("foo"), []byte("salt"), 64)

	// 'any codec' handling
	ProdCodec(c, t)

	enc, err := c.EncodeBytes(p, nil)
	assert.Nil(t, err)

	// Ensure we can't fuck around with additional data
	_, err2 := c.DecodeBytes(enc, ad)
	assert.True(t, err2 != nil)

	// Ensure same payload does not encrypt the same way
	enc2, err := c.EncodeBytes(p, nil)
	assert.NotEqual(t, enc, enc2)

	// But it still can be decrypted
	dec, err := c.DecodeBytes(enc2, nil)
	assert.Nil(t, err)
	assert.Equal(t, p, dec)

	// Ensure we're good with additional data too
	enc3, err := c.EncodeBytes(p, ad)
	dec, err = c.DecodeBytes(enc3, ad)
	assert.Nil(t, err)
	assert.Equal(t, p, dec)
}

func TestCompressingCodec(t *testing.T) {
	c := &CompressingCodec{}
	ProdCodec(c, t)

	p := []byte(compressible)
	enc, err := c.EncodeBytes(p, nil)
	assert.Nil(t, err)
	assert.True(t, len(enc) < len(compressible))
	assert.Equal(t, len(enc), 29) // much less than the original ~100b
}

func TestNopCodecChain(t *testing.T) {
	c := &CodecChain{}
	ProdCodec(c, t)
}
func TestCodecChain(t *testing.T) {
	c1 := EncryptingCodec{}.Init([]byte("foo"), []byte("salt"), 64)
	c2 := &CompressingCodec{}
	c := CodecChain{}.Init(c1, c2)
	ProdCodec(c, t)

	p := []byte(compressible)
	enc, err := c.EncodeBytes(p, nil)
	assert.Nil(t, err)
	assert.True(t, len(enc) < len(compressible))
	assert.Equal(t, len(enc), 62) // bit less than the original ~100
}

func BenchmarkCodec(b *testing.B) {
	runEncode := func(b *testing.B, c Codec, p []byte) {
		b.SetBytes(int64(len(p)))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			enc, err := c.EncodeBytes(p, nil)
			if err != nil || enc == nil {
				log.Panic(err)
			}

		}
	}
	runDecode := func(b *testing.B, c Codec, p []byte) {
		b.SetBytes(int64(len(p)))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			enc, err := c.EncodeBytes(p, nil)
			if err != nil || enc == nil {
				log.Panic(err)
			}

		}
	}
	add := func(c Codec, prefix string) {
		// Compressed
		l1 := fmt.Sprintf("Encode-%s-%s", prefix, "Random")
		p1 := make([]byte, 1024)
		_, err := rand.Read(p1)
		if err != nil {
			log.Panic(err)
		}
		b.Run(l1, func(b *testing.B) {
			runEncode(b, c, p1)
		})
		l1d := fmt.Sprintf("Encode-%s-%s", prefix, "Random")
		p1e, _ := c.EncodeBytes(p1, nil)
		b.Run(l1d, func(b *testing.B) {
			runDecode(b, c, p1e)
		})

		// Zero hero variant
		l2 := fmt.Sprintf("Encode-%s-%s", prefix, "Zeros")
		p2 := make([]byte, 1024)
		b.Run(l2, func(b *testing.B) {
			runEncode(b, c, p2)
		})
		l2d := fmt.Sprintf("Decode-%s-%s", prefix, "Zeros")
		p2e, _ := c.EncodeBytes(p2, nil)
		b.Run(l2d, func(b *testing.B) {
			runDecode(b, c, p2e)
		})

	}
	c1 := EncryptingCodec{}.Init([]byte("foo"), []byte("salt"), 64)
	c2 := &CompressingCodec{}
	cc := CodecChain{}.Init(c1, c2)
	add(c1, "AES")
	add(c2, "LZ4")
	add(cc, "AES+LZ4")
}
