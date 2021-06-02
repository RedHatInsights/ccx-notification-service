# ccx-notification-service
CCX notification service

[![GoDoc](https://godoc.org/github.com/RedHatInsights/ccx-notification-service?status.svg)](https://godoc.org/github.com/RedHatInsights/ccx-notification-service)
[![GitHub Pages](https://img.shields.io/badge/%20-GitHub%20Pages-informational)](https://redhatinsights.github.io/ccx-notification-service/)
[![Go Report Card](https://goreportcard.com/badge/github.com/RedHatInsights/ccx-notification-service)](https://goreportcard.com/report/github.com/RedHatInsights/ccx-notification-service)
[![Build Status](https://travis-ci.com/RedHatInsights/ccx-notification-service.svg?branch=master)](https://travis-ci.com/RedHatInsights/ccx-notification-service)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/RedHatInsights/ccx-notification-service)
[![License](https://img.shields.io/badge/license-Apache-blue)](https://github.com/RedHatInsights/ccx-notification-service/blob/master/LICENSE)

## Building

Use `make build` to build executable file with this service.

All Makefile targets:

```
Usage: make <OPTIONS> ... <TARGETS>

Available targets are:

clean                Run go clean
build                Keep this rule for compatibility
fmt                  Run go fmt -w for all sources
lint                 Run golint
vet                  Run go vet. Report likely mistakes in source code
cyclo                Run gocyclo
ineffassign          Run ineffassign checker
shellcheck           Run shellcheck
errcheck             Run errcheck
goconst              Run goconst checker
gosec                Run gosec checker
abcgo                Run ABC metrics checker
json-check           Check all JSONs for basic syntax
style                Run all the formatting related commands (fmt, vet, lint, cyclo) + check shell scripts
run                  Build the project and executes the binary
test                 Run the unit tests
bdd_tests            Run BDD tests
before_commit        Checks done before commit
help                 Show this help screen
```
