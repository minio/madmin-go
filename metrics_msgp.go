//
// Copyright (c) 2015-2024 MinIO, Inc.
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.
//

package madmin

import (
	"time"

	"github.com/tinylib/msgp/msgp"
)

// generic types are manually serialized for now.

// DecodeMsg implements msgp.Decodable
func (z *Segmented[T, PT]) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, err = dc.ReadMapHeader()
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	var zb0001Mask uint8 /* 3 bits */
	_ = zb0001Mask
	for zb0001 > 0 {
		zb0001--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "int":
			z.Interval, err = dc.ReadInt()
			if err != nil {
				err = msgp.WrapError(err, "Interval")
				return
			}
			zb0001Mask |= 0x1
		case "ft":
			z.FirstTime, err = dc.ReadTimeUTC()
			if err != nil {
				err = msgp.WrapError(err, "FirstTime")
				return
			}
			zb0001Mask |= 0x2
		case "segs":
			var zb0002 uint32
			zb0002, err = dc.ReadArrayHeader()
			if err != nil {
				err = msgp.WrapError(err, "Segments")
				return
			}
			if cap(z.Segments) >= int(zb0002) {
				z.Segments = (z.Segments)[:zb0002]
			} else {
				z.Segments = make([]T, zb0002)
			}
			for za0001 := range z.Segments {
				err = PT(&z.Segments[za0001]).DecodeMsg(dc)
				if err != nil {
					err = msgp.WrapError(err, "Segments", za0001)
					return
				}
			}
			zb0001Mask |= 0x4
		default:
			err = dc.Skip()
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	// Clear omitted fields.
	if zb0001Mask != 0x7 {
		if (zb0001Mask & 0x1) == 0 {
			z.Interval = 0
		}
		if (zb0001Mask & 0x2) == 0 {
			z.FirstTime = (time.Time{})
		}
		if (zb0001Mask & 0x4) == 0 {
			z.Segments = nil
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *Segmented[T, PT]) EncodeMsg(en *msgp.Writer) (err error) {
	// check for omitted fields
	zb0001Len := uint32(3)
	var zb0001Mask uint8 /* 3 bits */
	_ = zb0001Mask
	if z.Interval == 0 {
		zb0001Len--
		zb0001Mask |= 0x1
	}
	if z.FirstTime.IsZero() {
		zb0001Len--
		zb0001Mask |= 0x2
	}
	if z.Segments == nil {
		zb0001Len--
		zb0001Mask |= 0x4
	}
	// variable map header, size zb0001Len
	err = en.Append(0x80 | uint8(zb0001Len))
	if err != nil {
		return
	}

	// skip if no fields are to be emitted
	if zb0001Len != 0 {
		if (zb0001Mask & 0x1) == 0 { // if not omitted
			// write "int"
			err = en.Append(0xa3, 0x69, 0x6e, 0x74)
			if err != nil {
				return
			}
			err = en.WriteInt(z.Interval)
			if err != nil {
				err = msgp.WrapError(err, "Interval")
				return
			}
		}
		if (zb0001Mask & 0x2) == 0 { // if not omitted
			// write "ft"
			err = en.Append(0xa2, 0x66, 0x74)
			if err != nil {
				return
			}
			err = en.WriteTime(z.FirstTime)
			if err != nil {
				err = msgp.WrapError(err, "FirstTime")
				return
			}
		}
		if (zb0001Mask & 0x4) == 0 { // if not omitted
			// write "segs"
			err = en.Append(0xa4, 0x73, 0x65, 0x67, 0x73)
			if err != nil {
				return
			}
			err = en.WriteArrayHeader(uint32(len(z.Segments)))
			if err != nil {
				err = msgp.WrapError(err, "Segments")
				return
			}
			for za0001 := range z.Segments {
				err = PT(&z.Segments[za0001]).EncodeMsg(en)
				if err != nil {
					err = msgp.WrapError(err, "Segments", za0001)
					return
				}
			}
		}
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *Segmented[T, PT]) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// check for omitted fields
	zb0001Len := uint32(3)
	var zb0001Mask uint8 /* 3 bits */
	_ = zb0001Mask
	if z.Interval == 0 {
		zb0001Len--
		zb0001Mask |= 0x1
	}
	if z.FirstTime.IsZero() {
		zb0001Len--
		zb0001Mask |= 0x2
	}
	if z.Segments == nil {
		zb0001Len--
		zb0001Mask |= 0x4
	}
	// variable map header, size zb0001Len
	o = append(o, 0x80|uint8(zb0001Len))

	// skip if no fields are to be emitted
	if zb0001Len != 0 {
		if (zb0001Mask & 0x1) == 0 { // if not omitted
			// string "int"
			o = append(o, 0xa3, 0x69, 0x6e, 0x74)
			o = msgp.AppendInt(o, z.Interval)
		}
		if (zb0001Mask & 0x2) == 0 { // if not omitted
			// string "ft"
			o = append(o, 0xa2, 0x66, 0x74)
			o = msgp.AppendTime(o, z.FirstTime)
		}
		if (zb0001Mask & 0x4) == 0 { // if not omitted
			// string "segs"
			o = append(o, 0xa4, 0x73, 0x65, 0x67, 0x73)
			o = msgp.AppendArrayHeader(o, uint32(len(z.Segments)))
			for za0001 := range z.Segments {
				o, err = PT(&z.Segments[za0001]).MarshalMsg(o)
				if err != nil {
					err = msgp.WrapError(err, "Segments", za0001)
					return
				}
			}
		}
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Segmented[T, PT]) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	var zb0001Mask uint8 /* 3 bits */
	_ = zb0001Mask
	for zb0001 > 0 {
		zb0001--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "int":
			z.Interval, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Interval")
				return
			}
			zb0001Mask |= 0x1
		case "ft":
			z.FirstTime, bts, err = msgp.ReadTimeUTCBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "FirstTime")
				return
			}
			zb0001Mask |= 0x2
		case "segs":
			var zb0002 uint32
			zb0002, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Segments")
				return
			}
			if cap(z.Segments) >= int(zb0002) {
				z.Segments = (z.Segments)[:zb0002]
			} else {
				z.Segments = make([]T, zb0002)
			}
			for za0001 := range z.Segments {
				bts, err = PT(&z.Segments[za0001]).UnmarshalMsg(bts)
				if err != nil {
					err = msgp.WrapError(err, "Segments", za0001)
					return
				}
			}
			zb0001Mask |= 0x4
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	// Clear omitted fields.
	if zb0001Mask != 0x7 {
		if (zb0001Mask & 0x1) == 0 {
			z.Interval = 0
		}
		if (zb0001Mask & 0x2) == 0 {
			z.FirstTime = (time.Time{})
		}
		if (zb0001Mask & 0x4) == 0 {
			z.Segments = nil
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *Segmented[T, PT]) Msgsize() (s int) {
	s = 1 + 4 + msgp.IntSize + 3 + msgp.TimeSize + 5 + msgp.ArrayHeaderSize
	for za0001 := range z.Segments {
		s += PT(&z.Segments[za0001]).Msgsize()
	}
	return
}
