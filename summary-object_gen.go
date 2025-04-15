package madmin

// Code generated by github.com/tinylib/msgp DO NOT EDIT.

import (
	"github.com/tinylib/msgp/msgp"
)

// DecodeMsg implements msgp.Decodable
func (z *ObjectMetaSummary) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, err = dc.ReadMapHeader()
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "Filename":
			z.Filename, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Filename")
				return
			}
		case "Host":
			z.Host, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Host")
				return
			}
		case "Drive":
			z.Drive, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Drive")
				return
			}
		case "Size":
			z.Size, err = dc.ReadInt64()
			if err != nil {
				err = msgp.WrapError(err, "Size")
				return
			}
		case "Errors":
			var zb0002 uint32
			zb0002, err = dc.ReadArrayHeader()
			if err != nil {
				err = msgp.WrapError(err, "Errors")
				return
			}
			if cap(z.Errors) >= int(zb0002) {
				z.Errors = (z.Errors)[:zb0002]
			} else {
				z.Errors = make([]string, zb0002)
			}
			for za0001 := range z.Errors {
				z.Errors[za0001], err = dc.ReadString()
				if err != nil {
					err = msgp.WrapError(err, "Errors", za0001)
					return
				}
			}
		case "IsDeleteMarker":
			z.IsDeleteMarker, err = dc.ReadBool()
			if err != nil {
				err = msgp.WrapError(err, "IsDeleteMarker")
				return
			}
		case "ModTime":
			z.ModTime, err = dc.ReadInt64()
			if err != nil {
				err = msgp.WrapError(err, "ModTime")
				return
			}
		case "Signature":
			err = dc.ReadExactBytes((z.Signature)[:])
			if err != nil {
				err = msgp.WrapError(err, "Signature")
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *ObjectMetaSummary) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 8
	// write "Filename"
	err = en.Append(0x88, 0xa8, 0x46, 0x69, 0x6c, 0x65, 0x6e, 0x61, 0x6d, 0x65)
	if err != nil {
		return
	}
	err = en.WriteString(z.Filename)
	if err != nil {
		err = msgp.WrapError(err, "Filename")
		return
	}
	// write "Host"
	err = en.Append(0xa4, 0x48, 0x6f, 0x73, 0x74)
	if err != nil {
		return
	}
	err = en.WriteString(z.Host)
	if err != nil {
		err = msgp.WrapError(err, "Host")
		return
	}
	// write "Drive"
	err = en.Append(0xa5, 0x44, 0x72, 0x69, 0x76, 0x65)
	if err != nil {
		return
	}
	err = en.WriteString(z.Drive)
	if err != nil {
		err = msgp.WrapError(err, "Drive")
		return
	}
	// write "Size"
	err = en.Append(0xa4, 0x53, 0x69, 0x7a, 0x65)
	if err != nil {
		return
	}
	err = en.WriteInt64(z.Size)
	if err != nil {
		err = msgp.WrapError(err, "Size")
		return
	}
	// write "Errors"
	err = en.Append(0xa6, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x73)
	if err != nil {
		return
	}
	err = en.WriteArrayHeader(uint32(len(z.Errors)))
	if err != nil {
		err = msgp.WrapError(err, "Errors")
		return
	}
	for za0001 := range z.Errors {
		err = en.WriteString(z.Errors[za0001])
		if err != nil {
			err = msgp.WrapError(err, "Errors", za0001)
			return
		}
	}
	// write "IsDeleteMarker"
	err = en.Append(0xae, 0x49, 0x73, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x4d, 0x61, 0x72, 0x6b, 0x65, 0x72)
	if err != nil {
		return
	}
	err = en.WriteBool(z.IsDeleteMarker)
	if err != nil {
		err = msgp.WrapError(err, "IsDeleteMarker")
		return
	}
	// write "ModTime"
	err = en.Append(0xa7, 0x4d, 0x6f, 0x64, 0x54, 0x69, 0x6d, 0x65)
	if err != nil {
		return
	}
	err = en.WriteInt64(z.ModTime)
	if err != nil {
		err = msgp.WrapError(err, "ModTime")
		return
	}
	// write "Signature"
	err = en.Append(0xa9, 0x53, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65)
	if err != nil {
		return
	}
	err = en.WriteBytes((z.Signature)[:])
	if err != nil {
		err = msgp.WrapError(err, "Signature")
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *ObjectMetaSummary) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 8
	// string "Filename"
	o = append(o, 0x88, 0xa8, 0x46, 0x69, 0x6c, 0x65, 0x6e, 0x61, 0x6d, 0x65)
	o = msgp.AppendString(o, z.Filename)
	// string "Host"
	o = append(o, 0xa4, 0x48, 0x6f, 0x73, 0x74)
	o = msgp.AppendString(o, z.Host)
	// string "Drive"
	o = append(o, 0xa5, 0x44, 0x72, 0x69, 0x76, 0x65)
	o = msgp.AppendString(o, z.Drive)
	// string "Size"
	o = append(o, 0xa4, 0x53, 0x69, 0x7a, 0x65)
	o = msgp.AppendInt64(o, z.Size)
	// string "Errors"
	o = append(o, 0xa6, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x73)
	o = msgp.AppendArrayHeader(o, uint32(len(z.Errors)))
	for za0001 := range z.Errors {
		o = msgp.AppendString(o, z.Errors[za0001])
	}
	// string "IsDeleteMarker"
	o = append(o, 0xae, 0x49, 0x73, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x4d, 0x61, 0x72, 0x6b, 0x65, 0x72)
	o = msgp.AppendBool(o, z.IsDeleteMarker)
	// string "ModTime"
	o = append(o, 0xa7, 0x4d, 0x6f, 0x64, 0x54, 0x69, 0x6d, 0x65)
	o = msgp.AppendInt64(o, z.ModTime)
	// string "Signature"
	o = append(o, 0xa9, 0x53, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65)
	o = msgp.AppendBytes(o, (z.Signature)[:])
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *ObjectMetaSummary) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "Filename":
			z.Filename, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Filename")
				return
			}
		case "Host":
			z.Host, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Host")
				return
			}
		case "Drive":
			z.Drive, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Drive")
				return
			}
		case "Size":
			z.Size, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Size")
				return
			}
		case "Errors":
			var zb0002 uint32
			zb0002, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Errors")
				return
			}
			if cap(z.Errors) >= int(zb0002) {
				z.Errors = (z.Errors)[:zb0002]
			} else {
				z.Errors = make([]string, zb0002)
			}
			for za0001 := range z.Errors {
				z.Errors[za0001], bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					err = msgp.WrapError(err, "Errors", za0001)
					return
				}
			}
		case "IsDeleteMarker":
			z.IsDeleteMarker, bts, err = msgp.ReadBoolBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "IsDeleteMarker")
				return
			}
		case "ModTime":
			z.ModTime, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "ModTime")
				return
			}
		case "Signature":
			bts, err = msgp.ReadExactBytes(bts, (z.Signature)[:])
			if err != nil {
				err = msgp.WrapError(err, "Signature")
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *ObjectMetaSummary) Msgsize() (s int) {
	s = 1 + 9 + msgp.StringPrefixSize + len(z.Filename) + 5 + msgp.StringPrefixSize + len(z.Host) + 6 + msgp.StringPrefixSize + len(z.Drive) + 5 + msgp.Int64Size + 7 + msgp.ArrayHeaderSize
	for za0001 := range z.Errors {
		s += msgp.StringPrefixSize + len(z.Errors[za0001])
	}
	s += 15 + msgp.BoolSize + 8 + msgp.Int64Size + 10 + msgp.ArrayHeaderSize + (4 * (msgp.ByteSize))
	return
}

