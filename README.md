# ccx-notification-service
CCX Notification Service


[![forthebadge made-with-go](http://ForTheBadge.com/images/badges/made-with-go.svg)](https://go.dev/)

[![GoDoc](https://godoc.org/github.com/RedHatInsights/ccx-notification-service?status.svg)](https://godoc.org/github.com/RedHatInsights/ccx-notification-service)
[![GitHub Pages](https://img.shields.io/badge/%20-GitHub%20Pages-informational)](https://redhatinsights.github.io/ccx-notification-service/)
[![Go Report Card](https://goreportcard.com/badge/github.com/RedHatInsights/ccx-notification-service)](https://goreportcard.com/report/github.com/RedHatInsights/ccx-notification-service)
[![Build Status](https://ci.ext.devshift.net/buildStatus/icon?job=RedHatInsights-ccx-notification-service-gh-build-master)](https://ci.ext.devshift.net/job/RedHatInsights-ccx-notification-service-gh-build-master/)
[![Build Status](https://travis-ci.com/RedHatInsights/ccx-notification-service.svg?branch=master)](https://travis-ci.com/RedHatInsights/ccx-notification-service)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/RedHatInsights/ccx-notification-service)
[![License](https://img.shields.io/badge/license-Apache-blue)](https://github.com/RedHatInsights/ccx-notification-service/blob/master/LICENSE)

<!-- vim-markdown-toc GFM -->

* [Description](#description)
    * [Architecture](#architecture)
* [Building](#building)
    * [Makefile targets](#makefile-targets)
* [Configuration](#configuration)
* [Usage](#usage)
    * [All command line options](#all-command-line-options)
* [Database](#database)
    * [Schema description](#schema-description)
* [Notification templates](#notification-templates)
* [Definition of Done for new features and fixes](#definition-of-done-for-new-features-and-fixes)
* [Testing](#testing)
* [BDD tests](#bdd-tests)
* [Package manifest](#package-manifest)

<!-- vim-markdown-toc -->

## Description

The purpose of this service is to enable sending automatic email notifications
and ServiceLog events to users for all serious issues found in their OpenShift
clusters. The "instant" mode of this service runs as a cronjob every fifteen
minutes, and it sends a sequence of events to the configured Kafka topic so
that the
[notification-backend](https://github.com/RedHatInsights/notifications-backend)
can process them and create email notifications based on the provided events.
Additionally ServiceLog events are created, these can be displayed on cluster
pages. Currently the events are only created for the **important** and
**critical** issues found in the `new_reports` table of the configured
PostgreSQL database. Once the reports are processed, the DB is updated with
info about sent events by populating the `reported` table with the
corresponding information. For more info about initialising the database and
perform migrations, take a look at the [ccx-notification-writer
repository](https://github.com/RedHatInsights/ccx-notification-writer).

In the instant notification mode, one email will be received for each cluster
with important or critical issues.

Additionally this service exposes several metrics about consumed and
processed messages. These metrics can be aggregated by Prometheus and
displayed by Grafana tools.

### Architecture

Overall architecture and integration of this service is described
[in this document](https://redhatinsights.github.io/ccx-notification-service/architecture.html)

## Building

Use `make build` to build executable file with this service.

### Makefile targets

All Makefile targets:

```
Usage: make <OPTIONS> ... <TARGETS>

Available targets are:

clean                Run go clean
build                Build binary containing service executable
build-cover          Build binary with code coverage detection support
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
build-test           Build native binary with unit tests and benchmarks
profiler             Run the unit tests with profiler enabled
benchmark            Run benchmarks
benchmark.csv        Export benchmark results into CSV
cover                Generate HTML pages with code coverage
coverage             Display code coverage on terminal
bdd_tests            Run BDD tests (needs real dependencies)
bdd_tests_mock       Run BDD tests with mocked dependencies
before_commit        Checks done before commit
function_list        List all functions in generated binary file
help                 Show this help screen
```

## Configuration

Configuration is described
[in this document](https://redhatinsights.github.io/ccx-notification-service/configuration.html)

## Usage

Provided a valid configuration, you can start the service with `./ccx-notification-service --instant-reports` 

### All command line options

List of all available command line options:

```
  -instant-reports
        create instant reports
  -cleanup-on-startup
        perform database clean up on startup
  -show-authors
        show authors and exit
  -show-configuration
        show configuration
  -show-version
        show version and exit
  -max-age string
        max age for displaying/cleaning old records
  -new-reports-cleanup
        perform new reports clean up
  -old-reports-cleanup
        perform old reports clean up
  -print-new-reports-for-cleanup
        print new reports to be cleaned up
  -print-old-reports-for-cleanup
        print old reports to be cleaned up
```

## Database

PostgreSQL database is used as a storage.

Please look [at detailed schema
description](https://redhatinsights.github.io/ccx-notification-service/db-description/)
for more details about tables, indexes, and keys defined in this database.

### Schema description

DB schema description can be generated by `generate_db_schema_doc.sh` script.
Output is written into directory `docs/db-description/`. Its content can be
viewed [at this
address](https://redhatinsights.github.io/ccx-notification-service/db-description/).

## Notification templates

Notification templates used to send e-mails etc. to customers are stored in different repository:
[https://github.com/RedHatInsights/notifications-backend/](https://github.com/RedHatInsights/notifications-backend/)

Templates used by this notification service are available at:
[https://github.com/RedHatInsights/notifications-backend/tree/master/backend/src/main/resources/templates/AdvisorOpenshift](https://github.com/RedHatInsights/notifications-backend/tree/master/engine/src/main/resources/templates/AdvisorOpenshift)

## Definition of Done for new features and fixes

Please look at [DoD.md](DoD.md) document for definition of done for new features and fixes.


## Testing

Tests and its configuration is described [in this document](https://redhatinsights.github.io/ccx-notification-service/testing.html)



## BDD tests

Behaviour tests for this service are included in [Insights Behavioral
Spec](https://github.com/RedHatInsights/insights-behavioral-spec) repository.
In order to run these tests, the following steps need to be made:

1. clone the [Insights Behavioral Spec](https://github.com/RedHatInsights/insights-behavioral-spec) repository
1. go into the cloned subdirectory `insights-behavioral-spec`
1. run the `notification_service_tests.sh` from this subdirectory

List of all test scenarios prepared for this service is available at
<https://redhatinsights.github.io/insights-behavioral-spec/feature_list.html#ccx-notification-service>



## Package manifest

Package manifest is available at [docs/manifest.txt](docs/manifest.txt).
