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
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/minio/madmin-go/v2"
)

func main() {
	// Note: YOUR-ACCESSKEYID, YOUR-SECRETACCESSKEY are
	// dummy values, please replace them with original values.

	// API requests are secure (HTTPS) if secure=true and insecure (HTTP) otherwise.
	// New returns an MinIO Admin client object.
	madmClnt, err := madmin.New("your-minio.example.com:9000", "YOUR-ACCESSKEYID", "YOUR-SECRETACCESSKEY", true)
	if err != nil {
		log.Fatalln(err)
	}

	opts := madmin.HealOpts{
		Recursive: true,                  // recursively heal all objects at 'prefix'
		Remove:    true,                  // remove content that has lost quorum and not recoverable
		Recreate:  true,                  // rewrite all old non-inlined xl.meta to new xl.meta
		ScanMode:  madmin.HealNormalScan, // by default do not do 'deep' scanning
	}

	start, _, err := madmClnt.Heal(context.Background(), "healing-rewrite-bucket", "", opts, "", false, false)
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println("Healstart sequence ===")
	enc := json.NewEncoder(os.Stdout)
	if err = enc.Encode(&start); err != nil {
		log.Fatalln(err)
	}

	fmt.Println()
	for {
		_, status, err := madmClnt.Heal(context.Background(), "healing-rewrite-bucket", "", opts, start.ClientToken, false, false)
		if status.Summary == "finished" {
			fmt.Println("Healstatus on items ===")
			for _, item := range status.Items {
				if err = enc.Encode(&item); err != nil {
					log.Fatalln(err)
				}
			}
			break
		}
		if status.Summary == "stopped" {
			fmt.Println("Healstatus on items ===")
			fmt.Println("Heal failed with", status.FailureDetail)
			break
		}

		for _, item := range status.Items {
			if err = enc.Encode(&item); err != nil {
				log.Fatalln(err)
			}
		}

		time.Sleep(time.Second)
	}
}
