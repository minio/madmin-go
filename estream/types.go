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

package estream

//go:generate stringer -type=blockID -trimprefix=block

type blockID int8

const (
	blockPlainKey blockID = iota + 1
	blockEncryptedKey
	blockEncStream
	blockPlainStream
	blockDatablock
	blockEOS
	blockEOF
	blockError
)

type checksumType uint8

//go:generate stringer -type=checksumType -trimprefix=checksumType

const (
	checksumTypeNone checksumType = iota
	checksumTypeXxhash

	checksumTypeUnknown
)

func (c checksumType) valid() bool {
	return c >= checksumTypeNone && c < checksumTypeUnknown
}
