package madmin

// Code generated by github.com/tinylib/msgp DO NOT EDIT.

import (
	"github.com/tinylib/msgp/msgp"
)

// DecodeMsg implements msgp.Decodable
func (z *LatencyStat) DecodeMsg(dc *msgp.Reader) (err error) {
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
		case "Curr":
			z.Curr, err = dc.ReadDuration()
			if err != nil {
				err = msgp.WrapError(err, "Curr")
				return
			}
		case "Avg":
			z.Avg, err = dc.ReadDuration()
			if err != nil {
				err = msgp.WrapError(err, "Avg")
				return
			}
		case "Max":
			z.Max, err = dc.ReadDuration()
			if err != nil {
				err = msgp.WrapError(err, "Max")
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
func (z LatencyStat) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 3
	// write "Curr"
	err = en.Append(0x83, 0xa4, 0x43, 0x75, 0x72, 0x72)
	if err != nil {
		return
	}
	err = en.WriteDuration(z.Curr)
	if err != nil {
		err = msgp.WrapError(err, "Curr")
		return
	}
	// write "Avg"
	err = en.Append(0xa3, 0x41, 0x76, 0x67)
	if err != nil {
		return
	}
	err = en.WriteDuration(z.Avg)
	if err != nil {
		err = msgp.WrapError(err, "Avg")
		return
	}
	// write "Max"
	err = en.Append(0xa3, 0x4d, 0x61, 0x78)
	if err != nil {
		return
	}
	err = en.WriteDuration(z.Max)
	if err != nil {
		err = msgp.WrapError(err, "Max")
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z LatencyStat) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 3
	// string "Curr"
	o = append(o, 0x83, 0xa4, 0x43, 0x75, 0x72, 0x72)
	o = msgp.AppendDuration(o, z.Curr)
	// string "Avg"
	o = append(o, 0xa3, 0x41, 0x76, 0x67)
	o = msgp.AppendDuration(o, z.Avg)
	// string "Max"
	o = append(o, 0xa3, 0x4d, 0x61, 0x78)
	o = msgp.AppendDuration(o, z.Max)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *LatencyStat) UnmarshalMsg(bts []byte) (o []byte, err error) {
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
		case "Curr":
			z.Curr, bts, err = msgp.ReadDurationBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Curr")
				return
			}
		case "Avg":
			z.Avg, bts, err = msgp.ReadDurationBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Avg")
				return
			}
		case "Max":
			z.Max, bts, err = msgp.ReadDurationBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Max")
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
func (z LatencyStat) Msgsize() (s int) {
	s = 1 + 5 + msgp.DurationSize + 4 + msgp.DurationSize + 4 + msgp.DurationSize
	return
}

