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

// Package madmin_test
package madmin_test

import (
	"testing"

	"github.com/minio/madmin-go"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func TestMinioAdminClient(t *testing.T) {
	_, err := madmin.New("localhost:9000", "food", "food123", true)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMinioAdminClientWithOpts(t *testing.T) {
	creds := credentials.NewStaticV4("food", "food123", "")
	opts := madmin.Options{
		Creds:              creds,
		Secure:             true,
		InsecureSkipVerify: true,
	}
	_, err := madmin.NewWithOptions("localhost:9000", &opts)
	if err != nil {
		t.Fatal(err)
	}
}
