# MinIO Admin Automated Integration Testing

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

2. In your local `madmin-go` repository in the `madmin-go/integrationtests` module, replace the Go module reference `github.com/miniohq/eos/integrationtests` to point to the local `eos/integrationtests` module

3. In your local `eos` repository in the `eos/integrationtests` module, replace the Go module reference `github.com/minio/madmin-go/v4` to point to the local `madmin-go` module

4. Deploy MinIO using the local `eos/integrationtests` utilities which resolve the `madmin-go` dependencies to your local `madmin-go`

5. Add specific test logic

Note: `minio/madmin-go` repository `Makefile` contains target `integration-prep` which takes care of points (2) and (3).

---

## 🐞 Debugging Mode

All tests support a `--debug` flag:

```bash
go test -v ./... -args --debug
