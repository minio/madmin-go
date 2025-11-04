//go:build ignore

//
// Copyright (c) 2015-2025 MinIO, Inc.
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

	"github.com/minio/madmin-go/v4"
)

var client *madmin.AdminClient

func main() {
	verifyTLS := false
	var err error
	client, err = madmin.New("127.0.0.1:9001", "minio", "minio123", verifyTLS)
	if err != nil {
		log.Fatalln(err)
	}

	getClusterSummary()
	getPools()
	getSinglePool()
	getErasureSetsForSinglePool()
	getDrivesForSinglePool()
	getNode()
}

func getClusterSummary() {
	resp, xerr := client.ClusterSummaryQuery(context.Background(), madmin.ClusterSummaryResourceOpts{})
	if xerr != nil {
		log.Fatalln(xerr)
	}

	fmt.Printf("%+v", resp)
}

func getPools() {
	resp, xerr := client.PoolsQuery(context.Background(), &madmin.PoolsResourceOpts{
		Offset: 0,
		Limit:  1000,
		Filter: "",
		Sort:   "PoolIndex",
	})
	if xerr != nil {
		log.Fatalln(xerr)
	}

	for _, v := range resp.Results {
		fmt.Printf("%+v\n", v)
	}
}

func getSinglePool() {
	resp, xerr := client.PoolsQuery(context.Background(), &madmin.PoolsResourceOpts{
		Offset: 0,
		Limit:  1,
		Filter: "PoolIndex = 1",
	})
	if xerr != nil {
		log.Fatalln(xerr)
	}

	for _, v := range resp.Results {
		fmt.Printf("%+v\n", v)
	}
}

func getErasureSetsForSinglePool() {
	resp, xerr := client.ErasureSetsQuery(context.Background(), &madmin.ErasureSetsResourceOpts{
		Offset:       0,
		Limit:        1000,
		Filter:       "PoolIndex = 1",
		Sort:         "SetIndex",
		SortReversed: false,
	})
	if xerr != nil {
		log.Fatalln(xerr)
	}

	for _, v := range resp.Results {
		fmt.Printf("%+v\n", v)
	}
}

func getDrivesForSinglePool() {
	resp, xerr := client.DrivesQuery(context.Background(), &madmin.DrivesResourceOpts{
		Offset:       0,
		Limit:        1000,
		Filter:       "PoolIndex = 1",
		Sort:         "SetIndex",
		SortReversed: false,
	})
	if xerr != nil {
		log.Fatalln(xerr)
	}

	for _, v := range resp.Results {
		fmt.Printf("%+v\n", v)
	}
}

func getNode() {
	resp, xerr := client.NodesQuery(context.Background(), &madmin.NodesResourceOpts{
		Offset:       0,
		Limit:        1,
		Filter:       "",
		Sort:         "",
		SortReversed: false,
	})
	if xerr != nil {
		log.Fatalln(xerr)
	}
	b, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Println(string(b))
}
