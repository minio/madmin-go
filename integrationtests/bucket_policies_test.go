package integrationtests

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/minio/madmin-go/integrationtests/madminutils"
	"github.com/minio/madmin-go/v4"
	"github.com/minio/minio-go/v7"
	miniogo "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/miniohq/automation-go/docker"
	"github.com/miniohq/automation-go/utils"
	"github.com/testcontainers/testcontainers-go"
	"k8s.io/klog"

	eos "github.com/miniohq/eos/integrationtests"
	"github.com/stretchr/testify/require"
)

var (
	debugFlag = flag.Bool("debug", false, "enable debug mode")
)

// TestBucketPoliciesAndUsersAPI validates MinIO bucket policy behavior using both
// canned policies and a non-root user. It checks:
// - read-only and write-only bucket policies
// - permissions enforcement for uploading and downloading
// - user access control with attached policies
//
// APIs tested:
// - admin/v*/add-canned-policy
// - admin/v*/add-user
// - admin/v*/idp/builtin/policy/attach
//
// Steps:
//
//  1. Deploy MinIO pool of 4 servers
//
//  2. Build MinIO connection with root access and create 2 buckets,
//     `read-only-bucket` and `write-only-bucket`
//
//  3. Copy 100MiB object to bucket `read-only-bucket`
//
//  4. Create and add `read-only-bucket-policy` and `write-only-bucket-policy` policies
//
//  5. Create non-root user and attach policies from step(4) to the user
//
//  6. Build MinIO connection using non-root user credentials
//
//  7. Copy 100MiB object to bucket `write-only-bucket` with no error expected
//
//  8. Attempt to read object from `write-only-bucket` bucket and verify that
//     `AccessDenied` error is received
//
//  9. Attempt to copy another 100MiB object to `read-only-bucket` and verify that
//     `AccessDenied` error is received
//
// 10. Read 100MiB object (from step 3) from bucket `read-only-bucket` with no error expected
func TestBucketPoliciesAndUsersAPI(t *testing.T) {
	flag.Parse()
	ctx := context.Background()
	assert := require.New(t)

	eosImage := os.Getenv("EOS_IMAGE")
	assert.NotEmpty(eosImage, "EOS image not provided, cannot run test")

	// If --debug is set, the test is paused before ending to allow debugging the madmin API calls
	stopCh := make(chan struct{})
	if *debugFlag {
		klog.Info("DEBUG MODE ENABLED")
		go madminutils.ListenForSignalsToExitTest(t, stopCh)
	}

	defer func() {
		if *debugFlag {
			<-stopCh
		}
	}()

	minioAccessKey := "minio"
	minioSecretKey := "minio123"
	minioConfig := eos.MinIOConfig{
		Name:      "MinIO",
		Image:     eosImage,
		License:   madminutils.AIStorLicense,
		AccessKey: minioAccessKey,
		SecretKey: minioSecretKey,
		TmpDir:    t.TempDir(),
		TLS:       true, // Lets by default use TLS verification
		NodeCount: 4,    // Deploy 1 pool of 4 MinIO instances (each server defaults to 4 drives)
		PoolCount: 1,    // The default is 1, but let's explicitly set it for clarity.
	}

	docker.TestInDocker(ctx, t, func(network *testcontainers.DockerNetwork) {
		// Deploy MinIO
		containers := make(map[string]*testcontainers.Container)
		err := madminutils.DeployMinIO(ctx, t, &minioConfig, network, containers)
		assert.NoError(err, "Unable to deploy MinIO pool")

		minioConn, err := eos.ConnectMinIO(ctx, *containers[madminutils.MinioKey0], minioConfig)
		assert.NoError(err, "Unable to obtain MinIO connection")

		// Obtain the MinIO client
		minioClient, err := minioConn.MinioClient()
		assert.NoError(err, "Unable to obtain MinIO client")

		// Build admin client
		adminClnt, err := minioConn.AdminClient()
		assert.NoError(err, "Unable to obtain MinIO admin client")

		// Create a couple of buckets
		readOnlyBucket := "read-only-bucket"
		err = minioClient.MakeBucket(ctx, readOnlyBucket, miniogo.MakeBucketOptions{})
		assert.NoError(err, "Unable to create bucket")

		writeOnlyBucket := "write-only-bucket"
		err = minioClient.MakeBucket(ctx, writeOnlyBucket, miniogo.MakeBucketOptions{})
		assert.NoError(err, "Unable to create bucket")

		// Upload object to readOnlyBucketName
		const object0Name = "folder/subfolder/object0.bin"
		fileSize := int64(100 * 1024 * 1024) // 100MiB
		inFile := utils.NewTestBinaryFile(fileSize)

		info, err := minioClient.PutObject(ctx, readOnlyBucket, object0Name, inFile, fileSize, miniogo.PutObjectOptions{})
		assert.NoError(err, "Unable to copy object to bucket")
		assert.Equal(readOnlyBucket, info.Bucket, "Bucket mismatch")
		assert.Equal(fileSize, info.Size, "Size mismatch")
		klog.Infof("Copied a 100MiB object to bucket %q", readOnlyBucket)

		// Make bucket `readOnlyBucket` readonly
		readOnlyBucketPolicy := fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "",
      "Effect": "Allow",
      "Action": ["s3:GetObject"],
      "Resource": ["arn:aws:s3:::%s/*"]
    }
  ]
}`, readOnlyBucket)

		readOnlyBucketPolicyName := "read-only-bucket-policy"
		err = adminClnt.AddCannedPolicy(ctx, readOnlyBucketPolicyName, []byte(readOnlyBucketPolicy))
		assert.NoError(err, "Unable to add policy")

		// Make bucket `writeOnlyBucket`` readonly
		writeOnlyBucketPolicy := fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "",
      "Effect": "Allow",
      "Action": ["s3:PutObject"],
      "Resource": ["arn:aws:s3:::%s/*"]
    }
  ]
}`, writeOnlyBucket)

		writeOnlyBucketPolicyName := "write-only-bucket-policy"
		err = adminClnt.AddCannedPolicy(ctx, writeOnlyBucketPolicyName, []byte(writeOnlyBucketPolicy))
		assert.NoError(err, "Unable to add policy")

		// Add non-root user
		nonRootUsrAccessKey := "my-user"
		nonRootUsrSecretKey := nonRootUsrAccessKey + "123"
		err = adminClnt.AddUser(ctx, nonRootUsrAccessKey, nonRootUsrSecretKey)
		assert.NoError(err, "Unable to add non-root user")

		_, err = adminClnt.AttachPolicy(ctx, madmin.PolicyAssociationReq{
			Policies: []string{readOnlyBucketPolicyName, writeOnlyBucketPolicyName},
			User:     nonRootUsrAccessKey,
		})
		assert.NoError(err, "Unable to attach policies to non-root user")

		// Get connection with MinIO using non-root user
		minioConn, err = eos.ConnectMinIO(
			ctx,
			*containers[madminutils.MinioKey0],
			minioConfig,
			eos.WithCredentials(credentials.NewStaticV4(nonRootUsrAccessKey, nonRootUsrSecretKey, "")),
		)
		assert.NoError(err, "Unable to obtain MinIO connection with non-root user")

		// Obtain the MinIO client
		minioClient, err = minioConn.MinioClient()
		assert.NoError(err, "Unable to obtain MinIO client")

		// Upload object to writeOnlyBucket
		const object1Name = "folder/subfolder/object1.bin"
		fileSize = int64(100 * 1024 * 1024) // 100MiB
		inFile = utils.NewTestBinaryFile(fileSize)

		info, err = minioClient.PutObject(ctx, writeOnlyBucket, object1Name, inFile, fileSize, miniogo.PutObjectOptions{})
		assert.NoError(err, "Unable to copy object to bucket")
		assert.Equal(writeOnlyBucket, info.Bucket, "Bucket mismatch")
		assert.Equal(fileSize, info.Size, "Size mismatch")
		klog.Infof("Copied a 100MiB object to bucket %q", writeOnlyBucket)

		// Try to read object from writeOnlyBucket
		f, err := minioClient.GetObject(ctx, writeOnlyBucket, object1Name, minio.GetObjectOptions{})
		assert.NoError(err, "Unable to read object")
		defer f.Close()

		// Expect access denied error
		_, err = f.Stat()
		eos.AssertAccessDenied(assert, err)

		// Try to write to readOnlyBucket
		inFile.Seek(0, 0)
		_, err = minioClient.PutObject(ctx, readOnlyBucket, object1Name, inFile, fileSize, miniogo.PutObjectOptions{})
		eos.AssertAccessDenied(assert, err)

		// Read object from readOnlyBucket
		f, err = minioClient.GetObject(ctx, readOnlyBucket, object0Name, minio.GetObjectOptions{})
		assert.NoError(err, "Unable to read object")
		defer f.Close()

		getInfo, err := f.Stat()
		assert.NoError(err, "Unable to read object information")
		assert.Equal(fileSize, getInfo.Size, "Size mismatch")

		hasher := sha256.New()
		io.Copy(hasher, f)
		hash := hex.EncodeToString(hasher.Sum(nil))
		assert.Equal("4cbf988462cc3ba2e10e3aae9f5268546aa79016359fb45be7dd199c073125c0", hash, "Invalid SHA256 hash")
	})
}