// DecodeMsg implements msgp.Decodable
func (z *RStat) DecodeMsg(dc *msgp.Reader) (err error) {
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
		case "Count":
			z.Count, err = dc.ReadFloat64()
			if err != nil {
				err = msgp.WrapError(err, "Count")
				return
			}
		case "Bytes":
			z.Bytes, err = dc.ReadInt64()
			if err != nil {
				err = msgp.WrapError(err, "Bytes")
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
func (z RStat) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 2
	// write "Count"
	err = en.Append(0x82, 0xa5, 0x43, 0x6f, 0x75, 0x6e, 0x74)
	if err != nil {
		return
	}
	err = en.WriteFloat64(z.Count)
	if err != nil {
		err = msgp.WrapError(err, "Count")
		return
	}
	// write "Bytes"
	err = en.Append(0xa5, 0x42, 0x79, 0x74, 0x65, 0x73)
	if err != nil {
		return
	}
	err = en.WriteInt64(z.Bytes)
	if err != nil {
		err = msgp.WrapError(err, "Bytes")
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z RStat) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 2
	// string "Count"
	o = append(o, 0x82, 0xa5, 0x43, 0x6f, 0x75, 0x6e, 0x74)
	o = msgp.AppendFloat64(o, z.Count)
	// string "Bytes"
	o = append(o, 0xa5, 0x42, 0x79, 0x74, 0x65, 0x73)
	o = msgp.AppendInt64(o, z.Bytes)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *RStat) UnmarshalMsg(bts []byte) (o []byte, err error) {
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
		case "Count":
			z.Count, bts, err = msgp.ReadFloat64Bytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Count")
				return
			}
		case "Bytes":
			z.Bytes, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Bytes")
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
func (z RStat) Msgsize() (s int) {
	s = 1 + 6 + msgp.Float64Size + 6 + msgp.Int64Size
	return
}

// DecodeMsg implements msgp.Decodable
func (z *ReplicationMRF) DecodeMsg(dc *msgp.Reader) (err error) {
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
		case "n":
			z.NodeName, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "NodeName")
				return
			}
		case "b":
			z.Bucket, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Bucket")
				return
			}
		case "o":
			z.Object, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Object")
				return
			}
		case "v":
			z.VersionID, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "VersionID")
				return
			}
		case "rc":
			z.RetryCount, err = dc.ReadInt()
			if err != nil {
				err = msgp.WrapError(err, "RetryCount")
				return
			}
		case "err":
			z.Err, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Err")
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
func (z *ReplicationMRF) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 6
	// write "n"
	err = en.Append(0x86, 0xa1, 0x6e)
	if err != nil {
		return
	}
	err = en.WriteString(z.NodeName)
	if err != nil {
		err = msgp.WrapError(err, "NodeName")
		return
	}
	// write "b"
	err = en.Append(0xa1, 0x62)
	if err != nil {
		return
	}
	err = en.WriteString(z.Bucket)
	if err != nil {
		err = msgp.WrapError(err, "Bucket")
		return
	}
	// write "o"
	err = en.Append(0xa1, 0x6f)
	if err != nil {
		return
	}
	err = en.WriteString(z.Object)
	if err != nil {
		err = msgp.WrapError(err, "Object")
		return
	}
	// write "v"
	err = en.Append(0xa1, 0x76)
	if err != nil {
		return
	}
	err = en.WriteString(z.VersionID)
	if err != nil {
		err = msgp.WrapError(err, "VersionID")
		return
	}
	// write "rc"
	err = en.Append(0xa2, 0x72, 0x63)
	if err != nil {
		return
	}
	err = en.WriteInt(z.RetryCount)
	if err != nil {
		err = msgp.WrapError(err, "RetryCount")
		return
	}
	// write "err"
	err = en.Append(0xa3, 0x65, 0x72, 0x72)
	if err != nil {
		return
	}
	err = en.WriteString(z.Err)
	if err != nil {
		err = msgp.WrapError(err, "Err")
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *ReplicationMRF) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 6
	// string "n"
	o = append(o, 0x86, 0xa1, 0x6e)
	o = msgp.AppendString(o, z.NodeName)
	// string "b"
	o = append(o, 0xa1, 0x62)
	o = msgp.AppendString(o, z.Bucket)
	// string "o"
	o = append(o, 0xa1, 0x6f)
	o = msgp.AppendString(o, z.Object)
	// string "v"
	o = append(o, 0xa1, 0x76)
	o = msgp.AppendString(o, z.VersionID)
	// string "rc"
	o = append(o, 0xa2, 0x72, 0x63)
	o = msgp.AppendInt(o, z.RetryCount)
	// string "err"
	o = append(o, 0xa3, 0x65, 0x72, 0x72)
	o = msgp.AppendString(o, z.Err)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *ReplicationMRF) UnmarshalMsg(bts []byte) (o []byte, err error) {
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
		case "n":
			z.NodeName, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "NodeName")
				return
			}
		case "b":
			z.Bucket, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Bucket")
				return
			}
		case "o":
			z.Object, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Object")
				return
			}
		case "v":
			z.VersionID, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "VersionID")
				return
			}
		case "rc":
			z.RetryCount, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "RetryCount")
				return
			}
		case "err":
			z.Err, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Err")
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
func (z *ReplicationMRF) Msgsize() (s int) {
	s = 1 + 2 + msgp.StringPrefixSize + len(z.NodeName) + 2 + msgp.StringPrefixSize + len(z.Bucket) + 2 + msgp.StringPrefixSize + len(z.Object) + 2 + msgp.StringPrefixSize + len(z.VersionID) + 3 + msgp.IntSize + 4 + msgp.StringPrefixSize + len(z.Err)
	return
}

