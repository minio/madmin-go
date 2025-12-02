//go:build ignore

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
	// Note: YOUR-ACCESSKEYID, YOUR-SECRETACCESSKEY are
	// dummy values, please replace them with original values.

	// API requests are secure (HTTPS) if secure=true and insecure (HTTP) otherwise.
	// New returns an MinIO Admin client object.
	madmClnt, err := madmin.New("your-minio.example.com:9000", "YOUR-ACCESSKEYID", "YOUR-SECRETACCESSKEY", true)
	if err != nil {
		log.Fatalln(err)
	}

	ctx := context.Background()

	// Get current Error log configuration
	cfg, err := madmClnt.GetErrorLogConfig(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	// Print configuration as YAML with comments
	fmt.Println("Current Error Log Configuration:")
	fmt.Println(cfg.YAML())

	// Modify internal recorder settings
	cfg.Internal.Enable.Value = "on"
	cfg.Internal.DriveLimit.Value = "1Gi"
	cfg.Internal.FlushCount.Value = "50"
	cfg.Internal.FlushInterval.Value = "30s"

	// Add a webhook target for error logs
	cfg.Webhook = append(cfg.Webhook, madmin.ErrorWebhookConfig{
		WebhookConfig: madmin.WebhookConfig{
			Name:          madmin.LogField{Value: "error-webhook"},
			Enable:        madmin.LogField{Value: "on"},
			Endpoint:      madmin.LogField{Value: "http://localhost:8080/error-logs"},
			QueueSize:     madmin.LogField{Value: "5000"},
			TLSSkipVerify: madmin.LogField{Value: "off"},
		},
		FlushCount:    madmin.LogField{Value: "50"},
		FlushInterval: madmin.LogField{Value: "30s"},
	})

	// Set the updated configuration
	if err := madmClnt.SetErrorLogConfig(ctx, cfg); err != nil {
		log.Fatalln(err)
	}
	fmt.Println("Error log configuration updated successfully")

	// Reset to defaults
	if err := madmClnt.ResetErrorLogConfig(ctx); err != nil {
		log.Fatalln(err)
	}
	fmt.Println("Error log configuration reset to defaults")
}
