package madminutils

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"testing"
	"time"

	eos "github.com/miniohq/eos/integrationtests"
	"github.com/testcontainers/testcontainers-go"
)

// Constants used as keys for identifying target containers in a map.
const (
	MinioPoolKey string = "minio-pool-0"
	MinioKey0    string = "minio-pool-0-0"
	MinioKey1    string = "minio-pool-0-1"
	MinioKey2    string = "minio-pool-0-2"
	MinioKey3    string = "minio-pool-0-3"
	NginxKey     string = "nginx"
)

// DeployMinIO deploys MinIO using the given MinIO configuration
func DeployMinIO(
	ctx context.Context,
	t *testing.T,
	minioConfig *eos.MinIOConfig,
	network *testcontainers.DockerNetwork,
	outContainers map[string]*testcontainers.Container,
) error {
	// Deploy MinIO using MinIO configuration
	minioContainers, nginxContainer, err := eos.DeployMultipleMinIO(ctx, *minioConfig, network)
	if err != nil {
		return fmt.Errorf("Unable to deploy MinIO, %w", err)
	}

	// Add created containers to output map
	for i := range minioConfig.NodeCount {
		outContainers[MinioPoolKey+"-"+strconv.Itoa(i)] = &minioContainers[0][i]
	}
	outContainers[NginxKey] = &nginxContainer

	// Check MinIO readiness via Nginx (if enabled)
	targetContainer := outContainers[MinioKey0]
	if minioConfig.Proxy.Enabled {
		targetContainer = outContainers[NginxKey]

		// Wait a bit longer for Nginx full config to be reloaded
		time.Sleep(5 * time.Second)
	}

	endpoint, err := eos.Endpoint(ctx, *targetContainer)
	if err != nil {
		return fmt.Errorf("Unable to get MinIO hostname, %w", err)
	}

	if eos.MinIOReady(t, ctx, endpoint, *minioConfig) {
		return nil
	}
	return fmt.Errorf("%s is not ready", endpoint)
}

// ListenForSignalsToExitTest listens for SIGTERM, SIGINT or a timeout
// and sends a signal to the 'stop' channel to exit test before
// the Go test expires.
// IMPORTANT: This go routine times out 1 minute before go test expires,
// so it is expected for a user to set a --timeout value greater than
// 1 minute, otherwise this go routine times out after 9 minutes.
func ListenForSignalsToExitTest(t *testing.T, stopCh chan struct{}) {
	// Create a signal channel to listen for termination signals
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM, syscall.SIGINT)

	// To quit before the Go test exits to avoid a panic
	timeout := 9 * time.Minute
	deadline, ok := t.Deadline()
	if ok {
		until := time.Until(deadline)
		if until > time.Minute {
			timeout = until - time.Minute
		}
	}
	timeoutChan := time.After(timeout)

	// Wait for either a signal or the timeout
	select {
	case sigReceived := <-signalChan:
		// If a signal is received
		fmt.Printf("Received signal: %s\n", sigReceived)
	case <-timeoutChan:
		// If the timeout expires
		fmt.Println("Timeout reached. No signal received.")
	}

	// Close stop channel
	close(stopCh)
}