// DecodeMsg implements msgp.Decodable
func (z *ObjectPartSummary) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, err = dc.ReadMapHeader()
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "Part":
			z.Part, err = dc.ReadInt()
			if err != nil {
				err = msgp.WrapError(err, "Part")
				return
			}
		case "Pool":
			z.Pool, err = dc.ReadInt()
			if err != nil {
				err = msgp.WrapError(err, "Pool")
				return
			}
		case "Host":
			z.Host, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Host")
				return
			}
		case "Set":
			z.Set, err = dc.ReadInt()
			if err != nil {
				err = msgp.WrapError(err, "Set")
				return
			}
		case "Drive":
			z.Drive, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Drive")
				return
			}
		case "Filename":
			z.Filename, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Filename")
				return
			}
		case "Size":
			z.Size, err = dc.ReadInt64()
			if err != nil {
				err = msgp.WrapError(err, "Size")
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *ObjectPartSummary) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 7
	// write "Part"
	err = en.Append(0x87, 0xa4, 0x50, 0x61, 0x72, 0x74)
	if err != nil {
		return
	}
	err = en.WriteInt(z.Part)
	if err != nil {
		err = msgp.WrapError(err, "Part")
		return
	}
	// write "Pool"
	err = en.Append(0xa4, 0x50, 0x6f, 0x6f, 0x6c)
	if err != nil {
		return
	}
	err = en.WriteInt(z.Pool)
	if err != nil {
		err = msgp.WrapError(err, "Pool")
		return
	}
	// write "Host"
	err = en.Append(0xa4, 0x48, 0x6f, 0x73, 0x74)
	if err != nil {
		return
	}
	err = en.WriteString(z.Host)
	if err != nil {
		err = msgp.WrapError(err, "Host")
		return
	}
	// write "Set"
	err = en.Append(0xa3, 0x53, 0x65, 0x74)
	if err != nil {
		return
	}
	err = en.WriteInt(z.Set)
	if err != nil {
		err = msgp.WrapError(err, "Set")
		return
	}
	// write "Drive"
	err = en.Append(0xa5, 0x44, 0x72, 0x69, 0x76, 0x65)
	if err != nil {
		return
	}
	err = en.WriteString(z.Drive)
	if err != nil {
		err = msgp.WrapError(err, "Drive")
		return
	}
	// write "Filename"
	err = en.Append(0xa8, 0x46, 0x69, 0x6c, 0x65, 0x6e, 0x61, 0x6d, 0x65)
	if err != nil {
		return
	}
	err = en.WriteString(z.Filename)
	if err != nil {
		err = msgp.WrapError(err, "Filename")
		return
	}
	// write "Size"
	err = en.Append(0xa4, 0x53, 0x69, 0x7a, 0x65)
	if err != nil {
		return
	}
	err = en.WriteInt64(z.Size)
	if err != nil {
		err = msgp.WrapError(err, "Size")
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *ObjectPartSummary) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 7
	// string "Part"
	o = append(o, 0x87, 0xa4, 0x50, 0x61, 0x72, 0x74)
	o = msgp.AppendInt(o, z.Part)
	// string "Pool"
	o = append(o, 0xa4, 0x50, 0x6f, 0x6f, 0x6c)
	o = msgp.AppendInt(o, z.Pool)
	// string "Host"
	o = append(o, 0xa4, 0x48, 0x6f, 0x73, 0x74)
	o = msgp.AppendString(o, z.Host)
	// string "Set"
	o = append(o, 0xa3, 0x53, 0x65, 0x74)
	o = msgp.AppendInt(o, z.Set)
	// string "Drive"
	o = append(o, 0xa5, 0x44, 0x72, 0x69, 0x76, 0x65)
	o = msgp.AppendString(o, z.Drive)
	// string "Filename"
	o = append(o, 0xa8, 0x46, 0x69, 0x6c, 0x65, 0x6e, 0x61, 0x6d, 0x65)
	o = msgp.AppendString(o, z.Filename)
	// string "Size"
	o = append(o, 0xa4, 0x53, 0x69, 0x7a, 0x65)
	o = msgp.AppendInt64(o, z.Size)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *ObjectPartSummary) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "Part":
			z.Part, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Part")
				return
			}
		case "Pool":
			z.Pool, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Pool")
				return
			}
		case "Host":
			z.Host, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Host")
				return
			}
		case "Set":
			z.Set, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Set")
				return
			}
		case "Drive":
			z.Drive, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Drive")
				return
			}
		case "Filename":
			z.Filename, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Filename")
				return
			}
		case "Size":
			z.Size, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Size")
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *ObjectPartSummary) Msgsize() (s int) {
	s = 1 + 5 + msgp.IntSize + 5 + msgp.IntSize + 5 + msgp.StringPrefixSize + len(z.Host) + 4 + msgp.IntSize + 6 + msgp.StringPrefixSize + len(z.Drive) + 9 + msgp.StringPrefixSize + len(z.Filename) + 5 + msgp.Int64Size
	return
}

