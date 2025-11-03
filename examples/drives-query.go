//
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
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	// Initialize admin client with credentials
	madmClnt, err := madmin.NewWithOptions("127.0.0.1:9001", &madmin.Options{
		Creds:  credentials.NewStaticV4("minio", "minio123", ""),
		Secure: false, // Set to true for HTTPS
	})
	if err != nil {
		log.Fatalln("Error initializing admin client:", err)
	}

	// Example 1: Basic query without any options
	fmt.Println("=== Basic DrivesQuery (all drives) ===")
	drives, err := madmClnt.DrivesQuery(context.Background(), nil)
	if err != nil {
		log.Println("Error querying drives:", err)
	} else {
		printDrivesResponse(drives)
	}

	// Example 2: Query with pagination
	fmt.Println("\n=== DrivesQuery with pagination (limit: 10, offset: 0) ===")
	drives, err = madmClnt.DrivesQuery(context.Background(), &madmin.DrivesResourceOpts{
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		log.Println("Error querying drives with pagination:", err)
	} else {
		printDrivesResponse(drives)
	}

	// Example 3: Query with filter
	fmt.Println("\n=== DrivesQuery with filter ===")
	drives, err = madmClnt.DrivesQuery(context.Background(), &madmin.DrivesResourceOpts{
		Filter: "DriveIndex = 2", // Example filter - adjust based on your needs
		Limit:  10,
	})
	if err != nil {
		log.Println("Error querying drives with filter:", err)
	} else {
		printDrivesResponse(drives)
	}

	// Example 4: Query with metrics enabled
	fmt.Println("\n=== DrivesQuery with metrics ===")
	drives, err = madmClnt.DrivesQuery(context.Background(), &madmin.DrivesResourceOpts{
		Metrics: true,
		Limit:   5,
	})
	if err != nil {
		log.Println("Error querying drives with metrics:", err)
	} else {
		printDrivesResponseWithMetrics(drives)
	}

	// Example 5: Query with last minute metrics
	fmt.Println("\n=== DrivesQuery with last minute metrics ===")
	drives, err = madmClnt.DrivesQuery(context.Background(), &madmin.DrivesResourceOpts{
		Metrics:    true,
		LastMinute: true,
		Limit:      5,
	})
	if err != nil {
		log.Println("Error querying drives with last minute metrics:", err)
	} else {
		printDrivesResponseWithMetrics(drives)
	}

	// Example 6: Query with last day metrics
	fmt.Println("\n=== DrivesQuery with last day metrics ===")
	drives, err = madmClnt.DrivesQuery(context.Background(), &madmin.DrivesResourceOpts{
		Metrics: true,
		LastDay: true,
		Limit:   5,
	})
	if err != nil {
		log.Println("Error querying drives with last day metrics:", err)
	} else {
		printDrivesResponseWithMetrics(drives)
	}

	// Example 7: Paginating through all drives
	fmt.Println("\n=== Paginating through all drives ===")
	offset := 0
	pageSize := 10
	pageNum := 1

	for {
		drives, err = madmClnt.DrivesQuery(context.Background(), &madmin.DrivesResourceOpts{
			Limit:  pageSize,
			Offset: offset,
		})
		if err != nil {
			log.Printf("Error querying drives at page %d: %v", pageNum, err)
			break
		}

		fmt.Printf("Page %d (showing %d of %d total drives):\n", pageNum, drives.Count, drives.Total)
		for i, drive := range drives.Results {
			fmt.Printf("  %d. Drive ID: %s, Path: %s, State: %s, Node: %s\n",
				i+1, drive.ID, drive.Path, drive.State, drive.NodeID)
		}

		// Check if we've reached the end
		if offset+drives.Count >= drives.Total {
			break
		}

		offset += pageSize
		pageNum++
	}
}

func printDrivesResponse(resp *madmin.PaginatedDrivesResponse) {
	fmt.Printf("Total drives: %d, Retrieved: %d, Offset: %d\n",
		resp.Total, resp.Count, resp.Offset)

	// Print aggregated metrics if available
	if resp.Aggregated.NDisks > 0 {
		fmt.Println("Aggregated metrics:")
		fmt.Printf("  Number of Disks: %d\n", resp.Aggregated.NDisks)
		if resp.Aggregated.Space.N > 0 {
			fmt.Printf("  Space - Free: %d, Used: %d\n",
				resp.Aggregated.Space.Free.Total, resp.Aggregated.Space.Used.Total)
		}
		if resp.Aggregated.Offline > 0 {
			fmt.Printf("  Offline Disks: %d\n", resp.Aggregated.Offline)
		}
		if resp.Aggregated.Healing > 0 {
			fmt.Printf("  Healing Disks: %d\n", resp.Aggregated.Healing)
		}
	}

	// Print individual drives
	for i, drive := range resp.Results {
		fmt.Printf("%d. Drive Details:\n", i+1)
		fmt.Printf("   ID: %s\n", drive.ID)
		fmt.Printf("   Path: %s\n", drive.Path)
		fmt.Printf("   Node ID: %s\n", drive.NodeID)
		fmt.Printf("   State: %s\n", drive.State)
		fmt.Printf("   Pool Index: %d, Set Index: %d, Drive Index: %d\n",
			drive.PoolIndex, drive.SetIndex, drive.DriveIndex)
		fmt.Printf("   Size: %d, Used: %d, Available: %d\n",
			drive.Size, drive.Used, drive.Available)
		fmt.Printf("   Inodes - Free: %d, Used: %d\n",
			drive.InodesFree, drive.InodesUsed)
		fmt.Printf("   UUID: %s\n", drive.UUID)
		fmt.Printf("   Healing: %v\n", drive.Healing)
	}
}

func printDrivesResponseWithMetrics(resp *madmin.PaginatedDrivesResponse) {
	fmt.Printf("Total drives: %d, Retrieved: %d, Offset: %d\n",
		resp.Total, resp.Count, resp.Offset)

	// Print aggregated metrics
	fmt.Println("Aggregated metrics:")
	printMetrics(&resp.Aggregated, "  ")

	// Print individual drives with metrics
	for i, drive := range resp.Results {
		fmt.Printf("%d. Drive: %s (Path: %s, Node: %s)\n",
			i+1, drive.ID, drive.Path, drive.NodeID)

		if drive.Metrics != nil {
			fmt.Println("   Metrics:")
			printMetrics(drive.Metrics, "     ")
		}
	}
}

func printMetrics(m *madmin.DiskMetric, indent string) {
	if m == nil {
		return
	}

	// Print as formatted JSON for better readability
	b, err := json.MarshalIndent(m, indent, "  ")
	if err != nil {
		fmt.Printf("%sError marshaling metrics: %v\n", indent, err)
		return
	}
	fmt.Println(string(b))
}
