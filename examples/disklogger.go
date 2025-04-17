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
	// madmClnt.SetCustomTransport(&http.Transport{
	// 	Proxy: http.ProxyFromEnvironment,
	// 	DialContext: (&net.Dialer{
	// 		Timeout:       5 * time.Second,
	// 		KeepAlive:     15 * time.Second,
	// 		FallbackDelay: 100 * time.Millisecond,
	// 	}).DialContext,
	// 	MaxIdleConns:          1024,
	// 	MaxIdleConnsPerHost:   1024,
	// 	ResponseHeaderTimeout: 60 * time.Second,
	// 	IdleConnTimeout:       60 * time.Second,
	// 	TLSHandshakeTimeout:   10 * time.Second,
	// 	ExpectContinueTimeout: 1 * time.Second,
	// 	// Set this value so that the underlying transport round-tripper
	// 	// doesn't try to auto decode the body of objects with
	// 	// content-encoding set to `gzip`.
	// 	//
	// 	// Refer:
	// 	//    https://golang.org/src/net/http/transport.go?h=roundTrip#L1843
	// 	DisableCompression: true,
	// 	TLSClientConfig: &tls.Config{
	// 		// Can't use SSLv3 because of POODLE and BEAST
	// 		// Can't use TLSv1.0 because of POODLE and BEAST using CBC cipher
	// 		// Can't use TLSv1.1 because of RC4 cipher usage
	// 		MinVersion:         tls.VersionTLS12,
	// 		InsecureSkipVerify: true,
	// 	},
	// })

	logCh := madmClnt.GetAuditEvents(context.Background(), "", "")
	i := 1
	for logInfo := range logCh {
		fmt.Printf("count: %d\n", i)
		i++
		fmt.Println("************************")
		fmt.Println(logInfo)
		fmt.Println("************************")
	}
}
