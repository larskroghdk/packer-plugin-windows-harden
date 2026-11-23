GOPATH              := $(shell go env GOPATH | tr '\\' '/')
GORELEASER_VERSION  := 2.18.2
STATICCHECK_VERSION := 2025.1.1
GOVULNCHECK_VERSION := 1.10.0
ACTIONLINT_VERSION  := 1.7.11
PACKER_SDC_VERSION  := $(shell go list -m -f '{{.Version}}' github.com/hashicorp/packer-plugin-sdk)
GORELEASER          ?= $(GOPATH)/bin/goreleaser
STATICCHECK         ?= $(GOPATH)/bin/staticcheck
GOVULNCHECK         ?= $(GOPATH)/bin/govulncheck
PACKER_SDC          ?= $(GOPATH)/bin/packer-sdc
PWSH                ?= pwsh
PESTER_SHELL        ?= powershell.exe
PACKER              ?= packer
DOWNLOADS           ?=
API_VERSION          = $(shell go run . describe 2>/dev/null | jq -r .api_version)

.PHONY: check lint fmt-check doccheck pslint hclfmt actionlint typecheck vulncheck \
	test test-go test-ps test-ai fmt generate generate-check baselines init \
	build release-snapshot release clean

check: lint typecheck vulncheck test

lint: fmt-check doccheck pslint hclfmt actionlint $(STATICCHECK)
	$(STATICCHECK) ./...

fmt-check:
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

doccheck:
	go run ./cmd/doccheck .
	@sh ./psdoccheck.sh

pslint:
	$(PWSH) -NoProfile -Command '$$ErrorActionPreference = "Stop"; $$r = "guest/scripts", "test", "scripts" | ForEach-Object { Invoke-ScriptAnalyzer -Path $$_ -Recurse -Severity Error }; $$r; if ($$r) { exit 1 }'

hclfmt:
	$(PACKER) fmt -check -recursive examples

actionlint:
	go run github.com/rhysd/actionlint/cmd/actionlint@v$(ACTIONLINT_VERSION)

typecheck:
	go build ./...
	go vet ./...

vulncheck: $(GOVULNCHECK)
	$(GOVULNCHECK) ./...

test: test-go test-ps test-ai

test-go:
	go test ./...

test-ps:
	$(PESTER_SHELL) -NoProfile -ExecutionPolicy Bypass -Command '$$c = New-PesterConfiguration; $$c.Run.Path = "test/pester"; $$c.Run.Exit = $$true; $$c.Output.Verbosity = "Detailed"; Invoke-Pester -Configuration $$c'

test-ai:
	$(PWSH) -NoProfile -Command 'Import-Module Pester -MinimumVersion 5.6.1; $$c = New-PesterConfiguration; $$c.Run.Path = "test/ai"; $$c.Run.Exit = $$true; $$c.Output.Verbosity = "Detailed"; Invoke-Pester -Configuration $$c'

fmt:
	gofmt -w .
	$(PACKER) fmt -recursive examples

generate: $(PACKER_SDC)
	cd provisioner && GOFILE=config.go GOPACKAGE=provisioner $(PACKER_SDC) mapstructure-to-hcl2 -type Config
	gofmt -w provisioner/config.hcl2spec.go

generate-check: generate
	git diff --exit-code provisioner/config.hcl2spec.go || (echo 'provisioner/config.hcl2spec.go is stale: run make generate and commit it'; exit 1)

baselines:
	@if [ -z "$(DOWNLOADS)" ]; then echo "usage: make baselines DOWNLOADS=<dir with SCT zips>"; exit 1; fi
	go run ./cmd/sctimport -downloads "$(DOWNLOADS)" -out internal/baseline/data

init: $(STATICCHECK) $(GOVULNCHECK) $(PACKER_SDC)
	go mod download

$(GORELEASER) $(STATICCHECK) $(GOVULNCHECK) $(PACKER_SDC): Makefile go.mod

$(GORELEASER):
	GOTOOLCHAIN=auto go install github.com/goreleaser/goreleaser/v2@v$(GORELEASER_VERSION)

$(STATICCHECK):
	go install honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)

$(GOVULNCHECK):
	go install golang.org/x/vuln/cmd/govulncheck@v$(GOVULNCHECK_VERSION)

$(PACKER_SDC):
	go install github.com/hashicorp/packer-plugin-sdk/cmd/packer-sdc@$(PACKER_SDC_VERSION)

build: $(GORELEASER)
	API_VERSION="$(API_VERSION)" $(GORELEASER) build --snapshot --skip=validate --clean --single-target

release-snapshot: $(GORELEASER)
	API_VERSION="$(API_VERSION)" $(GORELEASER) release --snapshot --skip=publish,sign --clean

release: $(GORELEASER)
	API_VERSION="$(API_VERSION)" $(GORELEASER) release --clean

clean:
	rm -rf dist reports windows-harden-report-*.json
