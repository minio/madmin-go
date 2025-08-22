GOPATH := $(shell go env GOPATH)
GOARCH := $(shell go env GOARCH)
GOOS := $(shell go env GOOS)

EOS_LOCAL_REF ?= '../eos'
MADMIN_LOCAL_REF ?= ../madmin-go
MADMIN_MODULE ?= github.com/minio/madmin-go/v4

.PHONY: prep-integration

integration-prep:
	@echo "Replace madmin-go module ref to local one and build AIStor MinIO binary/image."
	@cd ${EOS_LOCAL_REF} && \
		go mod edit -replace $(MADMIN_MODULE)=$(MADMIN_LOCAL_REF) && \
		go mod tidy && \
		GOOS=linux make build-dev && \
		docker build -q --no-cache -t minio/eos:noop --build-arg TARGETARCH=${GOARCH} . -f Dockerfile

integration-tests: integration-prep
	@cd integrationtests && \
	for test in $$(go test -list . | grep '^Test.*'); do \
		EOS_IMAGE="minio/eos:noop" go test -v -run=^$${test}$$ . -count=1 -timeout=30m; \
	done