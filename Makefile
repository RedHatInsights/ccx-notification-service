SHELL := /bin/bash

.PHONY: default clean build build-tests fmt lint shellcheck abcgo style run test test-postgres cover integration_tests rest_api_tests sqlite_db license before_commit bdd_tests help godoc install_docgo install_addlicense

SOURCES:=$(shell find . -name '*.go')
BINARY:=ccx-notification-service
DOCFILES:=$(addprefix docs/packages/, $(addsuffix .html, $(basename ${SOURCES})))

default: build

clean: ## Run go clean
	@go clean

build: ${BINARY} ## Build binary containing service executable

build-cover:	${SOURCES}  ## Build binary with code coverage detection support
	./build.sh -cover

${BINARY}: ${SOURCES}
	./build.sh


install_golangci-lint:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

fmt: install_golangci-lint ## Run go formatting
	@echo "Running go formatting"
	golangci-lint fmt

lint: install_golangci-lint ## Run go liting
	@echo "Running go linting"
	golangci-lint run --fix

shellcheck: ## Run shellcheck
	./shellcheck.sh

abcgo: ## Run ABC metrics checker
	@echo "Run ABC metrics checker"
	./abcgo.sh ${VERBOSE}

style: fmt lint shellcheck abcgo ## Run all the formatting related commands (fmt, vet, lint, cyclo) + check shell scripts

run: ${BINARY} ## Build the project and executes the binary
	./$^

gen-mocks: ## Generates the mocks using mockery. Needs go >= 1.18
	go install github.com/vektra/mockery/v2@latest  
	mockery --all --output tests/mocks

test: ${BINARY} ## Run the unit tests
	./unit-tests.sh

build-test: gen-mocks ## Build native binary with unit tests and benchmarks
	go test -c

profiler: ${BINARY} ## Run the unit tests with profiler enabled
	./profile.sh

benchmark.txt:	benchmark

benchmark: ${BINARY} ## Run benchmarks
	go test -bench=. -run ^$ -v `go list ./... | grep -v tests | tr '\n' ' '` | tee benchmark.txt
	# go test -bench=. -run=^$ | tee benchmark.txt

benchmark.csv:	benchmark.txt ## Export benchmark results into CSV
	awk '/Benchmark/{count ++; gsub(/BenchmarkTest/,""); printf("%d,%s,%s,%s\n",count,$$1,$$2,$$3)}' $< > $@

cover: test ## Generate HTML pages with code coverage
	@go tool cover -html=coverage.out

coverage: ## Display code coverage on terminal
	@go tool cover -func=coverage.out

license: install_addlicense
	addlicense -c "Red Hat, Inc" -l "apache" -ignore "docs/**" -v ./

bdd_tests: ## Run BDD tests (needs real dependencies)
	@echo "Run BDD tests with real dependencies"
	cp ./config-devel.toml bdd_tests/config.toml
	pushd bdd_tests/ && ./run_tests.sh && popd

bdd_tests_mock: ## Run BDD tests with mocked dependencies
	@echo "Run BDD tests with mocked dependencies"
	cp ./config-devel.toml bdd_tests/config.toml
	pushd bdd_tests/ && WITHMOCK=1 ./run_tests.sh && popd

before_commit: style test test-postgres integration_tests license ## Checks done before commit
	./check_coverage.sh

help: ## Show this help screen
	@echo 'Usage: make <OPTIONS> ... <TARGETS>'
	@echo ''
	@echo 'Available targets are:'
	@echo ''
	@grep -E '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ''

function_list: ${BINARY} ## List all functions in generated binary file
	go tool objdump ${BINARY} | grep ^TEXT | sed "s/^TEXT\s//g"

docs/packages/%.html: %.go
	mkdir -p $(dir $@)
	docgo -outdir $(dir $@) $^
	addlicense -c "Red Hat, Inc" -l "apache" -v $@

godoc: export GO111MODULE=off
godoc: install_docgo install_addlicense ${DOCFILES} docs/sources.md

docs/sources.md: docs/sources.tmpl.md ${DOCFILES}
	./gen_sources_md.sh

install_docgo: export GO111MODULE=off
install_docgo:
	[[ `command -v docgo` ]] || go get -u github.com/dhconnelly/docgo

install_addlicense: export GO111MODULE=off
install_addlicense:
	[[ `command -v addlicense` ]] || GO111MODULE=off go get -u github.com/google/addlicense
