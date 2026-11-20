GOPATH             := $(shell go env GOPATH | tr '\\' '/')
GORELEASER_VERSION := 2.18.2
STATICCHECK_VERSION := 2025.1.1
PACKER_SDC_VERSION := $(shell go list -m -f '{{.Version}}' github.com/hashicorp/packer-plugin-sdk)
GORELEASER         ?= $(GOPATH)/bin/goreleaser
STATICCHECK        ?= $(GOPATH)/bin/staticcheck
PACKER_SDC         ?= $(GOPATH)/bin/packer-sdc
PWSH               ?= pwsh
PESTER_SHELL       ?= powershell.exe
PACKER             ?= packer
DOWNLOADS          ?=
API_VERSION         = $(shell go run . describe 2>/dev/null | jq -r .api_version)

.PHONY: check lint fmt-check doccheck pslint hclfmt typecheck test test-go test-ps fmt generate generate-check baselines init build clean

check: lint typecheck test

lint: fmt-check doccheck pslint hclfmt
	$(STATICCHECK) ./...

fmt-check:
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

doccheck:
	go run ./cmd/doccheck .
	@sh ./psdoccheck.sh

pslint:
	$(PWSH) -NoProfile -Command '$$ErrorActionPreference = "Stop"; $$r = "guest/scripts", "test/pester" | ForEach-Object { Invoke-ScriptAnalyzer -Path $$_ -Recurse -Severity Error }; $$r; if ($$r) { exit 1 }'

hclfmt:
	$(PACKER) fmt -check -recursive examples

typecheck:
	go build ./...
	go vet ./...

test: test-go test-ps

test-go:
	go test ./...

test-ps:
	$(PESTER_SHELL) -NoProfile -ExecutionPolicy Bypass -Command '$$c = New-PesterConfiguration; $$c.Run.Path = "test/pester"; $$c.Run.Exit = $$true; $$c.Output.Verbosity = "Detailed"; Invoke-Pester -Configuration $$c'

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

init: $(GORELEASER) $(STATICCHECK) $(PACKER_SDC)
	go mod download

$(GORELEASER):
	go install github.com/goreleaser/goreleaser/v2@v$(GORELEASER_VERSION)

$(STATICCHECK):
	go install honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)

$(PACKER_SDC):
	go install github.com/hashicorp/packer-plugin-sdk/cmd/packer-sdc@$(PACKER_SDC_VERSION)

build: $(GORELEASER)
	API_VERSION="$(API_VERSION)" $(GORELEASER) build --snapshot --skip=validate --clean --single-target

clean:
	rm -rf dist reports windows-harden-report-*.json
