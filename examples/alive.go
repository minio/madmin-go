//
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

//go:build ignore
// +build ignore

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

package main

import (
	"context"
	"log"

	"github.com/minio/madmin-go/v2"
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

	st, err := madmClnt.ServerInfo(context.Background())
	if err != nil {
		log.Fatalln(err)
	}

	// API requests are secure (HTTPS) if secure=true and insecure (HTTPS) otherwise.
	// NewAnonymousClient returns an anonymous MinIO Admin client object.
	// Anonymous client doesn't require any credentials
	madmAnonClnt, err := madmin.NewAnonymousClient("your-minio.example.com:9000", true)
	if err != nil {
		log.Fatalln(err)
	}

	// madmAnonClnt.TraceOn(os.Stderr)
	for aliveResult := range madmAnonClnt.Alive(context.Background(), madmin.AliveOpts{}, st.Servers...) {
		log.Printf("%+v\n", aliveResult)
	}
}
