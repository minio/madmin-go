//
// Copyright (c) 2015-2022 MinIO, Inc.
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

//go:build fips
// +build fips

package madmin

// FIPSEnabled returns true if and only if FIPS 140-2 support
// is enabled.
//
// FIPS 140-2 requires that only specifc cryptographic
// primitives, like AES or SHA-256, are used and that
// those primitives are implemented by a FIPS 140-2
// certified cryptographic module.
func FIPSEnabled() bool { return true }
