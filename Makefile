.PHONY: generate update-generator check test lint lint-fix check-generated

GOLANGCI_LINT_VERSION ?= v2.10.1
GENERATOR_VERSION ?= latest
export GENERATOR_VERSION

# Regenerate from committed api/openapi.yaml
generate:
	go generate ./phenostore/

# Upgrade the generator and regenerate in the same change, keeping the SDK's
# minimum Go version. The tools module has its own Go version and dependencies.
update-generator:
	@set -eu; \
	sdk_go=$$(go list -m -f '{{.GoVersion}}'); \
	go -C tools get -tool "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$$GENERATOR_VERSION"; \
	go -C tools mod tidy; \
	$(MAKE) generate; \
	go mod tidy -go="$$sdk_go"; \
	$(MAKE) check

# Validate runtime compatibility on the minimum Go version in go.mod, even
# when generator maintenance requires a newer toolchain in tools/go.mod.
check:
	@set -eu; \
	sdk_go=$$(go list -m -f '{{.GoVersion}}'); \
	GOTOOLCHAIN="go$$sdk_go" go build ./...; \
	GOTOOLCHAIN="go$$sdk_go" $(MAKE) test lint; \
	GOTOOLCHAIN=auto $(MAKE) check-generated

test:
	go test -race -count=1 -timeout 5m ./...

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...

lint-fix:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run --fix ./...

# CI: verify the spec, generator version and committed output agree. Restore
# the original even if generation fails, so validation never changes the SDK.
check-generated:
	@set -eu; \
	backup=$$(mktemp); \
	cp phenostore/gen/generated.go "$$backup"; \
	trap 'cp "$$backup" phenostore/gen/generated.go; rm -f "$$backup"' EXIT; \
	go generate ./phenostore/; \
	diff -u "$$backup" phenostore/gen/generated.go || \
		{ echo "Generated files are out of date. Run 'make generate' and commit."; exit 1; }