// DecodeMsg implements msgp.Decodable
func (z *ObjectSummary) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, err = dc.ReadMapHeader()
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "Name":
			z.Name, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Name")
				return
			}
		case "Errors":
			var zb0002 uint32
			zb0002, err = dc.ReadArrayHeader()
			if err != nil {
				err = msgp.WrapError(err, "Errors")
				return
			}
			if cap(z.Errors) >= int(zb0002) {
				z.Errors = (z.Errors)[:zb0002]
			} else {
				z.Errors = make([]string, zb0002)
			}
			for za0001 := range z.Errors {
				z.Errors[za0001], err = dc.ReadString()
				if err != nil {
					err = msgp.WrapError(err, "Errors", za0001)
					return
				}
			}
		case "DataDir":
			z.DataDir, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "DataDir")
				return
			}
		case "IsInline":
			z.IsInline, err = dc.ReadBool()
			if err != nil {
				err = msgp.WrapError(err, "IsInline")
				return
			}
		case "PartNumbers":
			var zb0003 uint32
			zb0003, err = dc.ReadArrayHeader()
			if err != nil {
				err = msgp.WrapError(err, "PartNumbers")
				return
			}
			if cap(z.PartNumbers) >= int(zb0003) {
				z.PartNumbers = (z.PartNumbers)[:zb0003]
			} else {
				z.PartNumbers = make([]int, zb0003)
			}
			for za0002 := range z.PartNumbers {
				z.PartNumbers[za0002], err = dc.ReadInt()
				if err != nil {
					err = msgp.WrapError(err, "PartNumbers", za0002)
					return
				}
			}
		case "ErasureDist":
			var zb0004 uint32
			zb0004, err = dc.ReadArrayHeader()
			if err != nil {
				err = msgp.WrapError(err, "ErasureDist")
				return
			}
			if cap(z.ErasureDist) >= int(zb0004) {
				z.ErasureDist = (z.ErasureDist)[:zb0004]
			} else {
				z.ErasureDist = make([]uint8, zb0004)
			}
			for za0003 := range z.ErasureDist {
				z.ErasureDist[za0003], err = dc.ReadUint8()
				if err != nil {
					err = msgp.WrapError(err, "ErasureDist", za0003)
					return
				}
			}
		case "Metas":
			var zb0005 uint32
			zb0005, err = dc.ReadArrayHeader()
			if err != nil {
				err = msgp.WrapError(err, "Metas")
				return
			}
			if cap(z.Metas) >= int(zb0005) {
				z.Metas = (z.Metas)[:zb0005]
			} else {
				z.Metas = make([]*ObjectMetaSummary, zb0005)
			}
			for za0004 := range z.Metas {
				if dc.IsNil() {
					err = dc.ReadNil()
					if err != nil {
						err = msgp.WrapError(err, "Metas", za0004)
						return
					}
					z.Metas[za0004] = nil
				} else {
					if z.Metas[za0004] == nil {
						z.Metas[za0004] = new(ObjectMetaSummary)
					}
					err = z.Metas[za0004].DecodeMsg(dc)
					if err != nil {
						err = msgp.WrapError(err, "Metas", za0004)
						return
					}
				}
			}
		case "Parts":
			var zb0006 uint32
			zb0006, err = dc.ReadArrayHeader()
			if err != nil {
				err = msgp.WrapError(err, "Parts")
				return
			}
			if cap(z.Parts) >= int(zb0006) {
				z.Parts = (z.Parts)[:zb0006]
			} else {
				z.Parts = make([]*ObjectPartSummary, zb0006)
			}
			for za0005 := range z.Parts {
				if dc.IsNil() {
					err = dc.ReadNil()
					if err != nil {
						err = msgp.WrapError(err, "Parts", za0005)
						return
					}
					z.Parts[za0005] = nil
				} else {
					if z.Parts[za0005] == nil {
						z.Parts[za0005] = new(ObjectPartSummary)
					}
					err = z.Parts[za0005].DecodeMsg(dc)
					if err != nil {
						err = msgp.WrapError(err, "Parts", za0005)
						return
					}
				}
			}
		case "Unknown":
			var zb0007 uint32
			zb0007, err = dc.ReadArrayHeader()
			if err != nil {
				err = msgp.WrapError(err, "Unknown")
				return
			}
			if cap(z.Unknown) >= int(zb0007) {
				z.Unknown = (z.Unknown)[:zb0007]
			} else {
				z.Unknown = make([]*ObjectUnknownSummary, zb0007)
			}
			for za0006 := range z.Unknown {
				if dc.IsNil() {
					err = dc.ReadNil()
					if err != nil {
						err = msgp.WrapError(err, "Unknown", za0006)
						return
					}
					z.Unknown[za0006] = nil
				} else {
					if z.Unknown[za0006] == nil {
						z.Unknown[za0006] = new(ObjectUnknownSummary)
					}
					err = z.Unknown[za0006].DecodeMsg(dc)
					if err != nil {
						err = msgp.WrapError(err, "Unknown", za0006)
						return
					}
				}
			}
		default:
			err = dc.Skip()
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *ObjectSummary) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 9
	// write "Name"
	err = en.Append(0x89, 0xa4, 0x4e, 0x61, 0x6d, 0x65)
	if err != nil {
		return
	}
	err = en.WriteString(z.Name)
	if err != nil {
		err = msgp.WrapError(err, "Name")
		return
	}
	// write "Errors"
	err = en.Append(0xa6, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x73)
	if err != nil {
		return
	}
	err = en.WriteArrayHeader(uint32(len(z.Errors)))
	if err != nil {
		err = msgp.WrapError(err, "Errors")
		return
	}
	for za0001 := range z.Errors {
		err = en.WriteString(z.Errors[za0001])
		if err != nil {
			err = msgp.WrapError(err, "Errors", za0001)
			return
		}
	}
	// write "DataDir"
	err = en.Append(0xa7, 0x44, 0x61, 0x74, 0x61, 0x44, 0x69, 0x72)
	if err != nil {
		return
	}
	err = en.WriteString(z.DataDir)
	if err != nil {
		err = msgp.WrapError(err, "DataDir")
		return
	}
	// write "IsInline"
	err = en.Append(0xa8, 0x49, 0x73, 0x49, 0x6e, 0x6c, 0x69, 0x6e, 0x65)
	if err != nil {
		return
	}
	err = en.WriteBool(z.IsInline)
	if err != nil {
		err = msgp.WrapError(err, "IsInline")
		return
	}
	// write "PartNumbers"
	err = en.Append(0xab, 0x50, 0x61, 0x72, 0x74, 0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x73)
	if err != nil {
		return
	}
	err = en.WriteArrayHeader(uint32(len(z.PartNumbers)))
	if err != nil {
		err = msgp.WrapError(err, "PartNumbers")
		return
	}
	for za0002 := range z.PartNumbers {
		err = en.WriteInt(z.PartNumbers[za0002])
		if err != nil {
			err = msgp.WrapError(err, "PartNumbers", za0002)
			return
		}
	}
	// write "ErasureDist"
	err = en.Append(0xab, 0x45, 0x72, 0x61, 0x73, 0x75, 0x72, 0x65, 0x44, 0x69, 0x73, 0x74)
	if err != nil {
		return
	}
	err = en.WriteArrayHeader(uint32(len(z.ErasureDist)))
	if err != nil {
		err = msgp.WrapError(err, "ErasureDist")
		return
	}
	for za0003 := range z.ErasureDist {
		err = en.WriteUint8(z.ErasureDist[za0003])
		if err != nil {
			err = msgp.WrapError(err, "ErasureDist", za0003)
			return
		}
	}
	// write "Metas"
	err = en.Append(0xa5, 0x4d, 0x65, 0x74, 0x61, 0x73)
	if err != nil {
		return
	}
	err = en.WriteArrayHeader(uint32(len(z.Metas)))
	if err != nil {
		err = msgp.WrapError(err, "Metas")
		return
	}
	for za0004 := range z.Metas {
		if z.Metas[za0004] == nil {
			err = en.WriteNil()
			if err != nil {
				return
			}
		} else {
			err = z.Metas[za0004].EncodeMsg(en)
			if err != nil {
				err = msgp.WrapError(err, "Metas", za0004)
				return
			}
		}
	}
	// write "Parts"
	err = en.Append(0xa5, 0x50, 0x61, 0x72, 0x74, 0x73)
	if err != nil {
		return
	}
	err = en.WriteArrayHeader(uint32(len(z.Parts)))
	if err != nil {
		err = msgp.WrapError(err, "Parts")
		return
	}
	for za0005 := range z.Parts {
		if z.Parts[za0005] == nil {
			err = en.WriteNil()
			if err != nil {
				return
			}
		} else {
			err = z.Parts[za0005].EncodeMsg(en)
			if err != nil {
				err = msgp.WrapError(err, "Parts", za0005)
				return
			}
		}
	}
	// write "Unknown"
	err = en.Append(0xa7, 0x55, 0x6e, 0x6b, 0x6e, 0x6f, 0x77, 0x6e)
	if err != nil {
		return
	}
	err = en.WriteArrayHeader(uint32(len(z.Unknown)))
	if err != nil {
		err = msgp.WrapError(err, "Unknown")
		return
	}
	for za0006 := range z.Unknown {
		if z.Unknown[za0006] == nil {
			err = en.WriteNil()
			if err != nil {
				return
			}
		} else {
			err = z.Unknown[za0006].EncodeMsg(en)
			if err != nil {
				err = msgp.WrapError(err, "Unknown", za0006)
				return
			}
		}
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *ObjectSummary) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 9
	// string "Name"
	o = append(o, 0x89, 0xa4, 0x4e, 0x61, 0x6d, 0x65)
	o = msgp.AppendString(o, z.Name)
	// string "Errors"
	o = append(o, 0xa6, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x73)
	o = msgp.AppendArrayHeader(o, uint32(len(z.Errors)))
	for za0001 := range z.Errors {
		o = msgp.AppendString(o, z.Errors[za0001])
	}
	// string "DataDir"
	o = append(o, 0xa7, 0x44, 0x61, 0x74, 0x61, 0x44, 0x69, 0x72)
	o = msgp.AppendString(o, z.DataDir)
	// string "IsInline"
	o = append(o, 0xa8, 0x49, 0x73, 0x49, 0x6e, 0x6c, 0x69, 0x6e, 0x65)
	o = msgp.AppendBool(o, z.IsInline)
	// string "PartNumbers"
	o = append(o, 0xab, 0x50, 0x61, 0x72, 0x74, 0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x73)
	o = msgp.AppendArrayHeader(o, uint32(len(z.PartNumbers)))
	for za0002 := range z.PartNumbers {
		o = msgp.AppendInt(o, z.PartNumbers[za0002])
	}
	// string "ErasureDist"
	o = append(o, 0xab, 0x45, 0x72, 0x61, 0x73, 0x75, 0x72, 0x65, 0x44, 0x69, 0x73, 0x74)
	o = msgp.AppendArrayHeader(o, uint32(len(z.ErasureDist)))
	for za0003 := range z.ErasureDist {
		o = msgp.AppendUint8(o, z.ErasureDist[za0003])
	}
	// string "Metas"
	o = append(o, 0xa5, 0x4d, 0x65, 0x74, 0x61, 0x73)
	o = msgp.AppendArrayHeader(o, uint32(len(z.Metas)))
	for za0004 := range z.Metas {
		if z.Metas[za0004] == nil {
			o = msgp.AppendNil(o)
		} else {
			o, err = z.Metas[za0004].MarshalMsg(o)
			if err != nil {
				err = msgp.WrapError(err, "Metas", za0004)
				return
			}
		}
	}
	// string "Parts"
	o = append(o, 0xa5, 0x50, 0x61, 0x72, 0x74, 0x73)
	o = msgp.AppendArrayHeader(o, uint32(len(z.Parts)))
	for za0005 := range z.Parts {
		if z.Parts[za0005] == nil {
			o = msgp.AppendNil(o)
		} else {
			o, err = z.Parts[za0005].MarshalMsg(o)
			if err != nil {
				err = msgp.WrapError(err, "Parts", za0005)
				return
			}
		}
	}
	// string "Unknown"
	o = append(o, 0xa7, 0x55, 0x6e, 0x6b, 0x6e, 0x6f, 0x77, 0x6e)
	o = msgp.AppendArrayHeader(o, uint32(len(z.Unknown)))
	for za0006 := range z.Unknown {
		if z.Unknown[za0006] == nil {
			o = msgp.AppendNil(o)
		} else {
			o, err = z.Unknown[za0006].MarshalMsg(o)
			if err != nil {
				err = msgp.WrapError(err, "Unknown", za0006)
				return
			}
		}
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *ObjectSummary) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "Name":
			z.Name, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Name")
				return
			}
		case "Errors":
			var zb0002 uint32
			zb0002, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Errors")
				return
			}
			if cap(z.Errors) >= int(zb0002) {
				z.Errors = (z.Errors)[:zb0002]
			} else {
				z.Errors = make([]string, zb0002)
			}
			for za0001 := range z.Errors {
				z.Errors[za0001], bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					err = msgp.WrapError(err, "Errors", za0001)
					return
				}
			}
		case "DataDir":
			z.DataDir, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "DataDir")
				return
			}
		case "IsInline":
			z.IsInline, bts, err = msgp.ReadBoolBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "IsInline")
				return
			}
		case "PartNumbers":
			var zb0003 uint32
			zb0003, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "PartNumbers")
				return
			}
			if cap(z.PartNumbers) >= int(zb0003) {
				z.PartNumbers = (z.PartNumbers)[:zb0003]
			} else {
				z.PartNumbers = make([]int, zb0003)
			}
			for za0002 := range z.PartNumbers {
				z.PartNumbers[za0002], bts, err = msgp.ReadIntBytes(bts)
				if err != nil {
					err = msgp.WrapError(err, "PartNumbers", za0002)
					return
				}
			}
		case "ErasureDist":
			var zb0004 uint32
			zb0004, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "ErasureDist")
				return
			}
			if cap(z.ErasureDist) >= int(zb0004) {
				z.ErasureDist = (z.ErasureDist)[:zb0004]
			} else {
				z.ErasureDist = make([]uint8, zb0004)
			}
			for za0003 := range z.ErasureDist {
				z.ErasureDist[za0003], bts, err = msgp.ReadUint8Bytes(bts)
				if err != nil {
					err = msgp.WrapError(err, "ErasureDist", za0003)
					return
				}
			}
		case "Metas":
			var zb0005 uint32
			zb0005, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Metas")
				return
			}
			if cap(z.Metas) >= int(zb0005) {
				z.Metas = (z.Metas)[:zb0005]
			} else {
				z.Metas = make([]*ObjectMetaSummary, zb0005)
			}
			for za0004 := range z.Metas {
				if msgp.IsNil(bts) {
					bts, err = msgp.ReadNilBytes(bts)
					if err != nil {
						return
					}
					z.Metas[za0004] = nil
				} else {
					if z.Metas[za0004] == nil {
						z.Metas[za0004] = new(ObjectMetaSummary)
					}
					bts, err = z.Metas[za0004].UnmarshalMsg(bts)
					if err != nil {
						err = msgp.WrapError(err, "Metas", za0004)
						return
					}
				}
			}
		case "Parts":
			var zb0006 uint32
			zb0006, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Parts")
				return
			}
			if cap(z.Parts) >= int(zb0006) {
				z.Parts = (z.Parts)[:zb0006]
			} else {
				z.Parts = make([]*ObjectPartSummary, zb0006)
			}
			for za0005 := range z.Parts {
				if msgp.IsNil(bts) {
					bts, err = msgp.ReadNilBytes(bts)
					if err != nil {
						return
					}
					z.Parts[za0005] = nil
				} else {
					if z.Parts[za0005] == nil {
						z.Parts[za0005] = new(ObjectPartSummary)
					}
					bts, err = z.Parts[za0005].UnmarshalMsg(bts)
					if err != nil {
						err = msgp.WrapError(err, "Parts", za0005)
						return
					}
				}
			}
		case "Unknown":
			var zb0007 uint32
			zb0007, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Unknown")
				return
			}
			if cap(z.Unknown) >= int(zb0007) {
				z.Unknown = (z.Unknown)[:zb0007]
			} else {
				z.Unknown = make([]*ObjectUnknownSummary, zb0007)
			}
			for za0006 := range z.Unknown {
				if msgp.IsNil(bts) {
					bts, err = msgp.ReadNilBytes(bts)
					if err != nil {
						return
					}
					z.Unknown[za0006] = nil
				} else {
					if z.Unknown[za0006] == nil {
						z.Unknown[za0006] = new(ObjectUnknownSummary)
					}
					bts, err = z.Unknown[za0006].UnmarshalMsg(bts)
					if err != nil {
						err = msgp.WrapError(err, "Unknown", za0006)
						return
					}
				}
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *ObjectSummary) Msgsize() (s int) {
	s = 1 + 5 + msgp.StringPrefixSize + len(z.Name) + 7 + msgp.ArrayHeaderSize
	for za0001 := range z.Errors {
		s += msgp.StringPrefixSize + len(z.Errors[za0001])
	}
	s += 8 + msgp.StringPrefixSize + len(z.DataDir) + 9 + msgp.BoolSize + 12 + msgp.ArrayHeaderSize + (len(z.PartNumbers) * (msgp.IntSize)) + 12 + msgp.ArrayHeaderSize + (len(z.ErasureDist) * (msgp.Uint8Size)) + 6 + msgp.ArrayHeaderSize
	for za0004 := range z.Metas {
		if z.Metas[za0004] == nil {
			s += msgp.NilSize
		} else {
			s += z.Metas[za0004].Msgsize()
		}
	}
	s += 6 + msgp.ArrayHeaderSize
	for za0005 := range z.Parts {
		if z.Parts[za0005] == nil {
			s += msgp.NilSize
		} else {
			s += z.Parts[za0005].Msgsize()
		}
	}
	s += 8 + msgp.ArrayHeaderSize
	for za0006 := range z.Unknown {
		if z.Unknown[za0006] == nil {
			s += msgp.NilSize
		} else {
			s += z.Unknown[za0006].Msgsize()
		}
	}
	return
}

