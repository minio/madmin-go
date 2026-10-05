// Copyright (c) 2015-2026 MinIO, Inc.
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
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package madmin

// AdminSelfLockoutErrorCode is returned when an IAM admin mutation would remove
// the caller's own access (HTTP 403 Forbidden).
const AdminSelfLockoutErrorCode = "XMinioAdminSelfLockout"

// AdminSelfLockoutMessage is the default Description for AdminSelfLockoutErrorCode.
const AdminSelfLockoutMessage = "This operation would remove your own access"

// IsAdminSelfLockout reports whether err, or any error it wraps or joins, is
// an admin API self-lockout refusal.
func IsAdminSelfLockout(err error) bool {
	switch e := err.(type) {
	case nil:
		return false
	case ErrorResponse:
		if e.Code == AdminSelfLockoutErrorCode {
			return true
		}
	case *ErrorResponse:
		if e != nil && e.Code == AdminSelfLockoutErrorCode {
			return true
		}
	}
	switch e := err.(type) {
	case interface{ Unwrap() []error }:
		for _, child := range e.Unwrap() {
			if IsAdminSelfLockout(child) {
				return true
			}
		}
	case interface{ Unwrap() error }:
		return IsAdminSelfLockout(e.Unwrap())
	}
	return false
}
