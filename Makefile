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
# A semantic version for packages: v1.2.3 -> 1.2.3, anything else -> 0.0.0-dev+<id>.
CLEANVER := $(patsubst %-dirty,%,$(VERSION))
SEMVER := $(if $(filter v%,$(CLEANVER)),$(patsubst v%,%,$(CLEANVER)),0.0.0-dev+$(CLEANVER))

# Windows setup (NSIS runs on Linux).
setup-windows: # after build-windows (and signing); does not rebuild, so signatures stay
	cp README.md LICENSE dist/
	$(MAKENSIS) -V2 -DVERSION=$(SEMVER) -DDIST=$(CURDIR)/dist -DOUT=$(CURDIR)/dist/USB-Backup-Setup.exe packaging/windows/usb-backup.nsi

# Linux packages: .deb and .rpm (nfpm) and .tar.gz with install.sh, for
# amd64 and arm64.
LINUX_TAR_FILES := packaging/linux/install.sh packaging/linux/usb-backup@.service \
	packaging/linux/usb-backup.service packaging/linux/usb-backup.timer packaging/linux/usb-backup-cli.1 README.md LICENSE
packages-linux: build-linux
	for a in amd64 arm64; do \
		mkdir -p dist/nfpm && cp dist/linux-$$a/usb-backup-cli dist/nfpm/usb-backup-cli || exit 1; \
		for p in deb rpm; do VERSION=$(SEMVER) ARCH=$$a $(NFPM) package -f packaging/linux/nfpm.yaml -p $$p -t dist/ || exit 1; done; \
		d=dist/tar/usb-backup-$(SEMVER)-linux-$$a; mkdir -p $$d; \
		cp dist/linux-$$a/usb-backup-cli $(LINUX_TAR_FILES) $$d/ || exit 1; \
		tar -C dist/tar -czf $$d.tar.gz usb-backup-$(SEMVER)-linux-$$a || exit 1; \
		mv $$d.tar.gz dist/; \
	done

# Test binary with fake disks and a fake clock (build tag "fakes"). Never ship it.
build-testbin:
	$(BUILD) -tags fakes -ldflags "-X main.version=$(VERSION)" -o dist/test/usb-backup-cli$(shell $(GO) env GOEXE) ./cmd/usb-backup-cli

clean:
	rm -rf dist coverage.out

# Standalone gosec without the reviewed exceptions (//nolint:gosec with a
# reason, G304/G703 in .golangci.yml): every finding, for a full audit.
gosec-raw:
	$(GOSEC) -quiet -exclude-generated ./...
