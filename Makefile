.DEFAULT_GOAL := help

SRC        := src
BINARY     := persadomapicli
VERSION_PKG := github.com/persado/martechapi-cli/src/pkg/version
COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS    := -s -w -X $(VERSION_PKG).Commit=$(COMMIT) -X $(VERSION_PKG).BuildTime=$(BUILD_TIME)

COVER_MIN             ?= 70
GOLANGCI_LINT_VERSION := v2.15.0
GOVULNCHECK_VERSION   := v1.9.0
PLATFORMS             := windows/amd64 windows/arm64 linux/amd64 darwin/arm64

.PHONY: help build test test-race cover cover-html cover-check vet fmt fmt-check lint \
	dist go-bump vuln hooks prepush tidy verify download clean all ci

help: ## Show this help
	@awk 'BEGIN {FS = ":.*## "} /^[a-z-]+:.*## / {printf "  %-14s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Static CGO_ENABLED=0 binary -> src/bin/persadomapicli (commit/build-time stamped)
	cd $(SRC) && CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o bin/$(BINARY) ./cmd/persadomapicli

dist: ## Cross-compile PLATFORMS -> src/dist/persadomapicli-<os>-<arch>[.exe] + SHA256SUMS (darwin named macos)
	rm -rf $(SRC)/dist && mkdir -p $(SRC)/dist
	cd $(SRC) && for p in $(PLATFORMS); do \
	  os=$${p%/*}; arch=$${p#*/}; name=$$os; ext=; \
	  [ $$os = darwin ] && name=macos; [ $$os = windows ] && ext=.exe; \
	  CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags '$(LDFLAGS)' \
	    -o dist/$(BINARY)-$$name-$$arch$$ext ./cmd/$(BINARY) || exit 1; \
	done
	cd $(SRC)/dist && shasum -a 256 $(BINARY)-* > SHA256SUMS && cat SHA256SUMS

test: ## go test ./... -count=1
	cd $(SRC) && go test ./... -count=1

test-race: ## CGO_ENABLED=1 go test -race ./... -count=1
	cd $(SRC) && CGO_ENABLED=1 go test -race ./... -count=1

cover: ## Coverage profile (src/coverage.out) + total line
	cd $(SRC) && go test ./... -count=1 -coverprofile=coverage.out && go tool cover -func=coverage.out | tail -1

cover-html: cover ## HTML report -> src/coverage.html
	cd $(SRC) && go tool cover -html=coverage.out -o coverage.html

cover-check: ## Fail if total coverage (with -race) < COVER_MIN (70%)
	cd $(SRC) && CGO_ENABLED=1 go test -race ./... -count=1 -coverprofile=coverage.out && \
	go tool cover -func=coverage.out | awk -v min=$(COVER_MIN) \
	  '/^total:/ {sub("%", "", $$3); printf "total coverage %s%% (min %s%%)\n", $$3, min; if ($$3 + 0 < min) exit 1}'

vet: ## go vet ./...
	cd $(SRC) && go vet ./...

fmt: ## gofmt -w .
	cd $(SRC) && gofmt -w .

fmt-check: ## Fail if any file is not gofmt-clean
	@out=$$(cd $(SRC) && gofmt -l .); if [ -n "$$out" ]; then echo "not gofmt-clean:"; echo "$$out"; exit 1; fi

lint: ## golangci-lint at GOLANGCI_LINT_VERSION, built with the go.mod Go (default rules)
	cd $(SRC) && go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...

go-bump: ## Move src/go.mod to Go X.Y.Z (V=X.Y.Z); CI reads its Go version from go.mod
	@test -n "$(V)" || { echo "usage: make go-bump V=X.Y.Z"; exit 1; }
	cd $(SRC) && go mod edit -go=$(V)

vuln: ## govulncheck ./... at GOVULNCHECK_VERSION; fails on any reachable vulnerability
	cd $(SRC) && go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

hooks: ## Install the git hooks in dev/githooks (pre-commit: vuln, pre-push: ci)
	git config core.hooksPath dev/githooks

prepush: ## Run the pre-push gate without pushing
	dev/githooks/pre-push

tidy: ## go mod tidy
	cd $(SRC) && go mod tidy

verify: ## go mod verify
	cd $(SRC) && go mod verify

download: ## go mod download
	cd $(SRC) && go mod download

clean: ## Remove src/bin, src/dist, src/coverage.out, src/coverage.html
	rm -rf $(SRC)/bin $(SRC)/dist $(SRC)/coverage.out $(SRC)/coverage.html

all: fmt-check vet test build ## fmt-check + vet + test + build (local default)

ci: fmt-check vet lint vuln cover-check build ## fmt-check + vet + lint + vuln + cover-check + build
