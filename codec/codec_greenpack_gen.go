// Code generated by GREENPACK (github.com/glycerine/greenpack). DO NOT EDIT.

package codec

import (
	"github.com/glycerine/greenpack/msgp"
)

// DecodeMsg implements msgp.Decodable
// We treat empty fields as if we read a Nil from the wire.
func (z *CompressedData) DecodeMsg(dc *msgp.Reader) (err error) {

	var zgensym_ea6ee3ecf6671bc7_0 uint32
	zgensym_ea6ee3ecf6671bc7_0, err = dc.ReadArrayHeader()
	if err != nil {
		return
	}
	if zgensym_ea6ee3ecf6671bc7_0 != 2 {
		err = msgp.ArrayError{Wanted: 2, Got: zgensym_ea6ee3ecf6671bc7_0}
		return
	}
	{
		var zgensym_ea6ee3ecf6671bc7_1 byte
		zgensym_ea6ee3ecf6671bc7_1, err = dc.ReadByte()
		z.CompressionType = CompressionType(zgensym_ea6ee3ecf6671bc7_1)
	}
	if err != nil {
		return
	}
	z.RawData, err = dc.ReadBytes(z.RawData)
	if err != nil {
		return
	}
	if p, ok := interface{}(z).(msgp.PostLoad); ok {
		p.PostLoadHook()
	}

	return
}

// EncodeMsg implements msgp.Encodable
func (z *CompressedData) EncodeMsg(en *msgp.Writer) (err error) {
	if p, ok := interface{}(z).(msgp.PreSave); ok {
		p.PreSaveHook()
	}

	// array header, size 2
	err = en.Append(0x92)
	if err != nil {
		return err
	}
	err = en.WriteByte(byte(z.CompressionType))
	if err != nil {
		return
	}
	err = en.WriteBytes(z.RawData)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *CompressedData) MarshalMsg(b []byte) (o []byte, err error) {
	if p, ok := interface{}(z).(msgp.PreSave); ok {
		p.PreSaveHook()
	}

	o = msgp.Require(b, z.Msgsize())
	// array header, size 2
	o = append(o, 0x92)
	o = msgp.AppendByte(o, byte(z.CompressionType))
	o = msgp.AppendBytes(o, z.RawData)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *CompressedData) UnmarshalMsg(bts []byte) (o []byte, err error) {
	return z.UnmarshalMsgWithCfg(bts, nil)
}
func (z *CompressedData) UnmarshalMsgWithCfg(bts []byte, cfg *msgp.RuntimeConfig) (o []byte, err error) {
	var nbs msgp.NilBitsStack
	nbs.Init(cfg)
	var sawTopNil bool
	if msgp.IsNil(bts) {
		sawTopNil = true
		bts = nbs.PushAlwaysNil(bts[1:])
	}

	var zgensym_ea6ee3ecf6671bc7_2 uint32
	zgensym_ea6ee3ecf6671bc7_2, bts, err = nbs.ReadArrayHeaderBytes(bts)
	if err != nil {
		return
	}
	if zgensym_ea6ee3ecf6671bc7_2 != 2 {
		err = msgp.ArrayError{Wanted: 2, Got: zgensym_ea6ee3ecf6671bc7_2}
		return
	}
	{
		var zgensym_ea6ee3ecf6671bc7_3 byte
		zgensym_ea6ee3ecf6671bc7_3, bts, err = nbs.ReadByteBytes(bts)

		if err != nil {
			return
		}
		z.CompressionType = CompressionType(zgensym_ea6ee3ecf6671bc7_3)
	}
	if nbs.AlwaysNil || msgp.IsNil(bts) {
		if !nbs.AlwaysNil {
			bts = bts[1:]
		}
		z.RawData = z.RawData[:0]
	} else {
		z.RawData, bts, err = nbs.ReadBytesBytes(bts, z.RawData)

		if err != nil {
			return
		}
	}
	if err != nil {
		return
	}
	if sawTopNil {
		bts = nbs.PopAlwaysNil()
	}
	o = bts
	if p, ok := interface{}(z).(msgp.PostLoad); ok {
		p.PostLoadHook()
	}

	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *CompressedData) Msgsize() (s int) {
	s = 1 + msgp.ByteSize + msgp.BytesPrefixSize + len(z.RawData)
	return
}

// DecodeMsg implements msgp.Decodable
// We treat empty fields as if we read a Nil from the wire.
func (z *CompressionType) DecodeMsg(dc *msgp.Reader) (err error) {

	{
		var zgensym_ea6ee3ecf6671bc7_4 byte
		zgensym_ea6ee3ecf6671bc7_4, err = dc.ReadByte()
		(*z) = CompressionType(zgensym_ea6ee3ecf6671bc7_4)
	}
	if err != nil {
		return
	}
	if p, ok := interface{}(z).(msgp.PostLoad); ok {
		p.PostLoadHook()
	}

	return
}

