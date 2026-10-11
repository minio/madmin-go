# Golang Admin Client API Reference [![Slack](https://slack.min.io/slack?type=svg)](https://slack.min.io)

The MinIO Admin Golang Client SDK provides APIs to manage MinIO services.

This document assumes that you have a working [Golang setup](https://golang.org/doc/install).

## Initialize MinIO Admin Client object.

## MinIO

```go

package main

import (
    "fmt"

    "github.com/minio/madmin-go/v4"
)

func main() {
    // Use a secure connection.
    ssl := true

    // Initialize minio client object.
    mdmClnt, err := madmin.New("your-minio.example.com:9000", "YOUR-ACCESSKEYID", "YOUR-SECRETKEY", ssl)
    if err != nil {
        fmt.Println(err)
        return
    }

    // Fetch service status.
	info, err := mdmClnt.ClusterInfo(context.Background())
    if err != nil {
        fmt.Println(err)
        return
    }
	fmt.Printf("%#v\n", info)
}
```

## Admin API path validation

Resource identifiers used in admin API paths, including tier names, scanner
bucket names, inventory IDs, and node hostnames, must be a single path segment.
Empty values, dot segments, and values containing `/` or `\` are rejected
before a request is sent. Other URL delimiters are escaped.

See [issue #689](https://github.com/minio/madmin-go/issues/689) and
[pull request #690](https://github.com/minio/madmin-go/pull/690).

## Documentation

All documentation is available [here](https://pkg.go.dev/github.com/minio/madmin-go/v4)

## License

This SDK is licensed under [GNU AGPLv3](https://github.com/minio/madmin-go/blob/master/LICENSE).
