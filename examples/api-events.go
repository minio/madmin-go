package main

import (
	"context"
	"fmt"
	"log"

	"github.com/minio/madmin-go/v3"
)

func main() {
	// Note: YOUR-ACCESSKEYID, YOUR-SECRETACCESSKEY and my-bucketname are
	// dummy values, please replace them with original values.

	// API requests are secure (HTTPS) if secure=true and insecure (HTTP) otherwise.
	// New returns an MinIO Admin client object.
	// https://172.18.0.3:32476 pcLjKJ0U5B7rCbtX d5FxdvrZro4WvodjFReeoNtya5LxncfE
	madmClnt, err := madmin.New("localhost:9000", "minio", "minio123", false)
	if err != nil {
		log.Fatalln(err)
	}
	eventCh, err := madmClnt.GetAPIEvents(context.Background(), madmin.APIEventOpts{})
	if err != nil {
		log.Fatalln(err)
	}

	for event := range eventCh {
		fmt.Printf("Event: %+v\n", event)
	}
}
