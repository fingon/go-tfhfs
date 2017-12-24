/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Sun Dec 24 20:30:06 2017 mstenber
 * Last modified: Sun Dec 24 20:30:24 2017 mstenber
 * Edit time:     0 min
 *
 */

// Semi-legacy tests to ensure cbor is still slow
package storage

import (
	"log"
	"testing"

	"github.com/ugorji/go/codec"
)

func BenchmarkCBORDecode(b *testing.B) {
	var bh codec.CborHandle
	var buf []byte
	enc := codec.NewEncoderBytes(&buf, &bh)
	md := BlockMetadata{}
	if err := enc.Encode(md); err != nil {
		log.Fatal(err)
	}
	// log.Printf("Encoded length: %d", len(buf))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dec := codec.NewDecoderBytes(buf, &bh)
		var v BlockMetadata
		if err := dec.Decode(&v); err != nil {
			log.Fatal(err)
		}
	}
}

func BenchmarkCBOREncode(b *testing.B) {
	var bh codec.CborHandle
	md := BlockMetadata{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf []byte
		enc := codec.NewEncoderBytes(&buf, &bh)
		if err := enc.Encode(md); err != nil {
			log.Fatal(err)
		}
	}
}
