//go:build ignore
// +build ignore

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
	"log"

	"github.com/minio/madmin-go/v3"
)

func main() {
	// Note: YOUR-ACCESSKEYID, YOUR-SECRETACCESSKEY and my-bucketname are
	// dummy values, please replace them with original values.

	// API requests are secure (HTTPS) if secure=true and insecure (HTTP) otherwise.
	// New returns an MinIO Admin client object.
	bucket := "bucket"
	client, err := madmin.New("your-minio.example.com:9000", "YOUR-ACCESSKEYID", "YOUR-SECRETACCESSKEY", true)
	if err != nil {
		log.Fatalln(err)
	}
	// return bucket metadata as zipped content
	r, err := client.ExportBucketMetadata(context.Background(), bucket)
	if err != nil {
		log.Fatalln(err)
	}

	// set bucket metadata to bucket on a new cluster
	client2, err := madmin.New("your-minio.example.com:9001", "YOUR-ACCESSKEYID", "YOUR-SECRETACCESSKEY", true)
	if err != nil {
		log.Fatalln(err)
	}
	bucket2 := "bucket"
	// set bucket metadata from reader
	if err := client2.ImportBucketMetadata(context.Background(), bucket2, r); err != nil {
		log.Fatalln(err)
	}
}
