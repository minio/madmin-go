//
// MinIO Object Storage (c) 2021 MinIO, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package madmin

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

var (
	withCreateDate    = []byte(`{"PolicyName":"readwrite","Policy":{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["admin:*"]},{"Effect":"Allow","Action":["s3:*"],"Resource":["arn:aws:s3:::*"]}]},"CreateDate":"2020-03-15T10:10:10Z","UpdateDate":"2021-03-15T10:10:10Z"}`)
	withoutCreateDate = []byte(`{"PolicyName":"readwrite","Policy":{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["admin:*"]},{"Effect":"Allow","Action":["s3:*"],"Resource":["arn:aws:s3:::*"]}]}}`)
)

func TestPolicyInfo(t *testing.T) {
	testCases := []struct {
		pi          *PolicyInfo
		expectedBuf []byte
	}{
		{
			&PolicyInfo{
				PolicyName: "readwrite",
				Policy:     []byte(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["admin:*"]},{"Effect":"Allow","Action":["s3:*"],"Resource":["arn:aws:s3:::*"]}]}`),
				CreateDate: time.Date(2020, time.March, 15, 10, 10, 10, 0, time.UTC),
				UpdateDate: time.Date(2021, time.March, 15, 10, 10, 10, 0, time.UTC),
			},
			withCreateDate,
		},
		{
			&PolicyInfo{
				PolicyName: "readwrite",
				Policy:     []byte(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["admin:*"]},{"Effect":"Allow","Action":["s3:*"],"Resource":["arn:aws:s3:::*"]}]}`),
			},
			withoutCreateDate,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run("", func(t *testing.T) {
			buf, err := json.Marshal(testCase.pi)
			if err != nil {
				t.Error(err)
			}
			if !bytes.Equal(buf, testCase.expectedBuf) {
				t.Errorf("expected %s, got %s", string(testCase.expectedBuf), string(buf))
			}
		})
	}
}
