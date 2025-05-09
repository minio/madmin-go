package main

import (
	"context"
	"fmt"
	"log"

	"github.com/minio/madmin-go/v4"
)

func main() {
	// Note: YOUR-ACCESSKEYID, YOUR-SECRETACCESSKEY and my-bucketname are
	// dummy values, please replace them with original values.

	// API requests are secure (HTTPS) if secure=true and insecure (HTTP) otherwise.
	// New returns an MinIO Admin client object.
	madmClnt, err := madmin.New("localhost:9000", "minio", "minio123", false)
	if err != nil {
		log.Fatalln(err)
	}
	if err != nil {
		log.Fatalln(err)
	}
	eventCh, err := madmClnt.GetErrorEvents(context.Background(), madmin.ErrorEventOpts{})
	if err != nil {
		log.Fatalln(err)
	}

	for event := range eventCh {
		fmt.Printf("Event: %+v\n", event)
	}
}
