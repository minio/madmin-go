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

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsAdminSelfLockout(t *testing.T) {
	lockout := ErrorResponse{
		Code:    AdminSelfLockoutErrorCode,
		Message: AdminSelfLockoutMessage,
	}
	if !IsAdminSelfLockout(lockout) {
		t.Fatal("expected direct ErrorResponse to be self-lockout")
	}
	if !IsAdminSelfLockout(fmt.Errorf("wrapped: %w", lockout)) {
		t.Fatal("expected wrapped ErrorResponse to be self-lockout")
	}
	if !IsAdminSelfLockout(&lockout) {
		t.Fatal("expected pointer ErrorResponse to be self-lockout")
	}
	if !IsAdminSelfLockout(fmt.Errorf("wrapped: %w", &lockout)) {
		t.Fatal("expected wrapped pointer ErrorResponse to be self-lockout")
	}

	other := ErrorResponse{Code: "XMinioAdminNoSuchUser", Message: "no user"}
	if IsAdminSelfLockout(other) {
		t.Fatal("expected other admin code to not be self-lockout")
	}
	if IsAdminSelfLockout(&other) {
		t.Fatal("expected other pointer admin code to not be self-lockout")
	}
	if IsAdminSelfLockout(errors.New("plain error")) {
		t.Fatal("expected non-ErrorResponse to not be self-lockout")
	}
	if IsAdminSelfLockout(nil) {
		t.Fatal("expected nil to not be self-lockout")
	}
	var nilResp *ErrorResponse
	if IsAdminSelfLockout(nilResp) {
		t.Fatal("expected nil *ErrorResponse to not be self-lockout")
	}
}

func TestAdminSelfLockoutConstants(t *testing.T) {
	if AdminSelfLockoutErrorCode != "XMinioAdminSelfLockout" {
		t.Fatalf("code = %q", AdminSelfLockoutErrorCode)
	}
	if AdminSelfLockoutMessage == "" {
		t.Fatal("expected non-empty default message")
	}
}
