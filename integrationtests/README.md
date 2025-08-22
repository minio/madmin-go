# AIStor MinIO API Integration Testing With madmin Clients

This test suite is designed to validate the **MinIO Admin (madmin) client and MinIO APIs behavior** through automated integration testing using the [automation-go](https://github.com/miniohq/automation-go) framework. The goal is to verify that changes made to the MinIO admin subsystem integrate correctly with MinIO’s backend APIs, and to ensure comprehensive test coverage of all MinIO Admin APIs invoked via the madmin client. This ensures that the system behaves as expected under real-world deployment conditions and that the Admin API contract remains stable, functional, and consistent across releases.

These tests use MinIO’s official clients (`minio-go`, `madmin-go`) and run MinIO itself inside Docker containers via `testcontainers-go`.

---

## ✅ Test Purpose

Each test is designed to:

- Validate specific MinIO API behavior
- Use **real madmin clients** (not mocks) against a **real MinIO deployment**
- Exercise:
  - `admin` API calls (e.g., adding policies, users)
  - `S3`-style operations (PUT/GET object)
- Test **end-to-end functionality** (not just isolated units)

---

## 📦 Prerequisites

To run or develop against this test suite, ensure you have the following installed:

| Tool                     | Purpose                             | Version/Notes     |
|--------------------------|-------------------------------------|-------------------|
| Go                       | Build and run tests                 | Go 1.24+ recommended |
| Docker                   | Run MinIO in isolated containers    | Latest            |
| [`make`](https://www.gnu.org/software/make/)        | Build/test automation           | Latest |
| [`mc` CLI (optional)](https://min.io/docs/minio/linux/reference/minio-mc.html) | Debug MinIO outside test code | Optional |

Also ensure you have access to a MinIO Docker image (e.g., via `EOS_IMAGE` environment variable).

---

## Integration Testing strategy

This integration testing strategy ensures that changes in the `madmin-go` repo do correctly integrate with the AIStor MinIO APIs. The following steps outline the integration testing strategy in general:

1. Checkout `minio/madmin-go` and `miniohq/eos` repositories locally

2. In your local `eos` repository in the `eos/integrationtests` module, replace the Go module reference `github.com/minio/madmin-go/v4` to point to the local `madmin-go` module

3. Deploy MinIO using the local `eos/integrationtests` utilities which resolve the `madmin-go` dependencies to your local `madmin-go`

4. Add specific test logic

Note: `minio/madmin-go` repository `Makefile` contains target `integration-prep` which takes care of point (2).

---

## How to test locally?

The provided Makefile includes helpful targets to streamline the process of setting up the local environment and running the integration tests. AIStor is deployed using testcontainer in Go.

### Integration tests

The following steps describe the process to run the integration tests locally:

1. **Clone the `minio/madmin-go` and `miniohq/eos` repositories locally:**

   If you already have both repositories checked out and up to date, you can skip this step.

   The following commands assume that the repositories will be cloned into your `$HOME` directory. You can change the directory path as needed.

   ```bash
   cd $HOME
   git clone https://github.com/minio/madmin-go.git
   git clone https://github.com/miniohq/eos.git
   ```

2. **Build AIStor MinIO EOS Image:**

   As outlined in the `Integration Testing Strategy` section, the local eos repository needs to replace the Go module reference `github.com/minio/madmin-go/v4` with a reference to the local `madmin-go` module. This replacement, along with building the AIStor MinIO image, is handled by running the following command from the root directory of your local `madmin-go` repository:

   ```bash
   EOS_LOCAL_REF=../eos MADMIN_LOCAL_REF=../madmin-go make integration-prep
   ```

   Where:

   - `EOS_LOCAL_REF`: Specifies the relative path to your local `eos` repository. Update this path as needed to reflect your local directory structure.
   - `MADMIN_LOCAL_REF`: Specifies the relative path from the local `eos` repository to the local `madmin-go` repository. Modify this value as needed to match your local directory structure.

3. **Set up docker environment:**

   To deploy MinIO pool, [testcontainers](https://golang.testcontainers.org) package from Go is used. This package requires access to the Docker daemon socket to manage the lifecycle of containers. By default, it expects the socket to be available at `/var/run/docker.sock`.

   However, for users running Docker inside environments like Colima, which provides a lightweight VM to run Docker on macOS and Linux, the Docker daemon socket may be located elsewhere. To resolve this discrepancy, the recommended solution is to create a symbolic link pointing to the actual location of the Docker socket.

   For Colima users, to determine the location of your Docker socket, run:

   ```bash
   colima status
   ```

   Once you have located the Docker daemon socket, create a symbolic link from its actual location to the default path expected by `testcontainers`. For example, in my setup, this can be done with:

   ```bash
   sudo ln -sf $HOME/.colima/default/docker.sock /var/run/docker.sock
   ```

4. **Run ALL Integration Tests:**

   After setting up the environment, run ALL integration tests with the following command:

   ```bash
   make integration-tests
   ```

   Above target executes all integration tests from your local `madmin-go/integrationtests` directory.

5. **Run a Single Integration Test:**

   To execute a specific integration test, follow the steps below::

   - Navigate to `madmin-go/integrationtests` directory:

     ```bash
     cd integrationtests
     ```

   - To run a specific integration test, use the following command:

     ```bash
     EOS_IMAGE="minio/eos:noop" go test . -v -run=TestBucketPoliciesAndUsersAPI -count 1 --timeout 30m
     ```

     Where:

     - `EOS_IMAGE`: The name of the `eos` image built using the `integration-prep` target in the Makefile.

   - To keep the AIStor MinIO pool instance running after test completes (successfully or not), for debugging purposes, set the `--debug` argument when executing the go test:

     ```bash
     EOS_IMAGE="minio/eos:noop" go test . -v -run=TestBucketPoliciesAndUsersAPI -count 1 --timeout 30m -args --debug
     ```

     Note: When debugging is enabled, after the integration test complete, the Go test will remain blocked, waiting for either a termination/interrupt signal (e.g., `Ctrl + C`) or for an internal timer (set to a default duration of 9 minutes) to expire. Since the default Go test timeout is 10 minutes, the internal timer is designed to stop the test’s goroutine before the overall timeout is reached. You can adjust the Go test timeout by passing the `--timeout` flag when running `go test`. During this waiting period, the AIStor MinIO pool remains accessible locally until one of these conditions occurs.

## How to add more Integration tests?

Currently, the integration tests are placed within the `integrationtests` directory. As long as go tests are added in this directory our CI/CD pipeline and our local Makefile targets will be able to grab those tests automatically for their execution.

To improve test organization and readability, a header is added to each test to describe the test behavior and the list of AIStor APIs tested, as shown in the example below:

```go
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
    ...
}
```

### Resources

Follow structure of sample test `TestBucketPoliciesAndUsersAPI` to design more integration tests. Utilities from below Go modules can be used so that there is no need to reinvent the wheel. 

- [Automation Go framework](https://github.com/miniohq/automation-go)
- [EOS testing resources](https://github.com/miniohq/eos)


