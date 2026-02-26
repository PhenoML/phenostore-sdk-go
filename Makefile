.PHONY: generate test lint lint-fix check-generated

GOLANGCI_LINT_VERSION ?= v2.10.1

# Regenerate from committed api/openapi.yaml
generate:
	go generate ./phenostore/

test:
	go test -race -count=1 -timeout 5m ./...

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...

lint-fix:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run --fix ./...

# CI: verify generated.go matches api/openapi.yaml
check-generated:
	cp phenostore/gen/generated.go phenostore/gen/generated.go.bak
	go generate ./phenostore/
	diff phenostore/gen/generated.go phenostore/gen/generated.go.bak || \
		(echo "Generated files are out of date. Run 'make generate' and commit."; \
		 mv phenostore/gen/generated.go.bak phenostore/gen/generated.go; exit 1)
	rm -f phenostore/gen/generated.go.bak
