# AGENTS.md

## Purpose

The CCX Notification Service detects new or changed OCP Advisor rule hits and notifies customers about them. It runs as a periodic CronJob (with `--instant-reports --verbose`) on OpenShift, not as a long-running deployment. Two separate CronJobs exist in production: one sends notifications via Kafka (`to-notification-backend`) and the other sends entries to the OCM ServiceLog API (`to-service-log`). Only one target can be active per deployment.

The service reads from the notification database (populated by `ccx-notification-writer` consuming from Kafka), compares new reports against previously reported ones using a cooldown mechanism, and sends notifications only for genuinely new issues that exceed the configured total risk threshold.

## Important Notes

- For broader context on the architecture, data flow, and notification targets, see the docs in `docs/` (particularly `architecture.md`, `data_flow.md`, `event_targets.md`, and `configuration.md`).
- **Do not read files under `docs/_outdated/`**. These contain outdated documentation that has not been corrected and will mislead you.

## Repository Structure

```text
ccx-notification-service/
├── cmd/ccx-notification-service/   # Entry point (main.go) and CLI flag parsing (cli.go)
├── conf/                           # Configuration loading, TOML parsing, Clowder integration
├── dashboards/                     # Grafana dashboard definitions
├── deploy/                         # ClowdApp deployment manifest (clowdapp.yaml)
├── differ/                         # Core business logic (see below)
├── docs/                           # Documentation (architecture, data flow, configuration, etc.)
│   └── _outdated/                  # Outdated docs moved here, do not use
├── ocmclient/                      # OCM (OpenShift Cluster Manager) client interface
├── producer/                       # Notification delivery implementations
│   ├── kafka/                      # Kafka producer (sends to platform.notifications.ingress)
│   ├── servicelog/                  # ServiceLog producer (REST calls to OCM API)
│   └── disabled/                   # No-op producer for testing
├── scripts/                        # Utility shell scripts
├── tests/                          # Test fixtures, mock configs, and generated mocks
├── tools/                          # Diagram generation scripts
├── types/                          # Shared type definitions (CliFlags, Report, ClusterEntry, etc.)
├── config.toml                     # Default configuration file
├── config-devel.toml               # Local development configuration
├── Makefile                        # Build, test, and lint targets
├── Dockerfile                      # Container image build
└── .golangci.yml                   # Golangci-lint configuration
```

## Key Packages

### `differ/` (core logic)

This is where most of the business logic lives. Key files and their responsibilities:

- **`differ.go`** — Main processing loop. `Run()` is the entry point called from `main()`. It creates a `Differ` instance via `New()`, then calls `start()` which runs the initialization sequence and processing loop. The `start()` function follows this order: register metrics, fetch rule content from content service, set up notification states and types from DB, read cluster list from `new_reports`, filter clusters, load previously reported records for cooldown, then call `ProcessClusters()`. If you need to add a new initialization step (e.g., connecting to a second database), it goes inside `start()` in the appropriate position. The per-cluster processing happens in `processReportsByCluster()`. The Kafka and ServiceLog paths have duplicated per-rule filtering logic in two different functions: `produceEntriesToKafka()` filters rules inline during message building, while `getReportsWithIssuesToNotify()` pre-filters all rules into a separate list for the ServiceLog path. Both are marked with `//TODO: Duplicated`.
- **`comparator.go`** — Cooldown and notification state logic. `ShouldNotify()` is called per-rule (not per-cluster) to determine whether a notification should be sent by comparing against previously reported records. `IssueNotInReport()` does the actual rule comparison by Type, Module, and ErrorKey. Also contains the `updateNotificationRecord*` functions that write processing outcomes to the `reported` table.
- **`storage.go`** — Database abstraction layer. Defines the `Storage` interface and its PostgreSQL implementation (`DBStorage`). All SQL queries live here, including `ReadClusterList()`, `ReadReportForClusterAtTime()`, `ReadLastNotifiedRecordForClusterList()` (the cooldown query), and `WriteNotificationRecordForCluster()`.
- **`content.go`** — Fetches rule content (titles, descriptions, impacts, tags) from the content service.
- **`renderer.go`** — Calls the template renderer service to produce rendered reports for ServiceLog entries. Only used in the ServiceLog path.
- **`cleaner.go`** — Database cleanup operations for old `new_reports` and `reported` records.
- **`cluster_filter.go`** — Filters clusters against allow/block lists from the processing configuration.
- **`metrics.go`** — Prometheus metrics definitions and push logic.
- **`errors.go`** — Custom error types for different failure modes (Kafka, ServiceLog, storage, etc.).

