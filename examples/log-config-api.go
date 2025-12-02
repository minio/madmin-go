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

	// Get current API log configuration
	cfg, err := madmClnt.GetAPILogConfig(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	// Print configuration as YAML with comments
	fmt.Println("Current API Log Configuration:")
	fmt.Println(cfg.YAML())

	// Modify internal recorder settings
	cfg.Internal.Enable.Value = "on"
	cfg.Internal.DriveLimit.Value = "2Gi"
	cfg.Internal.FlushCount.Value = "100"
	cfg.Internal.FlushInterval.Value = "1m"

	// Add a webhook target
	cfg.Webhook = append(cfg.Webhook, madmin.APIWebhookConfig{
		WebhookConfig: madmin.WebhookConfig{
			Name:          madmin.LogField{Value: "mywebhook"},
			Enable:        madmin.LogField{Value: "on"},
			Endpoint:      madmin.LogField{Value: "http://localhost:8080/logs"},
			QueueSize:     madmin.LogField{Value: "10000"},
			TLSSkipVerify: madmin.LogField{Value: "off"},
		},
		FlushCount:    madmin.LogField{Value: "100"},
		FlushInterval: madmin.LogField{Value: "1m"},
	})

	// Set the updated configuration
	if err := madmClnt.SetAPILogConfig(ctx, cfg); err != nil {
		log.Fatalln(err)
	}
	fmt.Println("API log configuration updated successfully")

	// Refetch the configuration to get descriptions for all targets
	cfg, err = madmClnt.GetAPILogConfig(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	// Print configuration as YAML with comments
	fmt.Println("Updated API Log Configuration:")
	fmt.Println(cfg.YAML())

	// Reset to defaults
	if err := madmClnt.ResetAPILogConfig(ctx); err != nil {
		log.Fatalln(err)
	}
	fmt.Println("API log configuration reset to defaults")
}
