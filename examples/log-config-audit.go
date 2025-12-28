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

	// Get current Audit log configuration
	cfg, err := madmClnt.GetAuditLogConfig(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	// Print configuration as YAML with comments
	fmt.Println("Current Audit Log Configuration:")
	fmt.Println(cfg.YAML())

	// Modify internal recorder settings (audit only has enable)
	cfg.Internal.Enable.Value = "on"

	// Add a webhook target for audit logs (no batching for audit)
	cfg.Webhook = append(cfg.Webhook, madmin.AuditWebhookConfig{
		WebhookConfig: madmin.WebhookConfig{
			Name:          madmin.LogField{Value: "audit-webhook"},
			Enable:        madmin.LogField{Value: "on"},
			Endpoint:      madmin.LogField{Value: "http://localhost:8080/audit-logs"},
			QueueSize:     madmin.LogField{Value: "10000"},
			TLSSkipVerify: madmin.LogField{Value: "off"},
		},
	})

	// Add a Kafka target for audit logs
	cfg.Kafka = append(cfg.Kafka, madmin.AuditKafkaConfig{
		KafkaConfig: madmin.KafkaConfig{
			Name:    madmin.LogField{Value: "audit-kafka"},
			Enable:  madmin.LogField{Value: "on"},
			Brokers: madmin.LogField{Value: "localhost:9092"},
			Topic:   madmin.LogField{Value: "minio-audit"},
			TLS: madmin.KafkaTLSConfig{
				Enable:     madmin.LogField{Value: "off"},
				SkipVerify: madmin.LogField{Value: "off"},
			},
			SASL: madmin.KafkaSASLConfig{
				Enable: madmin.LogField{Value: "off"},
			},
		},
	})

	// Set the updated configuration
	if err := madmClnt.SetAuditLogConfig(ctx, cfg); err != nil {
		log.Fatalln(err)
	}
	fmt.Println("Audit log configuration updated successfully")

	// Reset to defaults
	if err := madmClnt.ResetAuditLogConfig(ctx); err != nil {
		log.Fatalln(err)
	}
	fmt.Println("Audit log configuration reset to defaults")
}