### Key concepts in the `differ` package

**Single database connection.** The service currently connects to a single database: the notification DB (shared with `ccx-notification-writer`). It does not connect to the aggregator database or any other database. All tables it reads from and writes to (`new_reports`, `reported`, `states`, `notification_types`, `read_errors`) are in this one database.

**The `new_reports` table.** This is the input queue for the service, populated by `ccx-notification-writer` consuming from Kafka. Each row contains `org_id`, `account_number`, `cluster` (UUID), `report` (full JSON with all rule hits), `updated_at`, and `kafka_offset`. When the service reads from this table, `ReadClusterList()` uses `SELECT DISTINCT ON (cluster) ... ORDER BY cluster, updated_at DESC`, which means only the most recent report per cluster is processed. Older entries for the same cluster are ignored. The table is not cleaned up during normal processing — cleanup is done by running the service with the `--new-reports-cleanup` flag (or `--cleanup-on-startup` to clean both `new_reports` and `reported` before processing).

**The `reported` table and its states.** Every time the service processes a cluster, it inserts a new row into the `reported` table (it never updates existing rows, the table is append-only). The primary key is `(org_id, cluster, notified_at)` where `notified_at` is set to `time.Now()` on each run, so each run produces a new row. The `state` column records what happened. The state IDs are read from the `states` table in the database at runtime via `ReadStates()`, but in practice they map to:

| State | Value   | Meaning |
|-------|---------|---------|
| 1     | `sent`  | Notification was delivered to the customer |
| 2     | `same`  | Skipped, no new issues compared to what was previously reported |
| 3     | `lower` | Skipped, all issues were below the total risk threshold |
| 4     | `error` | Notification delivery failed |

The `event_type_id` column tracks the notification target (1 = Kafka, 2 = ServiceLog, defined as constants in `types/types.go`). Kafka and ServiceLog cooldowns are tracked independently.

**The cooldown mechanism.** The service prevents notification spam by checking whether the customer was recently notified about the same issue. At startup, `ReadLastNotifiedRecordForClusterList()` loads the most recent **state=1 ("sent") only** record per cluster within the configured cooldown window. State=2/3/4 records are completely invisible to this query. For each rule in a new report, `ShouldNotify()` is called per-rule (not per-cluster). It deserializes the stored report from the last state=1 record and calls `IssueNotInReport()` to check if the rule (by Type, Module, and ErrorKey) was already reported. If found, the rule is skipped (still in cooldown). If not found or no state=1 record exists within the cooldown window, the rule is treated as new and the customer gets notified.

**The `report` column and its JSON format.** The `reported` table has a `report` column (varchar) that stores the full JSON report from `new_reports`. Currently this is always the unfiltered original report. `ShouldNotify()` uses this column for its per-rule comparison, which is why what gets stored here matters for future cooldown decisions. The report JSON uses a composite `rule_id|error_key` pipe-delimited format inside `reports[].rule_id` and a fully qualified module name in `reports[].component` (e.g., `ccx_rules_ocp.external.rules.test_rule.report`). However, the `rule_id` JSON field is **not deserialized** into the Go `ReportItem` struct — it is silently discarded during unmarshal. The Go struct uses `Module` (mapped from the `component` JSON field) and `ErrorKey` (mapped from the `key` JSON field) instead. The code derives rule names from the module using `moduleToRuleName()` and `ruleIDToRuleName()` helper functions in `differ.go`. Note that the aggregator database tables (`cluster_rule_toggle`, `rule_disable`) store `rule_id` and `error_key` as separate columns, so any code matching report rules against disabled rules needs to handle this format difference.

