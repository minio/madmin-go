//go:build ignore
// +build ignore

//
// MinIO Object Storage (c) 2022 MinIO, Inc.
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

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/minio/madmin-go"
)

func main() {
	// Note: YOUR-ACCESSKEYID, YOUR-SECRETACCESSKEY and my-bucketname are
	// dummy values, please replace them with original values.

	// API requests are secure (HTTPS) if secure=true and insecure (HTTP) otherwise.
	// New returns an MinIO Admin client object.
	madmClnt, err := madmin.New("your-minio.example.com:9000", "YOUR-ACCESSKEYID", "YOUR-SECRETACCESSKEY", true)
	if err != nil {
		log.Fatalln(err)
	}
	// Print a diff of unreplicated objects in a particular prefix in my-bucketname for the remote target specified.
	// Leaving out the ARN returns
	diffCh := madmClnt.BucketReplicationDiff(context.Background(), "my-bucketname", ReplDiffOpts{
		ARN:    "<remote-arn>",
		Prefix: "prefix/path",
	})
	for diff := range diffCh {
		if diff.Err != nil {
			log.Fatalln(diff.Err)
		}
		fmt.Println(diff)
	}
}
