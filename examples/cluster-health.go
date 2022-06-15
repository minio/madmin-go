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
	"log"

	"github.com/minio/madmin-go"
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
	// madmAnonClnt.TraceOn(nil)
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
