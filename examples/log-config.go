//go:build ignore
// +build ignore

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
	
	fmt.Printf("  Error Logs:\n")
	fmt.Printf("    Enabled: %v\n", logStatus.Error.Enabled)
	if logStatus.Error.DriveLimit != "" {
		fmt.Printf("    Drive Limit: %s\n", logStatus.Error.DriveLimit)
	}
	
	fmt.Printf("  Audit Logs:\n")
	fmt.Printf("    Enabled: %v\n", logStatus.Audit.Enabled)
	fmt.Println()

	// Example 2: Update log configuration
	fmt.Println("Updating log configuration...")
	
	// Enable API logs with 2GiB drive limit
	enableAPI := true
	apiDriveLimit := "2Gi"
	
	// Disable error logs
	disableError := false
	
	// Enable audit logs (no drive limit for audit logs as they use object storage)
	enableAudit := true
	
	newConfig := &madmin.LogRecorderConfig{
		API: &madmin.LogRecorderAPIConfig{
			Enable:     &enableAPI,
			DriveLimit: &apiDriveLimit,
		},
		Error: &madmin.LogRecorderErrorConfig{
			Enable: &disableError,
		},
		Audit: &madmin.LogRecorderAuditConfig{
			Enable: &enableAudit,
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
	
	fmt.Printf("  Error Logs:\n")
	fmt.Printf("    Enabled: %v\n", updatedStatus.Error.Enabled)
	if updatedStatus.Error.DriveLimit != "" {
		fmt.Printf("    Drive Limit: %s\n", updatedStatus.Error.DriveLimit)
	}
	
	fmt.Printf("  Audit Logs:\n")
	fmt.Printf("    Enabled: %v\n", updatedStatus.Audit.Enabled)
	fmt.Println()

	// Example 4: Partially update configuration (only modify specific fields)
	fmt.Println("Partially updating log configuration (only Error logs)...")
	
	// Only update error logs settings
	enableErrorAgain := true
	errorDriveLimit := "500Mi"
	
	partialConfig := &madmin.LogRecorderConfig{
		Error: &madmin.LogRecorderErrorConfig{
			Enable:     &enableErrorAgain,
			DriveLimit: &errorDriveLimit,
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
	
	fmt.Printf("  Error Logs:\n")
	fmt.Printf("    Enabled: %v\n", resetStatus.Error.Enabled)
	if resetStatus.Error.DriveLimit != "" {
		fmt.Printf("    Drive Limit: %s (default)\n", resetStatus.Error.DriveLimit)
	}
	
	fmt.Printf("  Audit Logs:\n")
	fmt.Printf("    Enabled: %v\n", resetStatus.Audit.Enabled)
}