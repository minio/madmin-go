//
//go:build ignore
// +build ignore

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

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/minio/madmin-go/v4"
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
	expiration := time.Now().Add(30 * time.Minute)
	addReq := madmin.AddServiceAccountReq{
		TargetUser: "my-username",
		Expiration: &expiration,
	}
	addRes, err := madminClient.AddServiceAccount(context.Background(), addReq)
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(addRes)

	// update service account
	newExpiration := time.Now().Add(45 * time.Minute)
	updateReq := madmin.UpdateServiceAccountReq{
		NewStatus:     "my-status",
		NewExpiration: &newExpiration,
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