### `conf/`

Configuration loading using Viper and TOML. The `ConfigStruct` holds all configuration sections. Supports environment variable overrides with the `CCX_NOTIFICATION_SERVICE__` prefix (double underscore separating sections, e.g. `CCX_NOTIFICATION_SERVICE__STORAGE__PG_HOST`). Integrates with Clowder via `app-common-go` for Kafka and database configuration in deployed environments.

### `types/`

Shared type definitions used across packages: `CliFlags`, `ClusterEntry`, `Report`, `ReportContent`, `EvaluatedReportItem`, `NotificationMessage`, `ServiceLogEntry`, `EventTarget`, state and notification type enums, etc.

### `producer/`

Implements the `Producer` interface (`ProduceMessage` + `Close`). Three implementations: `kafka` (sends notification messages to the Kafka topic), `servicelog` (sends entries to the OCM ServiceLog REST API), and `disabled` (no-op, used when the producer is configured but not active).

## How to Build

```bash
make build # builds the binary
```

The binary is placed in the current directory as `ccx-notification-service`.

## How to Run Tests

### Unit tests

```bash
make test           # runs unit tests via unit-tests.sh with coverage
make coverage       # displays coverage on terminal
make benchmark      # runs benchmark tests
```

The unit tests use `stretchr/testify` for assertions and `DATA-DOG/go-sqlmock` for database mocking. Generated mocks (via `mockery`) are in `tests/mocks/`. To regenerate mocks after interface changes:

```bash
make gen-mocks
```

Coverage threshold is 73%, enforced in CI.

### BDD tests

