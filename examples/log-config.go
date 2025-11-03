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
	"fmt"
	"log"
	"time"

	"github.com/minio/madmin-go/v4"
)

func main() {
	// Initialize madmin client
	mdmClnt, err := madmin.NewWithOptions("localhost:9000", &madmin.Options{
		Creds:  madmin.NewStaticCredentials("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	if err != nil {
		log.Fatalln(err)
	}

	ctx := context.Background()

	// Example 1: Get current log configuration
	fmt.Println("Getting current log configuration...")
	logStatus, err := mdmClnt.GetLogConfig(ctx)
	if err != nil {
		log.Fatalln("Error getting log config:", err)
	}

	fmt.Printf("Current Log Configuration:\n")
	fmt.Printf("  API Logs:\n")
	fmt.Printf("    Enabled: %v\n", logStatus.API.Enabled)
	if logStatus.API.DriveLimit != "" {
		fmt.Printf("    Drive Limit: %s\n", logStatus.API.DriveLimit)
	}
	if logStatus.API.FlushCount > 0 {
		fmt.Printf("    Flush Count: %d\n", logStatus.API.FlushCount)
	}
	if logStatus.API.FlushInterval > 0 {
		fmt.Printf("    Flush Interval: %v\n", logStatus.API.FlushInterval)
	}

	fmt.Printf("  Error Logs:\n")
	fmt.Printf("    Enabled: %v\n", logStatus.Error.Enabled)
	if logStatus.Error.DriveLimit != "" {
		fmt.Printf("    Drive Limit: %s\n", logStatus.Error.DriveLimit)
	}
	if logStatus.Error.FlushCount > 0 {
		fmt.Printf("    Flush Count: %d\n", logStatus.Error.FlushCount)
	}
	if logStatus.Error.FlushInterval > 0 {
		fmt.Printf("    Flush Interval: %v\n", logStatus.Error.FlushInterval)
	}

	fmt.Printf("  Audit Logs:\n")
	fmt.Printf("    Enabled: %v\n", logStatus.Audit.Enabled)
	if logStatus.Audit.FlushCount > 0 {
		fmt.Printf("    Flush Count: %d\n", logStatus.Audit.FlushCount)
	}
	if logStatus.Audit.FlushInterval > 0 {
		fmt.Printf("    Flush Interval: %v\n", logStatus.Audit.FlushInterval)
	}
	fmt.Println()

	// Example 2: Update log configuration
	fmt.Println("Updating log configuration...")

	// Enable API logs with 2GiB drive limit, flush count of 1000, and 5 minute flush interval
	enableAPI := true
	apiDriveLimit := "2Gi"
	apiFlushCount := 1000
	apiFlushInterval := 5 * time.Minute

	// Disable error logs
	disableError := false

	// Enable audit logs with flush count of 500 and 2 minute flush interval
	enableAudit := true
	auditFlushCount := 500
	auditFlushInterval := 2 * time.Minute

	newConfig := &madmin.LogRecorderConfig{
		API: &madmin.LogRecorderAPIConfig{
			Enable:        &enableAPI,
			DriveLimit:    &apiDriveLimit,
			FlushCount:    &apiFlushCount,
			FlushInterval: &apiFlushInterval,
		},
		Error: &madmin.LogRecorderErrorConfig{
			Enable: &disableError,
		},
		Audit: &madmin.LogRecorderAuditConfig{
			Enable:        &enableAudit,
			FlushCount:    &auditFlushCount,
			FlushInterval: &auditFlushInterval,
		},
	}

	err = mdmClnt.SetLogConfig(ctx, newConfig)
	if err != nil {
		log.Fatalln("Error setting log config:", err)
	}
	fmt.Println("Log configuration updated successfully")
	fmt.Println()

	// Example 3: Verify the updated configuration
	fmt.Println("Verifying updated configuration...")
	updatedStatus, err := mdmClnt.GetLogConfig(ctx)
	if err != nil {
		log.Fatalln("Error getting updated log config:", err)
	}

	fmt.Printf("Updated Log Configuration:\n")
	fmt.Printf("  API Logs:\n")
	fmt.Printf("    Enabled: %v\n", updatedStatus.API.Enabled)
	if updatedStatus.API.DriveLimit != "" {
		fmt.Printf("    Drive Limit: %s\n", updatedStatus.API.DriveLimit)
	}
	if updatedStatus.API.FlushCount > 0 {
		fmt.Printf("    Flush Count: %d\n", updatedStatus.API.FlushCount)
	}
	if updatedStatus.API.FlushInterval > 0 {
		fmt.Printf("    Flush Interval: %v\n", updatedStatus.API.FlushInterval)
	}

	fmt.Printf("  Error Logs:\n")
	fmt.Printf("    Enabled: %v\n", updatedStatus.Error.Enabled)
	if updatedStatus.Error.DriveLimit != "" {
		fmt.Printf("    Drive Limit: %s\n", updatedStatus.Error.DriveLimit)
	}
	if updatedStatus.Error.FlushCount > 0 {
		fmt.Printf("    Flush Count: %d\n", updatedStatus.Error.FlushCount)
	}
	if updatedStatus.Error.FlushInterval > 0 {
		fmt.Printf("    Flush Interval: %v\n", updatedStatus.Error.FlushInterval)
	}

	fmt.Printf("  Audit Logs:\n")
	fmt.Printf("    Enabled: %v\n", updatedStatus.Audit.Enabled)
	if updatedStatus.Audit.FlushCount > 0 {
		fmt.Printf("    Flush Count: %d\n", updatedStatus.Audit.FlushCount)
	}
	if updatedStatus.Audit.FlushInterval > 0 {
		fmt.Printf("    Flush Interval: %v\n", updatedStatus.Audit.FlushInterval)
	}
	fmt.Println()

	// Example 4: Partially update configuration (only modify specific fields)
	fmt.Println("Partially updating log configuration (only Error logs)...")

	// Only update error logs settings
	enableErrorAgain := true
	errorDriveLimit := "500Mi"
	errorFlushCount := 750
	errorFlushInterval := 3 * time.Minute

	partialConfig := &madmin.LogRecorderConfig{
		Error: &madmin.LogRecorderErrorConfig{
			Enable:        &enableErrorAgain,
			DriveLimit:    &errorDriveLimit,
			FlushCount:    &errorFlushCount,
			FlushInterval: &errorFlushInterval,
		},
		// API and Audit fields are nil, so they won't be modified
	}

	err = mdmClnt.SetLogConfig(ctx, partialConfig)
	if err != nil {
		log.Fatalln("Error setting partial log config:", err)
	}
	fmt.Println("Partial log configuration updated successfully")
	fmt.Println()

	// Example 5: Reset to default configuration
	fmt.Println("Resetting log configuration to defaults...")
	err = mdmClnt.ResetLogConfig(ctx)
	if err != nil {
		log.Fatalln("Error resetting log config:", err)
	}
	fmt.Println("Log configuration reset to defaults successfully")
	fmt.Println()

	// Verify reset
	fmt.Println("Verifying reset configuration...")
	resetStatus, err := mdmClnt.GetLogConfig(ctx)
	if err != nil {
		log.Fatalln("Error getting reset log config:", err)
	}

	fmt.Printf("Reset Log Configuration (Defaults):\n")
	fmt.Printf("  API Logs:\n")
	fmt.Printf("    Enabled: %v\n", resetStatus.API.Enabled)
	if resetStatus.API.DriveLimit != "" {
		fmt.Printf("    Drive Limit: %s (default)\n", resetStatus.API.DriveLimit)
	}
	if resetStatus.API.FlushCount > 0 {
		fmt.Printf("    Flush Count: %d (default)\n", resetStatus.API.FlushCount)
	}
	if resetStatus.API.FlushInterval > 0 {
		fmt.Printf("    Flush Interval: %v (default)\n", resetStatus.API.FlushInterval)
	}

	fmt.Printf("  Error Logs:\n")
	fmt.Printf("    Enabled: %v\n", resetStatus.Error.Enabled)
	if resetStatus.Error.DriveLimit != "" {
		fmt.Printf("    Drive Limit: %s (default)\n", resetStatus.Error.DriveLimit)
	}
	if resetStatus.Error.FlushCount > 0 {
		fmt.Printf("    Flush Count: %d (default)\n", resetStatus.Error.FlushCount)
	}
	if resetStatus.Error.FlushInterval > 0 {
		fmt.Printf("    Flush Interval: %v (default)\n", resetStatus.Error.FlushInterval)
	}

	fmt.Printf("  Audit Logs:\n")
	fmt.Printf("    Enabled: %v\n", resetStatus.Audit.Enabled)
	if resetStatus.Audit.FlushCount > 0 {
		fmt.Printf("    Flush Count: %d (default)\n", resetStatus.Audit.FlushCount)
	}
	if resetStatus.Audit.FlushInterval > 0 {
		fmt.Printf("    Flush Interval: %v (default)\n", resetStatus.Audit.FlushInterval)
	}
}