// EncodeMsg implements msgp.Encodable
func (z CompressionType) EncodeMsg(en *msgp.Writer) (err error) {
	if p, ok := interface{}(z).(msgp.PreSave); ok {
		p.PreSaveHook()
	}

	err = en.WriteByte(byte(z))
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z CompressionType) MarshalMsg(b []byte) (o []byte, err error) {
	if p, ok := interface{}(z).(msgp.PreSave); ok {
		p.PreSaveHook()
	}

	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendByte(o, byte(z))
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *CompressionType) UnmarshalMsg(bts []byte) (o []byte, err error) {
	return z.UnmarshalMsgWithCfg(bts, nil)
}
func (z *CompressionType) UnmarshalMsgWithCfg(bts []byte, cfg *msgp.RuntimeConfig) (o []byte, err error) {
	var nbs msgp.NilBitsStack
	nbs.Init(cfg)
	var sawTopNil bool
	if msgp.IsNil(bts) {
		sawTopNil = true
		bts = nbs.PushAlwaysNil(bts[1:])
	}

	{
		var zgensym_ea6ee3ecf6671bc7_5 byte
		zgensym_ea6ee3ecf6671bc7_5, bts, err = nbs.ReadByteBytes(bts)

		if err != nil {
			return
		}
		(*z) = CompressionType(zgensym_ea6ee3ecf6671bc7_5)
	}
	if sawTopNil {
		bts = nbs.PopAlwaysNil()
	}
	o = bts
	if p, ok := interface{}(z).(msgp.PostLoad); ok {
		p.PostLoadHook()
	}

	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z CompressionType) Msgsize() (s int) {
	s = msgp.ByteSize
	return
}

// DecodeMsg implements msgp.Decodable
// We treat empty fields as if we read a Nil from the wire.
func (z *EncryptedData) DecodeMsg(dc *msgp.Reader) (err error) {

	var zgensym_ea6ee3ecf6671bc7_6 uint32
	zgensym_ea6ee3ecf6671bc7_6, err = dc.ReadArrayHeader()
	if err != nil {
		return
	}
	if zgensym_ea6ee3ecf6671bc7_6 != 2 {
		err = msgp.ArrayError{Wanted: 2, Got: zgensym_ea6ee3ecf6671bc7_6}
		return
	}
	z.Nonce, err = dc.ReadBytes(z.Nonce)
	if err != nil {
		return
	}
	z.EncryptedData, err = dc.ReadBytes(z.EncryptedData)
	if err != nil {
		return
	}
	if p, ok := interface{}(z).(msgp.PostLoad); ok {
		p.PostLoadHook()
	}

	return
}

// EncodeMsg implements msgp.Encodable
func (z *EncryptedData) EncodeMsg(en *msgp.Writer) (err error) {
	if p, ok := interface{}(z).(msgp.PreSave); ok {
		p.PreSaveHook()
	}

	// array header, size 2
	err = en.Append(0x92)
	if err != nil {
		return err
	}
	err = en.WriteBytes(z.Nonce)
	if err != nil {
		return
	}
	err = en.WriteBytes(z.EncryptedData)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *EncryptedData) MarshalMsg(b []byte) (o []byte, err error) {
	if p, ok := interface{}(z).(msgp.PreSave); ok {
		p.PreSaveHook()
	}

	o = msgp.Require(b, z.Msgsize())
	// array header, size 2
	o = append(o, 0x92)
	o = msgp.AppendBytes(o, z.Nonce)
	o = msgp.AppendBytes(o, z.EncryptedData)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *EncryptedData) UnmarshalMsg(bts []byte) (o []byte, err error) {
	return z.UnmarshalMsgWithCfg(bts, nil)
}
func (z *EncryptedData) UnmarshalMsgWithCfg(bts []byte, cfg *msgp.RuntimeConfig) (o []byte, err error) {
	var nbs msgp.NilBitsStack
	nbs.Init(cfg)
	var sawTopNil bool
	if msgp.IsNil(bts) {
		sawTopNil = true
		bts = nbs.PushAlwaysNil(bts[1:])
	}

	var zgensym_ea6ee3ecf6671bc7_7 uint32
	zgensym_ea6ee3ecf6671bc7_7, bts, err = nbs.ReadArrayHeaderBytes(bts)
	if err != nil {
		return
	}
	if zgensym_ea6ee3ecf6671bc7_7 != 2 {
		err = msgp.ArrayError{Wanted: 2, Got: zgensym_ea6ee3ecf6671bc7_7}
		return
	}
	if nbs.AlwaysNil || msgp.IsNil(bts) {
		if !nbs.AlwaysNil {
			bts = bts[1:]
		}
		z.Nonce = z.Nonce[:0]
	} else {
		z.Nonce, bts, err = nbs.ReadBytesBytes(bts, z.Nonce)

		if err != nil {
			return
		}
	}
	if err != nil {
		return
	}
	if nbs.AlwaysNil || msgp.IsNil(bts) {
		if !nbs.AlwaysNil {
			bts = bts[1:]
		}
		z.EncryptedData = z.EncryptedData[:0]
	} else {
		z.EncryptedData, bts, err = nbs.ReadBytesBytes(bts, z.EncryptedData)

		if err != nil {
			return
		}
	}
	if err != nil {
		return
	}
	if sawTopNil {
		bts = nbs.PopAlwaysNil()
	}
	o = bts
	if p, ok := interface{}(z).(msgp.PostLoad); ok {
		p.PostLoadHook()
	}

	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *EncryptedData) Msgsize() (s int) {
	s = 1 + msgp.BytesPrefixSize + len(z.Nonce) + msgp.BytesPrefixSize + len(z.EncryptedData)
	return
}