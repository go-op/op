default: ci

ci: fmt lint cover

ci-full: ci dependencies-analyze openapi-check check-all-modules lint-markdown bench

test: 
	go test ./... ./examples/petstore/...

cover:
	go test -coverprofile=coverage.out ./... ./examples/petstore/...
	go tool cover -func=coverage.out

check-all-modules:
	./check-all-modules.sh

cover-web: cover
	go tool cover -html=coverage.out

bench:
	go test -bench ./... -benchmem

build:
	go build -v ./... ./examples/petstore/...

dependencies-analyze:
	which govulncheck || go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

fmt:
	which gofumpt || go install mvdan.cc/gofumpt@latest
	gofumpt -l -w -extra .

FIX := "--fix"
lint:
	which golangci-lint || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	golangci-lint run ${FIX} ./...

lint-markdown:
	markdownlint ${FIX} --ignore documentation/node_modules --dot .

# Update golden files
golden-update:
	(cd examples/petstore/lib && go test -update)

# Check OpenAPI spec generated for the Petstore example. Uses https://github.com/daveshanley/vacuum
openapi-check:
	vacuum lint -d examples/petstore/lib/testdata/doc/openapi.json

# Examples
example:
	( cd examples/full-app-gourmet && go run . -debug )

example-watch:
	( cd examples/full-app-gourmet && air -- -debug )

petstore:
	( cd examples/petstore && go run . -debug )

# Documentation website
docs:
	go run golang.org/x/pkgsite/cmd/pkgsite@latest -http localhost:8084

docs-open:
	go run golang.org/x/pkgsite/cmd/pkgsite@latest -http localhost:8084 -open

.PHONY: docs-open docs example-watch example lint lint-markdown fmt ci ci-full
.PHONY: dependencies-analyze build bench cover-web cover test petstore check-all-modules
.PHONY: golden-update openapi-check
