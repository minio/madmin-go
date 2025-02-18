# Golang Admin Client API Reference [![Slack](https://slack.min.io/slack?type=svg)](https://slack.min.io)
The MinIO Admin Golang Client SDK provides APIs to manage MinIO services.

This document assumes that you have a working [Golang setup](https://golang.org/doc/install).

## Initialize MinIO Admin Client object.

##  MinIO

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

## Documentation
All documentation is available [here](https://pkg.go.dev/github.com/minio/madmin-go/v4)

## License
This SDK is licensed under [GNU AGPLv3](https://github.com/minio/madmin-go/blob/master/LICENSE).