BDD tests are maintained in the [insights-behavioral-spec](https://github.com/RedHatInsights/insights-behavioral-spec) repository. To run them:

1. Clone `insights-behavioral-spec`
2. Run `./notification_service_tests.sh` from that repo

### Linting and style checks

```bash
make style          # runs shellcheck + abcgo + golangci-lint
make lint           # runs golangci-lint only (via pre-commit)
make shellcheck     # runs shellcheck on shell scripts
make abcgo          # runs ABC metrics checker
make before_commit  # full pre-commit check: style + test + license + coverage
```

The `.golangci.yml` configures the following linters: errcheck, goconst, gocyclo, gosec, govet, ineffassign, nilerr, prealloc, revive, staticcheck, unconvert, unused, zerologlint. Formatters: gofmt, goimports.

## CLI Flags

The service is invoked with CLI flags defined in `cmd/ccx-notification-service/cli.go`:

| Flag | Default | Description |
|------|---------|-------------|
| `--instant-reports` | `false` | Run the notification processing loop (required for normal operation) |
| `--verbose` | `false` | Enable verbose logging |
| `--show-version` | `false` | Print version and exit |
| `--show-authors` | `false` | Print authors and exit |
| `--show-configuration` | `false` | Print loaded configuration and exit |
| `--print-new-reports-for-cleanup` | `false` | Display old records from `new_reports` eligible for cleanup |
| `--new-reports-cleanup` | `false` | Delete old records from `new_reports` |
| `--print-old-reports-for-cleanup` | `false` | Display old records from `reported` eligible for cleanup |
| `--old-reports-cleanup` | `false` | Delete old records from `reported` |
| `--cleanup-on-startup` | `false` | Run both cleanups before starting the differ |
| `--max-age` | `""` | Max age threshold for cleanup operations (PostgreSQL interval format) |

In production, the service runs as `./ccx-notification-service --instant-reports --verbose`.

## Configuration

Configuration is loaded from `config.toml` (overridable via `CCX_NOTIFICATION_SERVICE_CONFIG_FILE` env var). Each key can be overridden by environment variables with the `CCX_NOTIFICATION_SERVICE__` prefix.

Key sections in the config file:

| Section | What it configures |
|---------|-------------------|
| `[storage]` | PostgreSQL connection (notification database) |
| `[kafka_broker]` | Kafka producer settings, topic, thresholds, cooldown, event filter, tag filter |
| `[service_log]` | ServiceLog/OCM API settings, thresholds, cooldown, event filter, tag filter |
| `[dependencies]` | Content service and template renderer URLs |
| `[notifications]` | Advisor URLs for notification payloads |
| `[metrics]` | Prometheus push gateway settings |
| `[processing]` | Cluster allow/block list filtering |
| `[logging]` | Log level and debug mode |
| `[cleaner]` | Max age for cleanup operations |
| `[sentry]` | Sentry error tracking |
| `[cloudwatch]` | AWS CloudWatch logging |

Only one of `kafka_broker` or `service_log` can be enabled at a time. The service will refuse to start if both are enabled.

## Deployment

The service is deployed via Clowder on OpenShift. The `deploy/clowdapp.yaml` defines two CronJobs:

- **`to-notification-backend`** — sends notifications via Kafka to the notifications backend
- **`to-service-log`** — sends entries to the OCM ServiceLog API

Both run `./ccx-notification-service --instant-reports --verbose` with different environment variable configurations (one enables Kafka, the other enables ServiceLog). The database is shared with `ccx-notification-writer` via the Clowder `sharedDbAppName` mechanism.

## CI/CD

- **GitHub Actions** (`.github/workflows/`): Go tests with coverage, linters, BDD tests, auto-merge for bot PRs, GitHub Pages generation
- **Konflux/Tekton** (`.tekton/`): Container image builds on pull requests and pushes
- **Pre-commit hooks** (`.pre-commit-config.yaml`): End-of-file fixes, trailing whitespace, JSON/TOML/YAML validation, shellcheck, golangci-lint, ABC metrics, Go version consistency, Ruff (Python), Renovate config validation

## Code Conventions

- Always reference the existing code style
- Standard Go project layout with `cmd/` for the entry point
- All database queries in `differ/storage.go`
- Configuration via Viper with TOML files and environment variable overrides
- Zerolog for structured logging
- `export_test.go` files exist as a Go workaround to allow test files (which use the external `package differ_test`) to access unexported functions. They create public aliases for private symbols. If you add a new unexported function that needs testing, add an alias to the corresponding `export_test.go`.
- Generated mocks in `tests/mocks/` via `mockery` (do not edit manually)
- Pre-commit hooks enforce style on every commit. Run `make before_commit` before pushing.

## Pull Request Guidelines

Before creating a PR, run `make before_commit` to verify style, tests, license headers, and coverage. PRs require at least 2 approvals from team members. Keep commits focused and reference the Jira issue key in the PR title (e.g., `CCXDEV-12345: Add aggregator DB connection`).

Checklist before pushing:
- `make before_commit` passes
- New code has unit tests with meaningful coverage
- No merge conflicts with the default branch
- PR description explains what changed and why

## Related Repositories

| Repo | Relationship |
|------|-------------|
| [ccx-notification-writer](https://github.com/RedHatInsights/ccx-notification-writer) | Consumes from Kafka and populates the `new_reports` table that this service reads from. Shares the same database. Also owns the database migrations and schema definitions. |
| [insights-content-service](https://github.com/RedHatInsights/insights-content-service) | Provides rule content (titles, descriptions, impacts, tags) that this service fetches at startup to evaluate rules. |
| [insights-content-template-renderer](https://github.com/RedHatInsights/insights-content-template-renderer) | Renders report messages from templates for ServiceLog entries. Only used in the ServiceLog path. |
| [insights-behavioral-spec](https://github.com/RedHatInsights/insights-behavioral-spec) | BDD test suite covering the notification service behavior. Run `./notification_service_tests.sh` from that repo. |


## Team info and pipelines context

This service is part of the ObsInt (Observability Intelligence) Processing team's External Data Pipeline. The `/team-info` skill provides a reference for the whole team, covering repositories, services, data flow across internal and external pipelines, app-interface deployment, clusters, and related skills. If you need broader context about how this service fits in, invoke `/team-info`. If the skill is not installed, the source is at [processing-tools/skills/team-info/SKILL.md](https://github.com/RedHatInsights/processing-tools/blob/master/skills/team-info/SKILL.md).
