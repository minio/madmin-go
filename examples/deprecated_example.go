package ci_test

import (
	"context"
	"fmt"

	"github.com/minio/madmin-go/v4"
)

// Deprecated: Use NewAdminClient instead.
func OldNew(ctx context.Context, endpoint, accessKey, secretKey string, secure bool) (*madmin.AdminClient, error) {
	return madmin.New(endpoint, accessKey, secretKey, secure)
}

func main() {
	ctx := context.Background()

	_, err := OldNew(ctx, "localhost:9000", "ACCESSKEY", "SECRETKEY", true)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	clnt, err := madmin.New("localhost:9000", "ACCESSKEY", "SECRETKEY", true) // correct usage
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	info, err := clnt.ClusterInfo(ctx)
	if err != nil {
		fmt.Println("Error fetching cluster info:", err)
		return
	}
	fmt.Printf("Cluster Info: %#v\n", info)
}
