# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is the MinIO Admin Golang Client SDK (`github.com/minio/madmin-go/v4`), which provides APIs to manage MinIO services. It's a Go library that enables programmatic administration of MinIO object storage clusters.

## Development Commands

### Testing
- **Run all tests**: `go test -v -race ./...`
- **Run tests for 386 architecture**: `GOARCH=386 GOOS=linux go test -v ./...`
- **Run single test**: `go test -v -run TestName ./...`

### Code Quality
- **Lint check**: `golangci-lint run --timeout 5m --config ./.golangci.yml`
- **Static analysis**: `go vet -vettool=$(which staticcheck) ./...`
- **Format code**: Automatically handled by `gofumpt` and `goimports` (configured in `.golangci.yml`)
- **Standard vet**: `go vet ./...`

### Code Generation
- **Install code generation tools**: `go install -v tool` (installs `msgp` and `stringer`)
- **Generate all**: `go generate ./...` (generates `*_gen.go` files using msgp and stringer)
- **Check generated files**: After generation, ensure no uncommitted `*_gen.go` files exist

### Build Commands
- **Build**: `go build ./...`
- **Install dependencies**: `go mod tidy`
- **Verify dependencies**: `go mod verify`

## Code Architecture

### Core Client Structure
- **`AdminClient`** (`api.go`): Main client struct that implements Amazon S3 compatible admin methods
- **Authentication**: Uses MinIO credentials with signature-based authentication via `minio-go/v7`
- **Transport**: HTTP client with configurable timeouts, retries, and TLS settings

### Command Categories
The codebase is organized into logical command groups, each in separate `*-commands.go` files:

- **Cluster Management**: `cluster-commands.go` - Node management, rebalancing, decommissioning
- **Health & Monitoring**: `health.go`, `metrics.go` - System health checks, performance metrics
- **Information**: `info-commands.go`, `info-v4-commands.go` - Server/cluster information APIs
- **Healing**: `heal-commands.go` - Data healing and consistency operations
- **User Management**: `user-commands.go`, `group-commands.go` - IAM user/group operations
- **Configuration**: `config-*-commands.go` - Server configuration management
- **Storage Management**: Tiering, bandwidth, quota operations
- **Security**: `kms-commands.go`, `policy-commands.go` - KMS and policy management
- **Replication**: `cluster-commands.go`, `replication-api.go` - Site replication setup
- **Logging**: `api-log*.go`, audit and error log management

### Code Generation
- **MessagePack Serialization**: Uses `msgp` to generate efficient binary serialization for structs
- **String Generation**: Uses `stringer` for enum string methods
- **Generated Files**: All `*_gen.go` files are auto-generated and should not be manually edited

### Key Data Structures
- **Health Information**: Complex nested structs in `health.go` for system diagnostics
- **Metrics**: Performance and resource utilization data structures in `metrics.go`
- **Service Info**: Server metadata and configuration structures
- **Healing Status**: Data repair and consistency check results

### Error Handling
- **API Errors**: Custom error types for different API response scenarios
- **Retry Logic**: Built-in retry mechanism with exponential backoff (`retry.go`)
- **Context Support**: All APIs support context-based cancellation

## Important Patterns

### Generated Code Management
- Never manually edit `*_gen.go` files
- After modifying structs with `msgp` tags, run `go generate ./...`
- CI ensures generated files are up-to-date before merging

### Testing Approach
- Use `go test -v -race ./...` to catch race conditions
- Test both standard and 386 architectures
- Examples in `examples/` directory demonstrate real usage patterns

### API Versioning
- The module uses semantic versioning (`v4`)
- Some APIs have version-specific implementations (e.g., `info-v4-commands.go`)
- Maintain backward compatibility within major versions

## Dependencies
- **Core**: `minio-go/v7` for S3 client functionality and credentials
- **Serialization**: `msgp` for fast binary marshaling
- **System Info**: `gopsutil/v4` for system metrics collection
- **Metrics**: Prometheus client libraries for metrics exposition
- **Security**: JWT tokens, crypto libraries for secure operations