// DecodeMsg implements msgp.Decodable
func (z *TimedErrStats) DecodeMsg(dc *msgp.Reader) (err error) {
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
		case "LastMinute":
			var zb0002 uint32
			zb0002, err = dc.ReadMapHeader()
			if err != nil {
				err = msgp.WrapError(err, "LastMinute")
				return
			}
			for zb0002 > 0 {
				zb0002--
				field, err = dc.ReadMapKeyPtr()
				if err != nil {
					err = msgp.WrapError(err, "LastMinute")
					return
				}
				switch msgp.UnsafeString(field) {
				case "Count":
					z.LastMinute.Count, err = dc.ReadFloat64()
					if err != nil {
						err = msgp.WrapError(err, "LastMinute", "Count")
						return
					}
				case "Bytes":
					z.LastMinute.Bytes, err = dc.ReadInt64()
					if err != nil {
						err = msgp.WrapError(err, "LastMinute", "Bytes")
						return
					}
				default:
					err = dc.Skip()
					if err != nil {
						err = msgp.WrapError(err, "LastMinute")
						return
					}
				}
			}
		case "LastHour":
			var zb0003 uint32
			zb0003, err = dc.ReadMapHeader()
			if err != nil {
				err = msgp.WrapError(err, "LastHour")
				return
			}
			for zb0003 > 0 {
				zb0003--
				field, err = dc.ReadMapKeyPtr()
				if err != nil {
					err = msgp.WrapError(err, "LastHour")
					return
				}
				switch msgp.UnsafeString(field) {
				case "Count":
					z.LastHour.Count, err = dc.ReadFloat64()
					if err != nil {
						err = msgp.WrapError(err, "LastHour", "Count")
						return
					}
				case "Bytes":
					z.LastHour.Bytes, err = dc.ReadInt64()
					if err != nil {
						err = msgp.WrapError(err, "LastHour", "Bytes")
						return
					}
				default:
					err = dc.Skip()
					if err != nil {
						err = msgp.WrapError(err, "LastHour")
						return
					}
				}
			}
		case "Totals":
			var zb0004 uint32
			zb0004, err = dc.ReadMapHeader()
			if err != nil {
				err = msgp.WrapError(err, "Totals")
				return
			}
			for zb0004 > 0 {
				zb0004--
				field, err = dc.ReadMapKeyPtr()
				if err != nil {
					err = msgp.WrapError(err, "Totals")
					return
				}
				switch msgp.UnsafeString(field) {
				case "Count":
					z.Totals.Count, err = dc.ReadFloat64()
					if err != nil {
						err = msgp.WrapError(err, "Totals", "Count")
						return
					}
				case "Bytes":
					z.Totals.Bytes, err = dc.ReadInt64()
					if err != nil {
						err = msgp.WrapError(err, "Totals", "Bytes")
						return
					}
				default:
					err = dc.Skip()
					if err != nil {
						err = msgp.WrapError(err, "Totals")
						return
					}
				}
			}
		case "ErrCounts":
			var zb0005 uint32
			zb0005, err = dc.ReadMapHeader()
			if err != nil {
				err = msgp.WrapError(err, "ErrCounts")
				return
			}
			if z.ErrCounts == nil {
				z.ErrCounts = make(map[string]int, zb0005)
			} else if len(z.ErrCounts) > 0 {
				for key := range z.ErrCounts {
					delete(z.ErrCounts, key)
				}
			}
			for zb0005 > 0 {
				zb0005--
				var za0001 string
				var za0002 int
				za0001, err = dc.ReadString()
				if err != nil {
					err = msgp.WrapError(err, "ErrCounts")
					return
				}
				za0002, err = dc.ReadInt()
				if err != nil {
					err = msgp.WrapError(err, "ErrCounts", za0001)
					return
				}
				z.ErrCounts[za0001] = za0002
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
func (z *TimedErrStats) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 4
	// write "LastMinute"
	err = en.Append(0x84, 0xaa, 0x4c, 0x61, 0x73, 0x74, 0x4d, 0x69, 0x6e, 0x75, 0x74, 0x65)
	if err != nil {
		return
	}
	// map header, size 2
	// write "Count"
	err = en.Append(0x82, 0xa5, 0x43, 0x6f, 0x75, 0x6e, 0x74)
	if err != nil {
		return
	}
	err = en.WriteFloat64(z.LastMinute.Count)
	if err != nil {
		err = msgp.WrapError(err, "LastMinute", "Count")
		return
	}
	// write "Bytes"
	err = en.Append(0xa5, 0x42, 0x79, 0x74, 0x65, 0x73)
	if err != nil {
		return
	}
	err = en.WriteInt64(z.LastMinute.Bytes)
	if err != nil {
		err = msgp.WrapError(err, "LastMinute", "Bytes")
		return
	}
	// write "LastHour"
	err = en.Append(0xa8, 0x4c, 0x61, 0x73, 0x74, 0x48, 0x6f, 0x75, 0x72)
	if err != nil {
		return
	}
	// map header, size 2
	// write "Count"
	err = en.Append(0x82, 0xa5, 0x43, 0x6f, 0x75, 0x6e, 0x74)
	if err != nil {
		return
	}
	err = en.WriteFloat64(z.LastHour.Count)
	if err != nil {
		err = msgp.WrapError(err, "LastHour", "Count")
		return
	}
	// write "Bytes"
	err = en.Append(0xa5, 0x42, 0x79, 0x74, 0x65, 0x73)
	if err != nil {
		return
	}
	err = en.WriteInt64(z.LastHour.Bytes)
	if err != nil {
		err = msgp.WrapError(err, "LastHour", "Bytes")
		return
	}
	// write "Totals"
	err = en.Append(0xa6, 0x54, 0x6f, 0x74, 0x61, 0x6c, 0x73)
	if err != nil {
		return
	}
	// map header, size 2
	// write "Count"
	err = en.Append(0x82, 0xa5, 0x43, 0x6f, 0x75, 0x6e, 0x74)
	if err != nil {
		return
	}
	err = en.WriteFloat64(z.Totals.Count)
	if err != nil {
		err = msgp.WrapError(err, "Totals", "Count")
		return
	}
	// write "Bytes"
	err = en.Append(0xa5, 0x42, 0x79, 0x74, 0x65, 0x73)
	if err != nil {
		return
	}
	err = en.WriteInt64(z.Totals.Bytes)
	if err != nil {
		err = msgp.WrapError(err, "Totals", "Bytes")
		return
	}
	// write "ErrCounts"
	err = en.Append(0xa9, 0x45, 0x72, 0x72, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x73)
	if err != nil {
		return
	}
	err = en.WriteMapHeader(uint32(len(z.ErrCounts)))
	if err != nil {
		err = msgp.WrapError(err, "ErrCounts")
		return
	}
	for za0001, za0002 := range z.ErrCounts {
		err = en.WriteString(za0001)
		if err != nil {
			err = msgp.WrapError(err, "ErrCounts")
			return
		}
		err = en.WriteInt(za0002)
		if err != nil {
			err = msgp.WrapError(err, "ErrCounts", za0001)
			return
		}
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *TimedErrStats) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 4
	// string "LastMinute"
	o = append(o, 0x84, 0xaa, 0x4c, 0x61, 0x73, 0x74, 0x4d, 0x69, 0x6e, 0x75, 0x74, 0x65)
	// map header, size 2
	// string "Count"
	o = append(o, 0x82, 0xa5, 0x43, 0x6f, 0x75, 0x6e, 0x74)
	o = msgp.AppendFloat64(o, z.LastMinute.Count)
	// string "Bytes"
	o = append(o, 0xa5, 0x42, 0x79, 0x74, 0x65, 0x73)
	o = msgp.AppendInt64(o, z.LastMinute.Bytes)
	// string "LastHour"
	o = append(o, 0xa8, 0x4c, 0x61, 0x73, 0x74, 0x48, 0x6f, 0x75, 0x72)
	// map header, size 2
	// string "Count"
	o = append(o, 0x82, 0xa5, 0x43, 0x6f, 0x75, 0x6e, 0x74)
	o = msgp.AppendFloat64(o, z.LastHour.Count)
	// string "Bytes"
	o = append(o, 0xa5, 0x42, 0x79, 0x74, 0x65, 0x73)
	o = msgp.AppendInt64(o, z.LastHour.Bytes)
	// string "Totals"
	o = append(o, 0xa6, 0x54, 0x6f, 0x74, 0x61, 0x6c, 0x73)
	// map header, size 2
	// string "Count"
	o = append(o, 0x82, 0xa5, 0x43, 0x6f, 0x75, 0x6e, 0x74)
	o = msgp.AppendFloat64(o, z.Totals.Count)
	// string "Bytes"
	o = append(o, 0xa5, 0x42, 0x79, 0x74, 0x65, 0x73)
	o = msgp.AppendInt64(o, z.Totals.Bytes)
	// string "ErrCounts"
	o = append(o, 0xa9, 0x45, 0x72, 0x72, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x73)
	o = msgp.AppendMapHeader(o, uint32(len(z.ErrCounts)))
	for za0001, za0002 := range z.ErrCounts {
		o = msgp.AppendString(o, za0001)
		o = msgp.AppendInt(o, za0002)
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *TimedErrStats) UnmarshalMsg(bts []byte) (o []byte, err error) {
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
		case "LastMinute":
			var zb0002 uint32
			zb0002, bts, err = msgp.ReadMapHeaderBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "LastMinute")
				return
			}
			for zb0002 > 0 {
				zb0002--
				field, bts, err = msgp.ReadMapKeyZC(bts)
				if err != nil {
					err = msgp.WrapError(err, "LastMinute")
					return
				}
				switch msgp.UnsafeString(field) {
				case "Count":
					z.LastMinute.Count, bts, err = msgp.ReadFloat64Bytes(bts)
					if err != nil {
						err = msgp.WrapError(err, "LastMinute", "Count")
						return
					}
				case "Bytes":
					z.LastMinute.Bytes, bts, err = msgp.ReadInt64Bytes(bts)
					if err != nil {
						err = msgp.WrapError(err, "LastMinute", "Bytes")
						return
					}
				default:
					bts, err = msgp.Skip(bts)
					if err != nil {
						err = msgp.WrapError(err, "LastMinute")
						return
					}
				}
			}
		case "LastHour":
			var zb0003 uint32
			zb0003, bts, err = msgp.ReadMapHeaderBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "LastHour")
				return
			}
			for zb0003 > 0 {
				zb0003--
				field, bts, err = msgp.ReadMapKeyZC(bts)
				if err != nil {
					err = msgp.WrapError(err, "LastHour")
					return
				}
				switch msgp.UnsafeString(field) {
				case "Count":
					z.LastHour.Count, bts, err = msgp.ReadFloat64Bytes(bts)
					if err != nil {
						err = msgp.WrapError(err, "LastHour", "Count")
						return
					}
				case "Bytes":
					z.LastHour.Bytes, bts, err = msgp.ReadInt64Bytes(bts)
					if err != nil {
						err = msgp.WrapError(err, "LastHour", "Bytes")
						return
					}
				default:
					bts, err = msgp.Skip(bts)
					if err != nil {
						err = msgp.WrapError(err, "LastHour")
						return
					}
				}
			}
		case "Totals":
			var zb0004 uint32
			zb0004, bts, err = msgp.ReadMapHeaderBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Totals")
				return
			}
			for zb0004 > 0 {
				zb0004--
				field, bts, err = msgp.ReadMapKeyZC(bts)
				if err != nil {
					err = msgp.WrapError(err, "Totals")
					return
				}
				switch msgp.UnsafeString(field) {
				case "Count":
					z.Totals.Count, bts, err = msgp.ReadFloat64Bytes(bts)
					if err != nil {
						err = msgp.WrapError(err, "Totals", "Count")
						return
					}
				case "Bytes":
					z.Totals.Bytes, bts, err = msgp.ReadInt64Bytes(bts)
					if err != nil {
						err = msgp.WrapError(err, "Totals", "Bytes")
						return
					}
				default:
					bts, err = msgp.Skip(bts)
					if err != nil {
						err = msgp.WrapError(err, "Totals")
						return
					}
				}
			}
		case "ErrCounts":
			var zb0005 uint32
			zb0005, bts, err = msgp.ReadMapHeaderBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "ErrCounts")
				return
			}
			if z.ErrCounts == nil {
				z.ErrCounts = make(map[string]int, zb0005)
			} else if len(z.ErrCounts) > 0 {
				for key := range z.ErrCounts {
					delete(z.ErrCounts, key)
				}
			}
			for zb0005 > 0 {
				var za0001 string
				var za0002 int
				zb0005--
				za0001, bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					err = msgp.WrapError(err, "ErrCounts")
					return
				}
				za0002, bts, err = msgp.ReadIntBytes(bts)
				if err != nil {
					err = msgp.WrapError(err, "ErrCounts", za0001)
					return
				}
				z.ErrCounts[za0001] = za0002
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
func (z *TimedErrStats) Msgsize() (s int) {
	s = 1 + 11 + 1 + 6 + msgp.Float64Size + 6 + msgp.Int64Size + 9 + 1 + 6 + msgp.Float64Size + 6 + msgp.Int64Size + 7 + 1 + 6 + msgp.Float64Size + 6 + msgp.Int64Size + 10 + msgp.MapHeaderSize
	if z.ErrCounts != nil {
		for za0001, za0002 := range z.ErrCounts {
			_ = za0002
			s += msgp.StringPrefixSize + len(za0001) + msgp.IntSize
		}
	}
	return
}
