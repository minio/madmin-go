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
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/minio/madmin-go/v4"
)

func main() {
	// Initialize admin client
	endpoint := getEnvOrDefault("MINIO_ENDPOINT", "localhost:9000")
	accessKey := getEnvOrDefault("MINIO_ACCESS_KEY", "minioadmin")
	secretKey := getEnvOrDefault("MINIO_SECRET_KEY", "minioadmin")
	useSSL := getEnvOrDefault("MINIO_USE_SSL", "false") == "true"

	if accessKey == "" || secretKey == "" {
		log.Fatal("Please set MINIO_ACCESS_KEY and MINIO_SECRET_KEY environment variables")
	}

	// Create admin client
	mdmClient, err := madmin.NewWithOptions(endpoint, &madmin.Options{
		Creds:  madmin.NewStaticCredentials(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		log.Fatalln(err)
	}

	ctx := context.Background()

	// Example 1: Create a Delta Sharing share with Delta tables
	fmt.Println("Creating Delta Sharing share...")
	createShareReq := madmin.CreateShareRequest{
		Name:        "analytics-share",
		Description: "Analytics data share for data team",
		Schemas: []madmin.DeltaSharingSchema{
			madmin.NewSchema("default", "Default schema for analytics",
				madmin.NewDeltaTable("sales", "data-lake", "sales/"),
				madmin.NewDeltaTable("customers", "data-lake", "customers/"),
			),
		},
	}

	shareResp, err := mdmClient.CreateShare(ctx, createShareReq)
	if err != nil {
		log.Printf("Failed to create share: %v", err)
	} else {
		fmt.Printf("Created share: %s (ID: %s)\n", shareResp.Share.Name, shareResp.Share.ID)
	}

	// Example 2: Create a share with Iceberg UniForm tables
	fmt.Println("\nCreating Delta Sharing share with UniForm tables...")
	uniformShareReq := madmin.CreateShareRequest{
		Name:        "iceberg-share",
		Description: "Iceberg tables exposed via UniForm",
		Schemas: []madmin.DeltaSharingSchema{
			madmin.NewSchema("warehouse", "Warehouse data",
				madmin.NewUniformTable("inventory", "warehouse1", "retail", "inventory_table"),
				madmin.NewUniformTable("orders", "warehouse1", "retail", "orders_table"),
			),
		},
	}

	uniformShareResp, err := mdmClient.CreateShare(ctx, uniformShareReq)
	if err != nil {
		log.Printf("Failed to create UniForm share: %v", err)
	} else {
		fmt.Printf("Created UniForm share: %s\n", uniformShareResp.Share.Name)
	}

	// Example 3: List all shares
	fmt.Println("\nListing all shares...")
	sharesResp, err := mdmClient.ListShares(ctx)
	if err != nil {
		log.Printf("Failed to list shares: %v", err)
	} else {
		for _, share := range sharesResp.Shares {
			fmt.Printf("- Share: %s\n", share.Name)
			fmt.Printf("  Description: %s\n", share.Description)
			fmt.Printf("  Schemas: %d\n", len(share.Schemas))
			for _, schema := range share.Schemas {
				fmt.Printf("    - Schema: %s (tables: %d)\n", schema.Name, len(schema.Tables))
			}
		}
	}

	// Example 4: Create access token for a share
	fmt.Println("\nCreating access token...")

	// Token expires in 30 days
	expiresAt := time.Now().Add(30 * 24 * time.Hour)
	tokenReq := madmin.CreateTokenRequest{
		Description: "Token for Databricks workspace",
		ExpiresAt:   &expiresAt,
	}

	tokenResp, err := mdmClient.CreateToken(ctx, "analytics-share", tokenReq)
	if err != nil {
		log.Printf("Failed to create token: %v", err)
	} else {
		fmt.Printf("Created token ID: %s\n", tokenResp.TokenID)
		fmt.Printf("Token (first 20 chars): %s...\n", tokenResp.Token[:20])

		// Save the profile to a file
		profilePath := "analytics-share-profile.json"
		profileData, _ := json.MarshalIndent(tokenResp.Profile, "", "  ")
		if err := os.WriteFile(profilePath, profileData, 0o600); err != nil {
			log.Printf("Failed to save profile: %v", err)
		} else {
			fmt.Printf("Saved profile to: %s\n", profilePath)

			// Show usage instructions
			fmt.Println("\n=== Databricks Usage ===")
			fmt.Printf("1. Upload '%s' to Databricks DBFS\n", profilePath)
			fmt.Println("2. In your Databricks notebook:")
			fmt.Println("   profile_path = '/dbfs/FileStore/delta-sharing/analytics-share-profile.json'")
			fmt.Println("   df = spark.read.format('deltaSharing').load(profile_path + '#analytics-share.default.sales')")
			fmt.Println("   df.show()")
		}
	}

	// Example 5: List tokens for a share
	fmt.Println("\nListing tokens for analytics-share...")
	tokensResp, err := mdmClient.ListTokens(ctx, "analytics-share")
	if err != nil {
		log.Printf("Failed to list tokens: %v", err)
	} else {
		for _, token := range tokensResp.Tokens {
			fmt.Printf("- Token ID: %s\n", token.TokenID)
			fmt.Printf("  Description: %s\n", token.Description)
			fmt.Printf("  Created: %s\n", token.CreatedAt.Format(time.RFC3339))
			if token.ExpiresAt != nil {
				fmt.Printf("  Expires: %s\n", token.ExpiresAt.Format(time.RFC3339))
			} else {
				fmt.Println("  Expires: Never")
			}
		}
	}

	// Example 6: Get share details
	fmt.Println("\nGetting share details...")
	share, err := mdmClient.GetShare(ctx, "analytics-share")
	if err != nil {
		log.Printf("Failed to get share: %v", err)
	} else {
		fmt.Printf("Share: %s\n", share.Name)
		fmt.Printf("ID: %s\n", share.ID)
		fmt.Printf("Created: %s\n", share.CreatedAt.Format(time.RFC3339))
		fmt.Printf("Schemas:\n")
		for _, schema := range share.Schemas {
			fmt.Printf("  - %s: %d tables\n", schema.Name, len(schema.Tables))
			for _, table := range schema.Tables {
				if table.SourceType == "delta" {
					fmt.Printf("    - %s (Delta): s3://%s/%s\n", table.Name, table.Bucket, table.Location)
				} else {
					fmt.Printf("    - %s (UniForm): %s.%s.%s\n", table.Name, table.Warehouse, table.Namespace, table.Table)
				}
			}
		}
	}

	// Example 7: Update share description
	fmt.Println("\nUpdating share description...")
	newDescription := "Updated analytics data share with new tables"
	updatedShare, err := mdmClient.UpdateShare(ctx, "analytics-share", &newDescription, nil)
	if err != nil {
		log.Printf("Failed to update share: %v", err)
	} else {
		fmt.Printf("Updated share: %s\n", updatedShare.Name)
		fmt.Printf("New description: %s\n", updatedShare.Description)
	}

	// Example 8: Clean up (uncomment to delete resources)
	// fmt.Println("\nCleaning up...")
	//
	// // Delete a specific token
	// if tokenResp != nil {
	//     err = mdmClient.DeleteToken(ctx, tokenResp.TokenID)
	//     if err != nil {
	//         log.Printf("Failed to delete token: %v", err)
	//     } else {
	//         fmt.Printf("Deleted token: %s\n", tokenResp.TokenID)
	//     }
	// }
	//
	// // Delete shares
	// err = mdmClient.DeleteShare(ctx, "analytics-share")
	// if err != nil {
	//     log.Printf("Failed to delete share: %v", err)
	// } else {
	//     fmt.Println("Deleted share: analytics-share")
	// }
	//
	// err = mdmClient.DeleteShare(ctx, "iceberg-share")
	// if err != nil {
	//     log.Printf("Failed to delete share: %v", err)
	// } else {
	//     fmt.Println("Deleted share: iceberg-share")
	// }

	fmt.Println("\nDelta Sharing admin operations completed!")
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