// DecodeMsg implements msgp.Decodable
func (z *ObjectSummaryOptions) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, err = dc.ReadMapHeader()
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "Bucket":
			z.Bucket, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Bucket")
				return
			}
		case "Object":
			z.Object, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Object")
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z ObjectSummaryOptions) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 2
	// write "Bucket"
	err = en.Append(0x82, 0xa6, 0x42, 0x75, 0x63, 0x6b, 0x65, 0x74)
	if err != nil {
		return
	}
	err = en.WriteString(z.Bucket)
	if err != nil {
		err = msgp.WrapError(err, "Bucket")
		return
	}
	// write "Object"
	err = en.Append(0xa6, 0x4f, 0x62, 0x6a, 0x65, 0x63, 0x74)
	if err != nil {
		return
	}
	err = en.WriteString(z.Object)
	if err != nil {
		err = msgp.WrapError(err, "Object")
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z ObjectSummaryOptions) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 2
	// string "Bucket"
	o = append(o, 0x82, 0xa6, 0x42, 0x75, 0x63, 0x6b, 0x65, 0x74)
	o = msgp.AppendString(o, z.Bucket)
	// string "Object"
	o = append(o, 0xa6, 0x4f, 0x62, 0x6a, 0x65, 0x63, 0x74)
	o = msgp.AppendString(o, z.Object)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *ObjectSummaryOptions) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "Bucket":
			z.Bucket, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Bucket")
				return
			}
		case "Object":
			z.Object, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Object")
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z ObjectSummaryOptions) Msgsize() (s int) {
	s = 1 + 7 + msgp.StringPrefixSize + len(z.Bucket) + 7 + msgp.StringPrefixSize + len(z.Object)
	return
}

