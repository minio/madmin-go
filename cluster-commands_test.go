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
	"bytes"
	"testing"
)

func TestSRSessionPolicy_MarshalUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name        string
		policyBytes []byte
	}{
		{
			name:        "ValidPolicy",
			policyBytes: []byte(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"s3:GetObject","Resource":"arn:aws:s3:::example-bucket/*"}]}`),
		},
		{
			name:        "EmptyPolicy",
			policyBytes: []byte(``),
		},
		{
			name:        "NullPolicy",
			policyBytes: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert policy bytes to SRSessionPolicy
			policy := SRSessionPolicy(tt.policyBytes)

			// Test MarshalJSON
			data, err := policy.MarshalJSON()
			if err != nil {
				t.Errorf("SRSessionPolicy.MarshalJSON() error = %v", err)
				return
			}

			// Test UnmarshalJSON
			var got SRSessionPolicy
			if err := got.UnmarshalJSON(data); err != nil {
				t.Errorf("SRSessionPolicy.UnmarshalJSON() error = %v", err)
				return
			}

			// Check if the result matches the original policy
			if !bytes.Equal(got, tt.policyBytes) {
				t.Errorf("SRSessionPolicy.UnmarshalJSON() = %s, want %s", got, tt.policyBytes)
			}
		})
	}
}
