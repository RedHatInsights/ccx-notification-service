SHELL := /bin/bash

.PHONY: default clean build fmt lint vet cyclo ineffassign shellcheck errcheck goconst gosec abcgo style run test test-postgres cover integration_tests rest_api_tests sqlite_db license before_commit bdd_tests help godoc install_docgo install_addlicense

SOURCES:=$(shell find . -name '*.go')
BINARY:=ccx-notification-service
DOCFILES:=$(addprefix docs/packages/, $(addsuffix .html, $(basename ${SOURCES})))

default: build

clean: ## Run go clean
	@go clean

build: ${BINARY} ## Keep this rule for compatibility

${BINARY}: ${SOURCES}
	./build.sh

fmt: ## Run go fmt -w for all sources
	@echo "Running go formatting"
	./gofmt.sh

lint: ## Run golint
	@echo "Running go lint"
	./golint.sh

vet: ## Run go vet. Report likely mistakes in source code
	@echo "Running go vet"
	./govet.sh

cyclo: ## Run gocyclo
	@echo "Running gocyclo"
	./gocyclo.sh

ineffassign: ## Run ineffassign checker
	@echo "Running ineffassign checker"
	./ineffassign.sh

shellcheck: ## Run shellcheck
	./shellcheck.sh

errcheck: ## Run errcheck
	@echo "Running errcheck"
	./goerrcheck.sh

goconst: ## Run goconst checker
	@echo "Running goconst checker"
	./goconst.sh ${VERBOSE}

gosec: ## Run gosec checker
	@echo "Running gosec checker"
	./gosec.sh ${VERBOSE}

abcgo: ## Run ABC metrics checker
	@echo "Run ABC metrics checker"
	./abcgo.sh ${VERBOSE}

style: fmt vet lint cyclo shellcheck errcheck goconst gosec ineffassign abcgo ## Run all the formatting related commands (fmt, vet, lint, cyclo) + check shell scripts

run: ${BINARY} ## Build the project and executes the binary
	./$^

gen-mocks: ## Generates the mocks using mockery. Needs go >= 1.18
	go install github.com/vektra/mockery/v2@latest  
	mockery --all --output tests/mocks

test: ${BINARY} ## Run the unit tests
	./unit-tests.sh

profiler: ${BINARY} ## Run the unit tests with profiler enabled
	./profile.sh

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

docs/packages/%.html: %.go
	mkdir -p $(dir $@)
	docgo -outdir $(dir $@) $^
	addlicense -c "Red Hat, Inc" -l "apache" -v $@

godoc: export GO111MODULE=off
godoc: install_docgo install_addlicense ${DOCFILES}

install_docgo: export GO111MODULE=off
install_docgo:
	[[ `command -v docgo` ]] || go get -u github.com/dhconnelly/docgo

install_addlicense: export GO111MODULE=off
install_addlicense:
	[[ `command -v addlicense` ]] || GO111MODULE=off go get -u github.com/google/addlicense