// DecodeMsg implements msgp.Decodable
func (z *ObjectUnknownSummary) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, err = dc.ReadMapHeader()
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "Pool":
			z.Pool, err = dc.ReadInt()
			if err != nil {
				err = msgp.WrapError(err, "Pool")
				return
			}
		case "Host":
			z.Host, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Host")
				return
			}
		case "Set":
			z.Set, err = dc.ReadInt()
			if err != nil {
				err = msgp.WrapError(err, "Set")
				return
			}
		case "Drive":
			z.Drive, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Drive")
				return
			}
		case "Filename":
			z.Filename, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Filename")
				return
			}
		case "Size":
			z.Size, err = dc.ReadInt64()
			if err != nil {
				err = msgp.WrapError(err, "Size")
				return
			}
		case "Dir":
			z.Dir, err = dc.ReadBool()
			if err != nil {
				err = msgp.WrapError(err, "Dir")
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *ObjectUnknownSummary) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 7
	// write "Pool"
	err = en.Append(0x87, 0xa4, 0x50, 0x6f, 0x6f, 0x6c)
	if err != nil {
		return
	}
	err = en.WriteInt(z.Pool)
	if err != nil {
		err = msgp.WrapError(err, "Pool")
		return
	}
	// write "Host"
	err = en.Append(0xa4, 0x48, 0x6f, 0x73, 0x74)
	if err != nil {
		return
	}
	err = en.WriteString(z.Host)
	if err != nil {
		err = msgp.WrapError(err, "Host")
		return
	}
	// write "Set"
	err = en.Append(0xa3, 0x53, 0x65, 0x74)
	if err != nil {
		return
	}
	err = en.WriteInt(z.Set)
	if err != nil {
		err = msgp.WrapError(err, "Set")
		return
	}
	// write "Drive"
	err = en.Append(0xa5, 0x44, 0x72, 0x69, 0x76, 0x65)
	if err != nil {
		return
	}
	err = en.WriteString(z.Drive)
	if err != nil {
		err = msgp.WrapError(err, "Drive")
		return
	}
	// write "Filename"
	err = en.Append(0xa8, 0x46, 0x69, 0x6c, 0x65, 0x6e, 0x61, 0x6d, 0x65)
	if err != nil {
		return
	}
	err = en.WriteString(z.Filename)
	if err != nil {
		err = msgp.WrapError(err, "Filename")
		return
	}
	// write "Size"
	err = en.Append(0xa4, 0x53, 0x69, 0x7a, 0x65)
	if err != nil {
		return
	}
	err = en.WriteInt64(z.Size)
	if err != nil {
		err = msgp.WrapError(err, "Size")
		return
	}
	// write "Dir"
	err = en.Append(0xa3, 0x44, 0x69, 0x72)
	if err != nil {
		return
	}
	err = en.WriteBool(z.Dir)
	if err != nil {
		err = msgp.WrapError(err, "Dir")
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *ObjectUnknownSummary) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 7
	// string "Pool"
	o = append(o, 0x87, 0xa4, 0x50, 0x6f, 0x6f, 0x6c)
	o = msgp.AppendInt(o, z.Pool)
	// string "Host"
	o = append(o, 0xa4, 0x48, 0x6f, 0x73, 0x74)
	o = msgp.AppendString(o, z.Host)
	// string "Set"
	o = append(o, 0xa3, 0x53, 0x65, 0x74)
	o = msgp.AppendInt(o, z.Set)
	// string "Drive"
	o = append(o, 0xa5, 0x44, 0x72, 0x69, 0x76, 0x65)
	o = msgp.AppendString(o, z.Drive)
	// string "Filename"
	o = append(o, 0xa8, 0x46, 0x69, 0x6c, 0x65, 0x6e, 0x61, 0x6d, 0x65)
	o = msgp.AppendString(o, z.Filename)
	// string "Size"
	o = append(o, 0xa4, 0x53, 0x69, 0x7a, 0x65)
	o = msgp.AppendInt64(o, z.Size)
	// string "Dir"
	o = append(o, 0xa3, 0x44, 0x69, 0x72)
	o = msgp.AppendBool(o, z.Dir)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *ObjectUnknownSummary) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "Pool":
			z.Pool, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Pool")
				return
			}
		case "Host":
			z.Host, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Host")
				return
			}
		case "Set":
			z.Set, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Set")
				return
			}
		case "Drive":
			z.Drive, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Drive")
				return
			}
		case "Filename":
			z.Filename, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Filename")
				return
			}
		case "Size":
			z.Size, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Size")
				return
			}
		case "Dir":
			z.Dir, bts, err = msgp.ReadBoolBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Dir")
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *ObjectUnknownSummary) Msgsize() (s int) {
	s = 1 + 5 + msgp.IntSize + 5 + msgp.StringPrefixSize + len(z.Host) + 4 + msgp.IntSize + 6 + msgp.StringPrefixSize + len(z.Drive) + 9 + msgp.StringPrefixSize + len(z.Filename) + 5 + msgp.Int64Size + 4 + msgp.BoolSize
	return
}
