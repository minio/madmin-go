# MinIO Delta Sharing Admin API

This document describes the Delta Sharing admin API available in the madmin-go library.

## Overview

The Delta Sharing admin API allows administrators to manage Delta Sharing shares and tokens programmatically. This includes creating shares that expose Delta Lake tables or Iceberg tables (via UniForm) to external clients using the Delta Sharing protocol.

## API Methods

### Share Management

- `CreateShare(ctx, req)` - Create a new Delta Sharing share
- `ListShares(ctx)` - List all shares
- `GetShare(ctx, shareName)` - Get details of a specific share
- `UpdateShare(ctx, shareName, description, schemas)` - Update share configuration
- `DeleteShare(ctx, shareName)` - Delete a share and all associated tokens

### Token Management

- `CreateToken(ctx, shareName, req)` - Create an access token for a share
- `ListTokens(ctx, shareName)` - List all tokens for a share
- `DeleteToken(ctx, tokenID)` - Revoke a specific token

## Usage Example

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/minio/madmin-go/v3"
)

func main() {
    // Initialize admin client
    mdmClient, err := madmin.New("localhost:9000", "access-key", "secret-key", false)
    if err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()

    // Create a Delta Sharing share
    req := madmin.CreateShareRequest{
        Name:        "analytics-share",
        Description: "Analytics data for the data team",
        Schemas: []madmin.DeltaSharingSchema{
            madmin.NewSchema("default", "Default schema",
                madmin.NewDeltaTable("sales", "data-lake", "sales/"),
                madmin.NewDeltaTable("customers", "data-lake", "customers/"),
            ),
        },
    }

    shareResp, err := mdmClient.CreateShare(ctx, req)
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Created share: %s", shareResp.Share.Name)

    // Create an access token
    expires := time.Now().Add(30 * 24 * time.Hour)
    tokenReq := madmin.CreateTokenRequest{
        Description: "Token for Databricks",
        ExpiresAt:   &expires,
    }

    tokenResp, err := mdmClient.CreateToken(ctx, "analytics-share", tokenReq)
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Created token: %s", tokenResp.TokenID)

    // The profile can be saved and used with Delta Sharing clients
    // tokenResp.Profile contains the connection information
}
```

## Data Types

### Table Types

#### Delta Tables
```go
table := madmin.NewDeltaTable("table-name", "bucket", "path/to/table/")
```

#### Iceberg Tables (UniForm)
```go
table := madmin.NewUniformTable("table-name", "warehouse", "namespace", "iceberg-table")
```

### Creating Schemas
```go
schema := madmin.NewSchema("schema-name", "description", table1, table2, ...)
```

## Profile Format

When creating a token, the API returns a Delta Sharing profile that can be used by clients:

### Version 1 (Bearer Token)
```json
{
  "shareCredentialsVersion": 1,
  "endpoint": "https://minio.example.com/_delta-sharing",
  "bearerToken": "dstkn_..."
}
```

### Version 2 (OAuth)
```json
{
  "shareCredentialsVersion": 2,
  "endpoint": "https://minio.example.com/_delta-sharing",
  "tokenEndpoint": "https://minio.example.com/oauth/token",
  "clientId": "client-id",
  "clientSecret": "client-secret",
  "scope": "delta-sharing:read"
}
```

## Client Usage

The profile can be used with various Delta Sharing clients:

### Databricks
```python
profile_path = '/path/to/profile.json'
df = spark.read.format('deltaSharing').load(profile_path + '#share.schema.table')
```

### Python Delta Sharing
```python
from delta_sharing import SharingClient, load_as_pandas

client = SharingClient('/path/to/profile.json')
df = load_as_pandas('/path/to/profile.json#share.schema.table')
```

## Error Handling

All methods return standard Go errors. The API may return `DeltaSharingError` with error codes matching the Delta Sharing protocol specification:

- `SHARE_NOT_FOUND` - Share does not exist
- `INVALID_TOKEN` - Token is invalid or malformed
- `TOKEN_EXPIRED` - Token has expired
- `PERMISSION_DENIED` - Insufficient permissions

## Permissions

Admin operations require the `admin:DeltaSharing` permission in MinIO AIStor.