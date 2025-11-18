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
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	// Initialize madmin client
	mdmClnt, err := madmin.NewWithOptions("localhost:9000", &madmin.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	if err != nil {
		log.Fatalln(err)
	}

	ctx := context.Background()

	// Example 1: Get current log configuration
	fmt.Println("Getting current log configuration...")

	apiStatus, err := mdmClnt.GetAPILogConfig(ctx)
	if err != nil {
		log.Fatalln("Error getting API log config:", err)
	}

	errorStatus, err := mdmClnt.GetErrorLogConfig(ctx)
	if err != nil {
		log.Fatalln("Error getting Error log config:", err)
	}

	auditStatus, err := mdmClnt.GetAuditLogConfig(ctx)
	if err != nil {
		log.Fatalln("Error getting Audit log config:", err)
	}

	fmt.Printf("Current Log Configuration:\n")
	fmt.Printf("  API Logs:\n")
	fmt.Printf("    Enabled: %v\n", apiStatus.Enabled)
	if apiStatus.DriveLimit != "" {
		fmt.Printf("    Drive Limit: %s\n", apiStatus.DriveLimit)
	}
	if apiStatus.FlushCount > 0 {
		fmt.Printf("    Flush Count: %d\n", apiStatus.FlushCount)
	}
	if apiStatus.FlushInterval > 0 {
		fmt.Printf("    Flush Interval: %v\n", apiStatus.FlushInterval)
	}

	fmt.Printf("  Error Logs:\n")
	fmt.Printf("    Enabled: %v\n", errorStatus.Enabled)
	if errorStatus.DriveLimit != "" {
		fmt.Printf("    Drive Limit: %s\n", errorStatus.DriveLimit)
	}
	if errorStatus.FlushCount > 0 {
		fmt.Printf("    Flush Count: %d\n", errorStatus.FlushCount)
	}
	if errorStatus.FlushInterval > 0 {
		fmt.Printf("    Flush Interval: %v\n", errorStatus.FlushInterval)
	}

	fmt.Printf("  Audit Logs:\n")
	fmt.Printf("    Enabled: %v\n", auditStatus.Enabled)
	if auditStatus.FlushCount > 0 {
		fmt.Printf("    Flush Count: %d\n", auditStatus.FlushCount)
	}
	if auditStatus.FlushInterval > 0 {
		fmt.Printf("    Flush Interval: %v\n", auditStatus.FlushInterval)
	}
	fmt.Println()

	// Example 2: Update log configuration
	fmt.Println("Updating log configuration...")

	// Enable API logs with 2GiB drive limit, flush count of 1000, and 5 minute flush interval
	enableAPI := true
	apiDriveLimit := "2Gi"
	apiFlushCount := 1000
	apiFlushInterval := 5 * time.Minute

	apiConfig := &madmin.LogRecorderAPIConfig{
		Enable:        &enableAPI,
		DriveLimit:    &apiDriveLimit,
		FlushCount:    &apiFlushCount,
		FlushInterval: &apiFlushInterval,
	}

	err = mdmClnt.SetAPILogConfig(ctx, apiConfig)
	if err != nil {
		log.Fatalln("Error setting API log config:", err)
	}

	// Disable error logs
	disableError := false
	errorConfig := &madmin.LogRecorderErrorConfig{
		Enable: &disableError,
	}

	err = mdmClnt.SetErrorLogConfig(ctx, errorConfig)
	if err != nil {
		log.Fatalln("Error setting Error log config:", err)
	}

	// Enable audit logs with flush count of 500 and 2 minute flush interval
	enableAudit := true
	auditFlushCount := 500
	auditFlushInterval := 2 * time.Minute

	auditConfig := &madmin.LogRecorderAuditConfig{
		Enable:        &enableAudit,
		FlushCount:    &auditFlushCount,
		FlushInterval: &auditFlushInterval,
	}

	err = mdmClnt.SetAuditLogConfig(ctx, auditConfig)
	if err != nil {
		log.Fatalln("Error setting Audit log config:", err)
	}

	fmt.Println("Log configuration updated successfully")
	fmt.Println()

	// Example 3: Verify the updated configuration
	fmt.Println("Verifying updated configuration...")

	updatedAPIStatus, err := mdmClnt.GetAPILogConfig(ctx)
	if err != nil {
		log.Fatalln("Error getting updated API log config:", err)
	}

	updatedErrorStatus, err := mdmClnt.GetErrorLogConfig(ctx)
	if err != nil {
		log.Fatalln("Error getting updated Error log config:", err)
	}

	updatedAuditStatus, err := mdmClnt.GetAuditLogConfig(ctx)
	if err != nil {
		log.Fatalln("Error getting updated Audit log config:", err)
	}

	fmt.Printf("Updated Log Configuration:\n")
	fmt.Printf("  API Logs:\n")
	fmt.Printf("    Enabled: %v\n", updatedAPIStatus.Enabled)
	if updatedAPIStatus.DriveLimit != "" {
		fmt.Printf("    Drive Limit: %s\n", updatedAPIStatus.DriveLimit)
	}
	if updatedAPIStatus.FlushCount > 0 {
		fmt.Printf("    Flush Count: %d\n", updatedAPIStatus.FlushCount)
	}
	if updatedAPIStatus.FlushInterval > 0 {
		fmt.Printf("    Flush Interval: %v\n", updatedAPIStatus.FlushInterval)
	}

	fmt.Printf("  Error Logs:\n")
	fmt.Printf("    Enabled: %v\n", updatedErrorStatus.Enabled)
	if updatedErrorStatus.DriveLimit != "" {
		fmt.Printf("    Drive Limit: %s\n", updatedErrorStatus.DriveLimit)
	}
	if updatedErrorStatus.FlushCount > 0 {
		fmt.Printf("    Flush Count: %d\n", updatedErrorStatus.FlushCount)
	}
	if updatedErrorStatus.FlushInterval > 0 {
		fmt.Printf("    Flush Interval: %v\n", updatedErrorStatus.FlushInterval)
	}

	fmt.Printf("  Audit Logs:\n")
	fmt.Printf("    Enabled: %v\n", updatedAuditStatus.Enabled)
	if updatedAuditStatus.FlushCount > 0 {
		fmt.Printf("    Flush Count: %d\n", updatedAuditStatus.FlushCount)
	}
	if updatedAuditStatus.FlushInterval > 0 {
		fmt.Printf("    Flush Interval: %v\n", updatedAuditStatus.FlushInterval)
	}
	fmt.Println()

	// Example 4: Partially update configuration (only modify specific fields)
	fmt.Println("Partially updating log configuration (only Error logs)...")

	// Only update error logs settings
	enableErrorAgain := true
	errorDriveLimit := "500Mi"
	errorFlushCount := 750
	errorFlushInterval := 3 * time.Minute

	partialErrorConfig := &madmin.LogRecorderErrorConfig{
		Enable:        &enableErrorAgain,
		DriveLimit:    &errorDriveLimit,
		FlushCount:    &errorFlushCount,
		FlushInterval: &errorFlushInterval,
	}

	err = mdmClnt.SetErrorLogConfig(ctx, partialErrorConfig)
	if err != nil {
		log.Fatalln("Error setting partial log config:", err)
	}
	fmt.Println("Partial log configuration updated successfully")
	fmt.Println()

	// Example 5: Reset to default configuration
	fmt.Println("Resetting log configuration to defaults...")

	err = mdmClnt.ResetAPILogConfig(ctx)
	if err != nil {
		log.Fatalln("Error resetting API log config:", err)
	}

	err = mdmClnt.ResetErrorLogConfig(ctx)
	if err != nil {
		log.Fatalln("Error resetting Error log config:", err)
	}

	err = mdmClnt.ResetAuditLogConfig(ctx)
	if err != nil {
		log.Fatalln("Error resetting Audit log config:", err)
	}

	fmt.Println("Log configuration reset to defaults successfully")
	fmt.Println()

	// Verify reset
	fmt.Println("Verifying reset configuration...")

	resetAPIStatus, err := mdmClnt.GetAPILogConfig(ctx)
	if err != nil {
		log.Fatalln("Error getting reset API log config:", err)
	}

	resetErrorStatus, err := mdmClnt.GetErrorLogConfig(ctx)
	if err != nil {
		log.Fatalln("Error getting reset Error log config:", err)
	}

	resetAuditStatus, err := mdmClnt.GetAuditLogConfig(ctx)
	if err != nil {
		log.Fatalln("Error getting reset Audit log config:", err)
	}

	fmt.Printf("Reset Log Configuration (Defaults):\n")
	fmt.Printf("  API Logs:\n")
	fmt.Printf("    Enabled: %v\n", resetAPIStatus.Enabled)
	if resetAPIStatus.DriveLimit != "" {
		fmt.Printf("    Drive Limit: %s (default)\n", resetAPIStatus.DriveLimit)
	}
	if resetAPIStatus.FlushCount > 0 {
		fmt.Printf("    Flush Count: %d (default)\n", resetAPIStatus.FlushCount)
	}
	if resetAPIStatus.FlushInterval > 0 {
		fmt.Printf("    Flush Interval: %v (default)\n", resetAPIStatus.FlushInterval)
	}

	fmt.Printf("  Error Logs:\n")
	fmt.Printf("    Enabled: %v\n", resetErrorStatus.Enabled)
	if resetErrorStatus.DriveLimit != "" {
		fmt.Printf("    Drive Limit: %s (default)\n", resetErrorStatus.DriveLimit)
	}
	if resetErrorStatus.FlushCount > 0 {
		fmt.Printf("    Flush Count: %d (default)\n", resetErrorStatus.FlushCount)
	}
	if resetErrorStatus.FlushInterval > 0 {
		fmt.Printf("    Flush Interval: %v (default)\n", resetErrorStatus.FlushInterval)
	}

	fmt.Printf("  Audit Logs:\n")
	fmt.Printf("    Enabled: %v\n", resetAuditStatus.Enabled)
	if resetAuditStatus.FlushCount > 0 {
		fmt.Printf("    Flush Count: %d (default)\n", resetAuditStatus.FlushCount)
	}
	if resetAuditStatus.FlushInterval > 0 {
		fmt.Printf("    Flush Interval: %v (default)\n", resetAuditStatus.FlushInterval)
	}
}
