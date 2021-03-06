/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Sun Dec 24 20:30:06 2017 mstenber
 * Last modified: Sat Dec 30 15:29:11 2017 mstenber
 * Edit time:     0 min
 *
 */

// Semi-legacy tests to ensure cbor is still slow
package storage

import (
	"log"
	"testing"

	"github.com/fingon/go-tfhfs/mlog"
	"github.com/ugorji/go/codec"
)

func BenchmarkCBORDecode(b *testing.B) {
	var bh codec.CborHandle
	var buf []byte
	enc := codec.NewEncoderBytes(&buf, &bh)
	md := BlockMetadata{}
	if err := enc.Encode(md); err != nil {
		log.Panic(err)
	}
	mlog.Printf2("storage/cbor_test", "Encoded length: %d", len(buf))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dec := codec.NewDecoderBytes(buf, &bh)
		var v BlockMetadata
		if err := dec.Decode(&v); err != nil {
			log.Panic(err)
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
			log.Panic(err)
		}
	}
}
