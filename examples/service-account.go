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
	madminClient, err := madmin.New("your-minio.example.com:9000", "YOUR-ACCESSKEYID", "YOUR-SECRETACCESSKEY", true)
	if err != nil {
		log.Fatalln(err)
	}
	ctx := context.Background()

	// add service account
	addReq := madmin.AddServiceAccountReq{
		TargetUser: "my-username",
	}
	addRes, err := madminClient.AddServiceAccount(context.Background(), addReq)
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(addRes)

	// update service account
	updateReq := madmin.UpdateServiceAccountReq{
		NewStatus: "my-status",
	}
	if err := madminClient.UpdateServiceAccount(ctx, "my-accesskey", updateReq); err != nil {
		log.Fatalln(err)
	}

	// get service account
	listRes, err := madminClient.ListServiceAccounts(ctx, "my-username")
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(listRes)

	// delete service account
	if err := madminClient.DeleteServiceAccount(ctx, "my-accesskey"); err != nil {
		log.Fatalln(err)
	}
}
