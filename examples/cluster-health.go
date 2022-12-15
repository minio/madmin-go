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

	"github.com/minio/madmin-go/v2"
)

func main() {
	// API requests are secure (HTTPS) if secure=true and insecure (HTTPS) otherwise.
	// NewAnonymousClient returns an anonymous MinIO Admin client object.
	// Anonymous client doesn't require any credentials
	madmAnonClnt, err := madmin.NewAnonymousClient("your-minio.example.com:9000", true)
	if err != nil {
		log.Fatalln(err)
	}
	// To enable trace :-
	// madmAnonClnt.TraceOn(os.Stderr)
	opts := madmin.HealthOpts{
		ClusterRead: false, // set to "true" to check if the cluster has read quorum
		Maintenance: false, // set to "true" to check if the cluster is taken down for maintenance
	}
	healthResult, err := madmAnonClnt.Healthy(context.Background(), opts)
	if err != nil {
		log.Fatalln(err)
	}
	log.Println(healthResult)
}
