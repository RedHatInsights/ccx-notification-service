# ccx-notification-service
CCX notification service

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
