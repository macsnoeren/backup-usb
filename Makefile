# USB Backup – build and test targets. No Docker: everything runs with go test.

GO       ?= go
GOBIN    := $(shell $(GO) env GOPATH)/bin
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X main.version=$(VERSION)
BUILD    := CGO_ENABLED=0 $(GO) build -trimpath
FUZZTIME ?= 30s

tool = $(shell command -v $(1) 2>/dev/null || echo $(GOBIN)/$(1))
GOLANGCI_LINT := $(call tool,golangci-lint)
GOVULNCHECK   := $(call tool,govulncheck)
GOSEC         := $(call tool,gosec)

.PHONY: setup-windows packages-linux gosec-raw all test test-integration test-e2e test-long cover lint security fuzz \
        build-windows build-linux build-testbin clean

all: lint test build-linux build-windows

test:
	$(GO) test ./...

test-integration:
	$(GO) test -tags integration ./...

test-e2e:
	$(GO) test -tags e2e -count=1 ./test/e2e/...

test-long:
	$(GO) test -tags long -timeout 3h ./...

cover:
	$(GO) test -tags integration -coverprofile=coverage.out ./internal/...
	$(GO) tool cover -func=coverage.out | tail -1

lint:
	$(GOLANGCI_LINT) run ./...
	GOOS=windows $(GOLANGCI_LINT) run ./...

security:
	$(GOVULNCHECK) ./...
	$(GOLANGCI_LINT) run --enable-only gosec ./...
	GOOS=windows $(GOLANGCI_LINT) run --enable-only gosec ./...
	$(GO) test -race ./...
	$(GO) test -race -tags integration ./...
	FUZZTIME=$(FUZZTIME) GO=$(GO) sh scripts/fuzz.sh

fuzz:
	FUZZTIME=$(FUZZTIME) GO=$(GO) sh scripts/fuzz.sh

build-windows:
	GOOS=windows GOARCH=amd64 $(BUILD) -ldflags "$(LDFLAGS)" -o dist/usb-backup-cli.exe ./cmd/usb-backup-cli
	GOOS=windows GOARCH=amd64 $(BUILD) -ldflags "-H windowsgui $(LDFLAGS)" -o dist/usb-backup.exe ./cmd/usb-backup
	GOOS=windows GOARCH=arm64 $(BUILD) -ldflags "$(LDFLAGS)" -o dist/windows-arm64/usb-backup-cli.exe ./cmd/usb-backup-cli
	GOOS=windows GOARCH=arm64 $(BUILD) -ldflags "-H windowsgui $(LDFLAGS)" -o dist/windows-arm64/usb-backup.exe ./cmd/usb-backup

build-linux:
	GOOS=linux GOARCH=amd64 $(BUILD) -ldflags "$(LDFLAGS)" -o dist/linux-amd64/usb-backup-cli ./cmd/usb-backup-cli
	GOOS=linux GOARCH=arm64 $(BUILD) -ldflags "$(LDFLAGS)" -o dist/linux-arm64/usb-backup-cli ./cmd/usb-backup-cli

MAKENSIS ?= makensis
NFPM := $(call tool,nfpm)
SEMVER := $(shell echo $(VERSION) | sed 's/^v//; s/-dirty//')

# Windows setup (NSIS runs on Linux).
setup-windows: build-windows
	cp README.md LICENSE dist/
	$(MAKENSIS) -V2 -DVERSION=$(SEMVER) -DDIST=$(CURDIR)/dist -DOUT=$(CURDIR)/dist/USB-Backup-Setup.exe packaging/windows/usb-backup.nsi

# Test binary with fake disks and a fake clock (build tag "fakes"). Never ship it.
build-testbin:
	$(BUILD) -tags fakes -ldflags "-X main.version=$(VERSION)" -o dist/test/usb-backup-cli$(shell $(GO) env GOEXE) ./cmd/usb-backup-cli

clean:
	rm -rf dist coverage.out

# Standalone gosec without the reviewed exceptions (//nolint:gosec with a
# reason, G304/G703 in .golangci.yml): every finding, for a full audit.
gosec-raw:
	$(GOSEC) -quiet -exclude-generated ./...
