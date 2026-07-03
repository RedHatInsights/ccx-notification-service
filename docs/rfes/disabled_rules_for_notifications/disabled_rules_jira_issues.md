# Disabled Rules Notification Feature — Jira Issues

**Project**: CCXDEV
**Label**: `obsint-processing`
**Fix version**: 2026Q3

---

## Epic Description

### Summary

Customers can disable individual OCP Advisor rules (per-cluster or org-wide), but the notification service currently ignores these preferences and sends notifications anyway. This epic adds support for both disable mechanisms, with correct re-enable and cooldown behavior, across both Kafka notifications and ServiceLog entries.

### Why

This has been asked for by customers several times. Users who explicitly disable a rule expect to stop receiving notifications about it. The current behavior creates inconsistencies between systems which are supposed to be presenting the same data, therefore creating noise for customers. Fixing this closes a gap between the Advisor UI (where users manage their rules) and the notification pipeline (which doesn't check atm).

### Changes included as part of this epic

- Notification service changes to read disabled rules from the aggregator database and suppress notifications accordingly (6 implementation issues)
- Infrastructure configuration to give the notification service read access to the aggregator DB (2 issues)
- BDD behavioral specifications: 20 scenarios covering disable, re-enable, cooldown interaction, and per-rule granularity (4 step definition issues + 1 verification gate)
- User documentation handoff for Technical Writers (1 issue)
- IQE integration test planning (1 issue)

### Design document

- [Design doc](https://docs.google.com/document/d/1rXegLU9miDtdObHYTlcI_6yaRIaRpfM50BCyoHvyCwI/edit?tab=t.0#heading=h.6m8ya575lhb9)

The full design doc covers the current architecture, proposed changes, sequence diagrams, a step-by-step re-enable walkthrough, cooldown interaction analysis, and edge cases:

### BDD behavioral specifications

The expected behavior is defined in 20 Gherkin scenarios (10 for Kafka, 10 for ServiceLog). These are the acceptance tests for the feature:

- [Kafka scenarios](https://github.com/RedHatInsights/insights-behavioral-spec/blob/main/features/ccx-notification-service/notifications_disabled_rules.feature)
- [ServiceLog scenarios](https://github.com/RedHatInsights/insights-behavioral-spec/blob/main/features/ccx-notification-service/service_log_disabled_rules.feature)

### Repositories involved

| Repo | What changes |
|------|-------------|
| [ccx-notification-service](https://github.com/RedHatInsights/ccx-notification-service) | Core implementation: CLI flag, aggregator DB connection, disabled rules filtering, report omission |
| [insights-behavioral-spec](https://github.com/RedHatInsights/insights-behavioral-spec) | BDD step definitions and scenario verification |
| [app-interface](https://gitlab.cee.redhat.com/service/app-interface) | SaaS file update for aggregator DB access in stage/prod |
| [iqe-ccx-plugin](https://gitlab.cee.redhat.com/insights-qe/iqe-ccx-plugin) | IQE integration test planning |

### Agentic implementation approach

This epic is structured as an experiment in agentic software development. The issues are intentionally more granular and verbose than usual because they are designed to be picked up by AI agents with limited context and minimal human intervention.

This epic is aimed to explore agentic workflows, which are multi-agent orchestration pipelines where independent AI agents handle different phases of implementation in sequence. As part of this epic, the agent workflows will be run by engineers "locally". Each implementation issue goes through a minimum of three phases:

1. **Implement**: an agent reads the issue, the design doc, and the relevant code, then writes the production code
2. **Unit tests**: a separate, independent agent writes tests based on the design doc (not by reading the implementation), then runs them.
3. **Verify**: a third independent agent reviews the diff against the design doc, checks for code smells, verifies against BDD specs.

- Each agent only sees the previous agent's code output, not its reasoning, which means the verifying agent has to evaluate the code on its own rather than just gobbling the previous agent's summary. The human engineer reviews the final branch and creates the PR.

- Two reusable agentic workflow scripts are to be created/tested as part of this epic ([CCXDEV-16555](https://redhat.atlassian.net/browse/CCXDEV-16555) and [CCXDEV-16554](https://redhat.atlassian.net/browse/CCXDEV-16554)). The Go workflow ([CCXDEV-16555](https://redhat.atlassian.net/browse/CCXDEV-16555)) is built in ccx-notification-service but designed to be generic enough to adapt to our other Go repositories (aggregator, smart-proxy, content-service, parquet-factory, etc.) with minimal changes. The BDD workflow ([CCXDEV-16554](https://redhat.atlassian.net/browse/CCXDEV-16554)) is built in insights-behavioral-spec and reusable for any future feature that adds BDD scenarios.

- The more granular the issues, the more detailed the acceptance criteria, design doc references, and additional context for agents sections in each issue are is what gives the agents the best chance of producing correct implementations. The "Failure handling" sections try to limit the agents' behavior when it encounters ambiguity or errors, reducing wasted compute and hopefully making failures easier to diagnose.

- **Cost tracking.** Each agentic workflow logs its token usage (and **estimated cost**) at completion. This gives us a rough idea of what each issue costs. That said, the biggest lever for reducing costs is the quality of the specification itself, since well-defined issues with clear context reduce the number of round-trips an agent needs.


## [CCXDEV-16553](https://redhat.atlassian.net/browse/CCXDEV-16553): Create AGENTS.md in ccx-notification-service

**Labels**: `obsint-processing`, `repo:ccx-notification-service`

### Goal

Check whether ccx-notification-service has an AGENTS.md (or CLAUDE.md) that describes the repo structure, how to build, how to run tests, and coding conventions. If it exists, verify it's up to date. If not, create one.

This is a prerequisite for the agentic implementation workflow. Without it, every agent session starts by exploring the repo blind, which wastes tokens and leads to inconsistent results.

### Acceptance criteria

- [ ] AGENTS.md exists in ccx-notification-service repo root
- [ ] It covers: repo structure, how to build, how to run tests, code conventions, key packages (differ, conf, types)
- [ ] Covers repo structure with a brief description of each top-level directory
- [ ] Describes how to build the service (go build or make target)
- [ ] Describes how to run unit tests, including the correct make targets
- [ ] Describes the key packages: `differ` (main processing logic), `conf` (configuration), `types` (type definitions), `producer` (Kafka and ServiceLog producers)
- [ ] Mentions the `deploy/clowdapp.yaml` and its two CronJobs (`to-notification-backend` and `to-service-log`)
- [ ] Contains a note telling agents to ignore `docs/_outdated/`
- [ ] A fresh Claude session in the repo can answer "how do I run the unit tests?" without exploring

### Blocked by

None.

---

## [CCXDEV-16554](https://redhat.atlassian.net/browse/CCXDEV-16554): Build reusable agentic workflow for BDD step definition issues

**Labels**: `obsint-processing`, `repo:insights-behavioral-spec`

### Goal

A reusable agentic workflow for implementing Python BDD step definitions in insights-behavioral-spec. Similar structure to the Go workflow (issue [CCXDEV-16555](https://redhat.atlassian.net/browse/CCXDEV-16555) ) but adapted for this repo's patterns.

**Phases:**

**Phase 1: Implement.** Agent reads the Jira issue, the feature files that use the new steps, and existing step definitions for patterns. Writes the new step definitions in Python.

**Phase 2: Verify.** Independent agent checks that step names match exactly what the feature files expect, runs `make code-style` and `make ruff`, and verifies the steps are importable (no syntax errors or missing imports).

**Phase 3: Run the tests.** After all the verifying steps, the tests are run for the final time and reports the changes in a human-friendly way.

**Failure handling:**

* If a test fails after 3 fix attempts, stop and describe the failure rather than looping
* If the design doc is ambiguous, flag it rather than guessing
* If a dependency issue isn't merged yet, stop and say so rather than working around it
* Never silently skip a failing check

### Acceptance criteria

- [ ] Workflow script exists in `.claude/workflows/` in insights-behavioral-spec (or equivalent location)
- [ ] Two phases: implement, verify
- [ ] Step names are checked against the feature files
- [ ] Code style checks pass
- [ ] Token usage logged

### Blocked by

None.

---

## [CCXDEV-16555](https://redhat.atlassian.net/browse/CCXDEV-16555): Build reusable agentic workflow for Go implementation issues

**Labels**: `obsint-processing`, `repo:ccx-notification-service`

### Goal

A reusable agentic workflow script that any team member can invoke to implement a Go code change. The workflow is built and tested in ccx-notification-service first, but should be generic enough to work across our other Go repositories (aggregator, smart-proxy, content-service, parquet-factory, etc.) with minimal adaptation. The workflow orchestrates three independent agents in sequence. Each agent only sees the previous agent's output (the code diff), not its reasoning, so that verification is based on what the code actually does rather than on what the previous agent said it did.

The workflow takes a Jira issue reference and produces a ready-to-review branch with production code, unit tests, and a verification report.

**Phases:**

**Phase 1: Implement.** A single agent reads the Jira issue body, the relevant section of the design doc, the relevant BDD scenarios, and the existing code. It writes the production code only, no tests.

**Phase 2: Unit tests.** An independent agent reads the Jira issue body and the design doc (same source of truth). It reads the diff from Phase 1 to know what was built, then writes unit tests based on the design and acceptance criteria, not by reverse-engineering the implementation. It runs `go test`. Failures here are a signal that Phase 1 may have a bug, not something to work around.

**Phase 3: Verify.** An independent agent reads the design doc and the full diff (code + tests). It checks whether the implementation matches the design, flags deviations, and runs code style checks, ideally `make before_commit` which invokes all the other tools (`go vet`, `gofmt`, `gosec`, etc.). It produces a short verification report.

**Token tracking.** The workflow logs `budget.spent()` at the end and outputs total token usage and estimated cost. This gives us a rough idea of what each issue costs. That said, the biggest lever for reducing costs is the quality of the specification itself, since well-defined issues with clear context reduce the number of round-trips an agent needs.

**Failure handling:**

* If a test fails after 3 fix attempts, stop and describe the failure rather than looping
* If the design doc is ambiguous, flag it rather than guessing
* If a dependency issue isn't merged yet, stop and say so rather than working around it
* Never silently skip a failing check

**Dry run.** After building the workflow, test it on the "Register `--ignore-disabled-rules` CLI flag" issue. This is a trivial change that validates the workflow mechanics without risking a complex implementation.

### Acceptance criteria

- [ ] Workflow script exists in `.claude/workflows/` in ccx-notification-service
- [ ] Three phases execute in sequence: implement, unit tests, verify
- [ ] Phase 2 writes tests based on the design doc, not by reading Phase 1's code logic
- [ ] Phase 3 adversarially checks the diff against the design doc
- [ ] Token usage is logged at the end
- [ ] Tested on the CLI flag issue as a dry run
- [ ] Short "how to invoke" instructions included

### Blocked by

[CCXDEV-16553](https://redhat.atlassian.net/browse/CCXDEV-16553)


---

## [CCXDEV-16556](https://redhat.atlassian.net/browse/CCXDEV-16556): BDD steps — insert/update for cluster_rule_toggle

**Labels**: `obsint-processing`, `repo:insights-behavioral-spec`

### Goal

Implement Behave step definitions for managing rows in the `cluster_rule_toggle` table (in the aggregator DB). These steps are used by the disabled rules BDD scenarios.

Steps needed:

* `I insert the following rule in the cluster_rule_toggle table for the following cluster` (with table: org id, account number, cluster name, rule_id, error_key, disabled)
* `I update the following rule in the cluster_rule_toggle table for the following cluster` (same table format, sets disabled to a new value)

Note: the Gherkin table columns `org id`, `account number`, and `cluster name` are not direct columns in the `cluster_rule_toggle` table. The actual DB table uses `cluster_id`. The step implementation needs to use the cluster name from the Gherkin table as the `cluster_id` value when inserting into the DB. The `org id` and `account number` columns are contextual identifiers used by other steps in the scenario but may not be needed for the actual `cluster_rule_toggle` INSERT.

The `cluster_rule_toggle` table also has a `user_id NOT NULL` column and an `updated_at TIMESTAMP NOT NULL` column that are not in the Gherkin table. The `user_id` column is not checked anywhere in the notification service processing, so use any placeholder value. Use the current timestamp for `updated_at` in the step implementation. The nullable timestamp columns (`disabled_at`, `enabled_at`) can be left as NULL.

### Acceptance criteria

- [ ] Both step definitions are implemented and importable
- [ ] Steps parse the Gherkin table correctly (note: `org id` with space, not underscore, per repo convention)
- [ ] Insert step creates a row in `cluster_rule_toggle` in the aggregator DB, mapping `cluster name` to `cluster_id` and using a default `user_id`
- [ ] Update step modifies the `disabled` column for the matching row
- [ ] Code passes `make code-style` and `make ruff`

### Additional context

* Feature files using these steps: `notifications_disabled_rules.feature`, `service_log_disabled_rules.feature`
* The `cluster_rule_toggle` schema is defined in `features/steps/common_aggregator.py` (line 46). Columns: `cluster_id`, `rule_id`, `user_id`, `disabled`, `disabled_at`, `enabled_at`, `updated_at`, `error_key`
* Follow the pattern of existing step definitions in `features/steps/notification_database.py`
* Column names in Gherkin tables use spaces (`org id`, `account number`) not underscores

### Blocked by

[CCXDEV-16554](https://redhat.atlassian.net/browse/CCXDEV-16554)

---

## [CCXDEV-16557](https://redhat.atlassian.net/browse/CCXDEV-16557): BDD steps — insert/delete for rule_disable

**Labels**: `obsint-processing`, `repo:insights-behavioral-spec`

### Goal

Implement Behave step definitions for managing rows in the `rule_disable` table (in the aggregator DB).

Steps needed:

* `I insert the following rule ack in the rule_disable table` (with table: org id, user id, rule_id, error_key)
* `I delete the following rule ack from the rule_disable table` (with table: org id, rule_id, error_key)

The `rule_disable` table has no `disabled` flag. Presence of a row means disabled. Re-enabling means deleting the row.

### Acceptance criteria

- [ ] Both step definitions are implemented and importable
- [ ] Insert step creates a row in `rule_disable`
- [ ] Delete step removes the matching row
- [ ] Code passes `make code-style` and `make ruff`

### Additional context

* Feature files using these steps: `notifications_disabled_rules.feature`, `service_log_disabled_rules.feature`
* The `rule_disable` schema is defined in `features/steps/common_aggregator.py` (line 133)
* The delete step does not include `user_id` in its table (re-enabling removes the ack regardless of who created it)

### Blocked by

[CCXDEV-16554](https://redhat.atlassian.net/browse/CCXDEV-16554)

---

## [CCXDEV-16558](https://redhat.atlassian.net/browse/CCXDEV-16558): BDD steps — multi-rule report insertion

**Labels**: `obsint-processing`, `repo:insights-behavioral-spec`

### Goal

Implement Behave step definitions for inserting a report that contains multiple rules, and for confirming a report contains specific rules.

Steps needed:

* `I insert 1 report with critical and important total risk rules for the following clusters` (with table: org id, account number, cluster name)
* `I confirm that the report contains the following rule` / `I confirm that the report contains the following rules` / `I confirm that the reports contain the following rule` — three step text variations that should all route to the same function. The repo already uses this pattern: register multiple decorators on one function to handle singular/plural (see `notification_database.py` lines 149-150, 158-159, 195-196 for examples). The underlying function takes the rule_id table and verifies the listed rules exist in the report(s).

### Acceptance criteria

- [ ] Multi-rule report step creates a single `new_reports` entry with both `TEST_RULE_CRITICAL_IMPACT` and `TEST_RULE_IMPORTANT_IMPACT` rules in the JSON
- [ ] Plural confirm step checks all listed rules exist in the report
- [ ] Code passes `make code-style` and `make ruff`

### Additional context

* Feature files: the multi-rule scenario (scenario 10) in both feature files
* Existing single-rule insertion: `generate_report_with_risk()` in `features/steps/notification_database.py` (line 347) — extend this pattern for multi-rule reports
* The available test rules: `TEST_RULE_CRITICAL_IMPACT`, `TEST_RULE_IMPORTANT_IMPACT`, `TEST_RULE_MODERATE_IMPACT`, `TEST_RULE_LOW_IMPACT` (same file, lines 351-354)

### Blocked by

[CCXDEV-16554](https://redhat.atlassian.net/browse/CCXDEV-16554)

---

## [CCXDEV-16559](https://redhat.atlassian.net/browse/CCXDEV-16559): BDD steps — notification content and reported table assertions

**Labels**: `obsint-processing`, `repo:insights-behavioral-spec`

### Goal

Implement Behave step definitions for asserting notification content and reported table state.

Steps needed:

* `the notification event should contain only the following rules` (with table: rule_id) — verifies the Kafka notification message contains exactly the listed rules and no others
* `the reported table should have {n} row` / `the reported table should have {n} rows` — singular and plural variants routing to the same function (same pattern as the rest of the repo). Counts all rows in the `reported` table (no filtering by state or event_type_id).

### Acceptance criteria

- [ ] Notification content step reads the Kafka message and verifies the exact set of rule events
- [ ] Reported table step counts rows and asserts the expected number
- [ ] Code passes `make code-style` and `make ruff`

### Additional context

* Feature files: the multi-rule scenario (scenario 10) in `notifications_disabled_rules.feature` uses the notification content step (Kafka-only, not used in the ServiceLog feature file). All scenarios in both feature files use the reported table step.
* Existing Kafka assertion pattern: `count_notification_events_kafka` in `features/steps/notification_service.py`
* The reported table count is a simple `SELECT COUNT(*) FROM reported`

### Blocked by

[CCXDEV-16554](https://redhat.atlassian.net/browse/CCXDEV-16554)

---

## [CCXDEV-16560](https://redhat.atlassian.net/browse/CCXDEV-16560): Register --ignore-disabled-rules CLI flag

**Labels**: `obsint-processing`, `repo:ccx-notification-service`

### Goal

Add a new `--ignore-disabled-rules` boolean CLI flag (default `false`) to the notification service. When set to `true`, the service skips the aggregator DB connection and processes all rules as if none were disabled.

This flag exists for local development, BDD testing, and debugging. It has no behavioral effect yet (the disabled rules feature isn't implemented), but the flag needs to be registered first.

### Acceptance criteria

- [ ] `IgnoreDisabledRules` field added to `CliFlags` struct in `types/types.go`
- [ ] Flag registered in `setupCliFlags()` in `cmd/ccx-notification-service/cli.go`
- [ ] Flag is visible in `--help` output
- [ ] Unit test verifying default value is `false`
- [ ] Any other unit tests to cover the code in a meaningful way

### Additional context

* Design doc section: "New CLI flag: `--ignore-disabled-rules`"
* Files to modify: `types/types.go` (CliFlags struct), `cmd/ccx-notification-service/cli.go` (setupCliFlags function)
* Follow the pattern of existing flags like `--instant-reports` and `--verbose`

### Failure handling

* If unsure about the flag description text, use: "skip disabled rules check, process all rules as if none are disabled"

### Blocked by

[CCXDEV-16555](https://redhat.atlassian.net/browse/CCXDEV-16555)

---

## [CCXDEV-16561](https://redhat.atlassian.net/browse/CCXDEV-16561): Add AggregatorStorage configuration

**Labels**: `obsint-processing`, `repo:ccx-notification-service`

### Goal

Add a new `AggregatorStorage` field to the service configuration so it can connect to the aggregator database. This reuses the existing `StorageConfiguration` type. This issue only adds the configuration fields and parsing logic; the actual database connection is established in [CCXDEV-16562](https://redhat.atlassian.net/browse/CCXDEV-16562).

### Acceptance criteria

- [ ] `AggregatorStorage StorageConfiguration` field added to `ConfigStruct` in `conf/config.go`
- [ ] `showConfiguration()` in `cmd/ccx-notification-service/cli.go` prints the aggregator storage config
- [ ] Environment variables `CCX_NOTIFICATION_SERVICE__AGGREGATOR_STORAGE__*` are recognized
- [ ] Unit test verifying the config is loaded correctly
- [ ] Any other unit tests to cover the code in a meaningful way

### Additional context

* Design doc section: "New dependency: Aggregator DB connection"
* Files to modify: `conf/config.go` (ConfigStruct), `cmd/ccx-notification-service/cli.go` (showConfiguration)
* Follow the pattern of the existing `Storage` field in ConfigStruct

### Failure handling

* If Clowder integration for multiple databases is unclear, skip it and note it in the output. The ClowdApp configuration is handled in a separate issue.

### Blocked by

[CCXDEV-16555](https://redhat.atlassian.net/browse/CCXDEV-16555)

---

## [CCXDEV-16562](https://redhat.atlassian.net/browse/CCXDEV-16562): Establish aggregator DB connection at startup

**Labels**: `obsint-processing`, `repo:ccx-notification-service`

### Goal

As the very first step in the `start()` function, before reading the cluster list or fetching content, establish a short-lived read-only connection to the aggregator DB. If `--ignore-disabled-rules` is set, skip this step entirely. If the aggregator DB is unavailable, fail fast and exit.

The connection is opened, used for the queries in [CCXDEV-16564](https://redhat.atlassian.net/browse/CCXDEV-16564) and [CCXDEV-16565](https://redhat.atlassian.net/browse/CCXDEV-16565), and then closed immediately. The aggregator DB is not kept open during per-cluster processing.

### Acceptance criteria

- [ ] Aggregator DB connection is established before any other startup work
- [ ] Connection is closed after disabled rules are fetched (before the processing loop)
- [ ] If `--ignore-disabled-rules` is set, no connection attempt is made
- [ ] If the aggregator DB is unreachable, the service exits with an appropriate error
- [ ] Unit test for the skip behavior when `--ignore-disabled-rules` is true
- [ ] Any other unit tests to cover the code in a meaningful way

### Additional context

* Design doc section: "Change A: Fetch and filter disabled rules" (connection lifecycle subsection)
* Proposed flow diagram: `docs/disabled_rules_proposed_flow.png`
* Use `NewStorage()` from `differ/storage.go` with the `AggregatorStorage StorageConfiguration` field in `ConfigStruct` (added as part of the configuration work). This reuses the existing `Storage` interface and keeps the approach generic in case other aggregator DB queries are needed in the future.
* Insert the connection logic early in the `start()` function (`differ.go:864`), before the content fetch
* **Important:** the `start()` function currently does not receive `cliFlags`, so the `IgnoreDisabledRules` flag is not directly accessible inside it. The recommended approach is to add an `IgnoreDisabledRules bool` field to the `Differ` struct and set it in `Run()` before calling `start()`, so that `start()` can check `d.IgnoreDisabledRules` without changing its signature.

### Failure handling

* If `NewStorage()` does not work with the aggregator config for some reason, document the issue in the PR description rather than falling back to a raw `*sql.DB` connection. We want a consistent approach for the second DB.

### Blocked by

[CCXDEV-16560](https://redhat.atlassian.net/browse/CCXDEV-16560), [CCXDEV-16561](https://redhat.atlassian.net/browse/CCXDEV-16561)

---

## [CCXDEV-16563](https://redhat.atlassian.net/browse/CCXDEV-16563): Update ClowdApp for aggregator DB access

**Labels**: `obsint-processing`, `repo:ccx-notification-service`

### Goal

Add the aggregator database as a shared database reference in `deploy/clowdapp.yaml` so the notification service can connect to it in production. Both CronJobs (`to-notification-backend` and `to-service-log`) need access.

### Acceptance criteria

- [ ] Aggregator DB is referenced in the ClowdApp as a shared database
- [ ] Both CronJob specs have access to the aggregator DB connection
- [ ] The `--ignore-disabled-rules` flag is NOT set in either CronJob
- [ ] The service pod can resolve the aggregator DB connection using the Clowder-injected environment variables

### Additional context

* File to modify: `deploy/clowdapp.yaml`
* The current ClowdApp already references a shared database from `ccx-notification-writer` (line 16-17: `database: sharedDbAppName: ccx-notification-writer`). The aggregator DB needs a similar shared reference.
* Two CronJob specs exist: `to-notification-backend` (around line 141) and `to-service-log` (around line 289). Both need access.
* The aggregator DB is already used by other services (insights-results-aggregator), so a ClowdApp for it should already exist to reference.

### Failure handling

* If unsure about the Clowder shared database syntax, check how `ccx-notification-writer` is referenced in the current ClowdApp and follow the same pattern for the aggregator.

### Blocked by

[CCXDEV-16561](https://redhat.atlassian.net/browse/CCXDEV-16561)

---

## [CCXDEV-16564](https://redhat.atlassian.net/browse/CCXDEV-16564): Fetch per-cluster disabled rules from cluster_rule_toggle

**Labels**: `obsint-processing`, `repo:ccx-notification-service`

### Goal

Using the aggregator DB connection from [CCXDEV-16562](https://redhat.atlassian.net/browse/CCXDEV-16562), execute `SELECT cluster_id, rule_id, error_key FROM cluster_rule_toggle WHERE disabled = 1` and store the results in a hash map on the `Differ` struct, keyed by `(cluster_id, rule_id, error_key)`.

### Acceptance criteria

- [ ] New field on the `Differ` struct for the cluster-level disabled rules map
- [ ] Query executes at startup and populates the map
- [ ] Map is empty (not nil) when `--ignore-disabled-rules` is set
- [ ] Unit test with a mock DB verifying the map is populated correctly
- [ ] Unit test verifying an empty table results in an empty map
- [ ] Any other unit tests to cover the code in a meaningful way

### Additional context

* Design doc section: "Change A: Fetch and filter disabled rules"
* The `cluster_rule_toggle` schema has `rule_id` and `error_key` as separate columns
* Production data is around 35k rows total across both tables. This is roughly 4MB to transfer and 6MB in Go structs.
* BDD scenarios that test this: all scenarios in `notifications_disabled_rules.feature` that use `cluster_rule_toggle`

### Failure handling

* If unsure about the Go map key type, use a struct with three fields ideally referencing existing custom types. Do not concatenate strings with a delimiter.

### Blocked by

[CCXDEV-16562](https://redhat.atlassian.net/browse/CCXDEV-16562)

---

## [CCXDEV-16565](https://redhat.atlassian.net/browse/CCXDEV-16565): Fetch org-wide acked rules from rule_disable

**Labels**: `obsint-processing`, `repo:ccx-notification-service`

### Goal

Using the aggregator DB connection from [CCXDEV-16562](https://redhat.atlassian.net/browse/CCXDEV-16562), execute `SELECT org_id, rule_id, error_key FROM rule_disable` and store the results in a separate hash map on the `Differ` struct, keyed by `(org_id, rule_id, error_key)`.

This is a different map from [CCXDEV-16564](https://redhat.atlassian.net/browse/CCXDEV-16564) because the keys are different: cluster-level disables use `cluster_id`, org-wide acks use `org_id`.

### Acceptance criteria

- [ ] New field on the `Differ` struct for the org-level disabled rules map
- [ ] Query executes at startup and populates the map
- [ ] Map is empty (not nil) when `--ignore-disabled-rules` is set
- [ ] Unit test with a mock DB verifying the map is populated correctly
- [ ] Unit test verifying an empty table results in an empty map
- [ ] Any other unit tests to cover the code in a meaningful way

### Additional context

* Design doc section: "Change A: Fetch and filter disabled rules"
* The `rule_disable` table has no `disabled` flag. The presence of a row means the rule is disabled. All rows are fetched.
* BDD scenarios: all scenarios using `rule_disable` table

### Failure handling

* Same key type guidance as [CCXDEV-16564](https://redhat.atlassian.net/browse/CCXDEV-16564). Use a struct, not string concatenation.

### Blocked by

[CCXDEV-16562](https://redhat.atlassian.net/browse/CCXDEV-16562)

---

## [CCXDEV-16566](https://redhat.atlassian.net/browse/CCXDEV-16566): Update app-interface SaaS file for aggregator DB

**Labels**: `obsint-processing`, `repo:app-interface`

### Goal

Update the ccx-notification-service SaaS file in app-interface to configure the aggregator DB connection for both stage and production. Verify the connection works in an ephemeral environment.

### Acceptance criteria

- [ ] SaaS file updated with aggregator DB parameters for stage
- [ ] SaaS file updated with aggregator DB parameters for production
- [ ] Verified in ephemeral that the notification service can connect to the aggregator DB and fetch disabled rules

### Additional context

* SaaS file location: `data/services/insights/ccx-data-pipeline/external-data-pipeline/ccx-notification-service.yml`
* The aggregator DB is already used by `insights-results-aggregator` and `insights-results-db-writer`, so the shared DB reference should already exist in app-interface

### Blocked by

[CCXDEV-16563](https://redhat.atlassian.net/browse/CCXDEV-16563)

---

## [CCXDEV-16567](https://redhat.atlassian.net/browse/CCXDEV-16567): Filter disabled rules in the Kafka processing path

**Labels**: `obsint-processing`, `repo:ccx-notification-service`

### Goal

In the `produceEntriesToKafka` function, add a disabled rules check before the total risk filter. For each rule in the report, check both the cluster-level disabled rules map keyed by `(cluster_id, rule_id, error_key)` and the org-level acked rules map keyed by `(org_id, rule_id, error_key)`. If the rule is found in either map, skip it entirely so that a disabled rule never reaches the total risk filter or `ShouldNotify`.

The check requires parsing the composite `rule_id|error_key` format from the report JSON into separate `rule_id` and `error_key` values to match against the maps.

### Acceptance criteria

- [ ] Disabled check is the first thing evaluated for each rule in `produceEntriesToKafka`
- [ ] Both `cluster_rule_toggle` and `rule_disable` maps are checked
- [ ] A disabled rule never reaches `ShouldNotify`
- [ ] The composite `rule_id|error_key` format is correctly parsed for matching
- [ ] Unit test: rule present in cluster-level map is skipped
- [ ] Unit test: rule present in org-level map is skipped
- [ ] Unit test: rule not in either map proceeds to total risk filter
- [ ] Any other unit tests to cover the code in a meaningful way

### Additional context

* Design doc section: "Change A: Fetch and filter disabled rules" (per-rule processing loop subsection)
* Function to modify: `produceEntriesToKafka` in `differ/differ.go:459`
* The report JSON uses `rule_id|error_key` composite format in `reports[].rule_id` and module name in `reports[].component`. The maps use separate fields.
* BDD scenarios: all Kafka scenarios in `notifications_disabled_rules.feature`

### Failure handling

* If the rule_id parsing logic is unclear, look at `moduleToRuleName()` and `ruleIDToRuleName()` in `differ.go` for existing parsing patterns.

### Blocked by

[CCXDEV-16564](https://redhat.atlassian.net/browse/CCXDEV-16564), [CCXDEV-16565](https://redhat.atlassian.net/browse/CCXDEV-16565)

---

## [CCXDEV-16568](https://redhat.atlassian.net/browse/CCXDEV-16568): Filter disabled rules in the ServiceLog processing path

**Labels**: `obsint-processing`, `repo:ccx-notification-service`

### Goal

In the `getReportsWithIssuesToNotify` function (the ServiceLog path), add a disabled rules check before the total risk filter. For each rule in the report, check both the cluster-level disabled rules map keyed by `(cluster_id, rule_id, error_key)` and the org-level acked rules map keyed by `(org_id, rule_id, error_key)`. If the rule is found in either map, skip it entirely so it never reaches the total risk filter or `ShouldNotify`.

The check requires parsing the composite `rule_id|error_key` format from the report JSON into separate `rule_id` and `error_key` values to match against the maps.

The code has a `//TODO: Duplicated` comment at `differ.go:334` acknowledging that this function duplicates the filtering logic from `produceEntriesToKafka`. The disabled check should be added in the same position relative to the other filters.

### Acceptance criteria

- [ ] Disabled check is the first thing evaluated for each rule in `getReportsWithIssuesToNotify`
- [ ] Both `cluster_rule_toggle` and `rule_disable` maps are checked
- [ ] A disabled rule never reaches `ShouldNotify`
- [ ] The composite `rule_id|error_key` format is correctly parsed for matching
- [ ] Unit test: disabled rule is excluded from the returned `reportsWithIssues`
- [ ] Unit test: active rule is included
- [ ] Any other unit tests to cover the code in a meaningful way

### Additional context

* Design doc section: "Change A: Fetch and filter disabled rules" (per-rule processing loop subsection)
* Function to modify: `getReportsWithIssuesToNotify` in `differ/differ.go:320`
* The `//TODO: Duplicated` comment at line 334 marks where the duplicated logic starts
* BDD scenarios: all ServiceLog scenarios in `service_log_disabled_rules.feature`

### Failure handling

* Consider whether the disabled check logic can be extracted into a shared helper used by both this function and `produceEntriesToKafka`. If so, do it. If the refactor feels risky, add the check inline and note the duplication.

### Blocked by

[CCXDEV-16564](https://redhat.atlassian.net/browse/CCXDEV-16564), [CCXDEV-16565](https://redhat.atlassian.net/browse/CCXDEV-16565)

---

## [CCXDEV-16569](https://redhat.atlassian.net/browse/CCXDEV-16569): Omit disabled rules from stored reports

**Labels**: `obsint-processing`, `repo:ccx-notification-service`

### Goal

Before writing to the `reported` table, filter the report JSON to remove disabled rules. When disabled rules are omitted from stored reports, re-enable detection works automatically because `IssueNotInReport` will treat the re-enabled rule as new when it compares against the stored report.

Currently, the full original report from `new_reports` is passed unchanged to `WriteNotificationRecordForCluster`. Instead of passing the raw report, the implementation should deserialize the report JSON, remove entries that match the disabled rules maps, re-serialize it, and pass the filtered version.

When a rule is later re-enabled, it will not be in the stored report, so `IssueNotInReport` will return true, treat it as a new issue, and send a notification to the customer. No changes to `ShouldNotify` or the cooldown logic are needed.

### Acceptance criteria

- [ ] The `report` column in `reported` contains only active (non-disabled) rules
- [ ] Both `cluster_rule_toggle` and `rule_disable` maps are used for filtering
- [ ] The composite `rule_id|error_key` format is correctly parsed
- [ ] When `--ignore-disabled-rules` is set (maps are empty), the report is stored unfiltered (existing behavior)
- [ ] Unit test: report with one disabled and one active rule stores only the active rule
- [ ] Unit test: report with all rules disabled stores an empty reports array
- [ ] Unit test: report with no disabled rules is stored unchanged
- [ ] Any other unit tests to cover the code in a meaningful way

### Additional context

* Design doc sections: "Change B: Omit disabled rules from stored reports" and "How Re-enabling Works" (section 6, the full step-by-step walkthrough)
* The report is of type `types.ClusterReport` (a string). Deserialize to `types.Report`, filter `Report.Reports`, re-serialize.
* **Kafka path**: the `report` variable is a parameter of `produceEntriesToKafka` (`differ.go:459`). Filter it before it reaches `updateNotificationRecordSameState` (line 511) or `updateNotificationRecordSentState` (line 537). The filtering can happen at the top of this function.
* **ServiceLog path**: the `report` variable is defined in `processReportsByCluster` (`differ.go:583`) and passed to `updateNotificationRecordState` (line 639). The filtering must happen in `processReportsByCluster` between reading the report and calling `updateNotificationRecordState`. It cannot happen inside `ProduceEntriesToServiceLog` because that function does not receive or return the report variable.
* BDD scenarios: the enable scenarios (4, 5), the re-enable scenarios (6, 7, 8, 9), and the multi-rule scenario (10)

### Failure handling

* The cooldown query reads state=1 records only. State=2 records are invisible to it. If this is confusing, re-read design doc section 7 (Cooldown Interaction) before implementing.

### Blocked by

[CCXDEV-16567](https://redhat.atlassian.net/browse/CCXDEV-16567), [CCXDEV-16568](https://redhat.atlassian.net/browse/CCXDEV-16568)

---

## [CCXDEV-16570](https://redhat.atlassian.net/browse/CCXDEV-16570): Remove @skip tags and verify all 20 BDD scenarios pass

**Labels**: `obsint-processing`, `repo:insights-behavioral-spec`, `repo:ccx-notification-service`

### Goal

This issue serves as the validation gate for the entire disabled rules feature, confirming that the deployed implementation matches the expected behavior. Remove the `@skip` tag from both disabled rules feature files and run all 20 scenarios against the fully implemented notification service.

Both the service implementation ([CCXDEV-16560](https://redhat.atlassian.net/browse/CCXDEV-16560) through [CCXDEV-16569](https://redhat.atlassian.net/browse/CCXDEV-16569)) and the BDD step definitions ([CCXDEV-16556](https://redhat.atlassian.net/browse/CCXDEV-16556) through [CCXDEV-16559](https://redhat.atlassian.net/browse/CCXDEV-16559)) must be complete. The BDD tests run the actual notification service binary against real PostgreSQL and Kafka instances (via Docker), with only external dependencies like the content service and service-log API being mocked. They also run as part of our CI pipelines where the infrastructure is spawned temporarily. Production deployment ([CCXDEV-16566](https://redhat.atlassian.net/browse/CCXDEV-16566)) is not required for this step.

### Acceptance criteria

- [ ] `@skip` tag removed from `notifications_disabled_rules.feature`
- [ ] `@skip` tag removed from `service_log_disabled_rules.feature`
- [ ] All 10 Kafka scenarios pass
- [ ] All 10 ServiceLog scenarios pass
- [ ] `make update-scenarios` run to update the scenario lists
- [ ] `make before_commit` passes
- [ ] the BDD tests running on the PR CI pass

### Blocked by

[CCXDEV-16569](https://redhat.atlassian.net/browse/CCXDEV-16569), [CCXDEV-16556](https://redhat.atlassian.net/browse/CCXDEV-16556), [CCXDEV-16557](https://redhat.atlassian.net/browse/CCXDEV-16557), [CCXDEV-16558](https://redhat.atlassian.net/browse/CCXDEV-16558), [CCXDEV-16559](https://redhat.atlassian.net/browse/CCXDEV-16559)

---

## [CCXDEV-16571](https://redhat.atlassian.net/browse/CCXDEV-16571): Write user-facing handoff document for the Docs team

**Labels**: `obsint-processing`, `repo:ccx-notification-service`

### Goal

Write a plain-language document describing the disabled rules feature from the customer's perspective. This document is handed off to Michelle Purcell (Technical Writer team point of contact) for incorporation into the OpenShift user documentation.

The document should cover:

* What changed: notifications now respect disabled rules
* The two ways to disable a rule (per-cluster and org-wide ack) and how each affects notifications
* Re-enable behavior: what happens when a customer re-enables a rule
* Cooldown interaction: re-enabling within cooldown does not trigger a new notification

No technical internals, no code references, no database schemas. Write it from the perspective of a customer using OCP Advisor.

### Acceptance criteria

- [ ] Document written in plain language suitable for user documentation
- [ ] Covers both disable mechanisms (per-cluster and org-wide)
- [ ] Explains re-enable behavior clearly
- [ ] Explains cooldown interaction in user-friendly terms
- [ ] Reviewed by at least one team member before handoff
- [ ] Shared with Michelle Purcell

### Blocked by

[CCXDEV-16569](https://redhat.atlassian.net/browse/CCXDEV-16569)

---

## [CCXDEV-16572](https://redhat.atlassian.net/browse/CCXDEV-16572): Plan and create IQE test tasks for disabled rules

**Labels**: `obsint-processing`, `repo:iqe-ccx-plugin`

###

Goal

This is a planning and research task. The goal is to figure out what IQE integration tests we need for the disabled rules feature and create Jira issues for each one.

**New feature description:** Customers using OCP Advisor ([https://console.redhat.com/openshift/advisor/](https://console.redhat.com/openshift/advisor/)) can disable individual rules they don't want to hear about. There are two ways to do this: they can disable a rule for a specific cluster (per-cluster disable), or they can acknowledge a rule across their entire organization (org-wide ack). Until now, the notification service ignored these preferences and sent notifications anyway.

This feature makes the notification service check which rules are disabled before sending notifications, so customers only get notified about rules they actually care about. If a customer later re-enables a rule, the notification service picks it up again and starts notifying them about it, as long as the cooldown period has passed.

The full design doc (linked in the epic) goes into much more detail about the internal mechanics, but the key idea is straightforward: before sending a notification, check if the rule is disabled, and if it is, skip it.

**IQE vs BDD tests** We already have 20 BDD scenarios that define the expected behavior (see the BDD feature files linked below). The BDD tests run the actual notification service binary against real PostgreSQL and Kafka (via Docker), with only external dependencies like the content service and service-log API being mocked. They also run as part of our CI pipelines where the infrastructure is spawned temporarily. IQE tests go a step further because they run against fully deployed environments (stage, ephemeral, production) using the actual smart-proxy API to disable rules and verify notification behavior end-to-end. The test patterns and assertions will look different from the BDD scenarios, but the behaviors to cover should be similar.

The `iqe-ccx-plugin` repo has an `AGENTS.md` file that explains the test framework, the repo structure, and how to run tests. There are also existing notification tests at `iqe_ccx/tests/test_notifications.py` that are good examples to follow.

**Suggested approach to learn about agentic development:**

Feel free to adapt it to whatever works best for you.

1. Start by cloning both `iqe-ccx-plugin` and `insights-behavioral-spec` repos locally if you haven't already.
2. Open a Claude Code session in the `iqe-ccx-plugin` repo. The `AGENTS.md` there will give Claude a good understanding of the project.
3. Before diving into the planning, it's a good idea to use Claude Code's `/teach` skill to get familiar with the notification service and the disabled rules design. Just type `/teach` and point it at the design doc. It will walk you through the concepts interactively at your own pace and create reference materials you can come back to later. This is especially valuable if the notification service codebase or the disabled rules design is new to you, since there's a lot of domain-specific context (cooldown behavior, state management, report omission) that's much easier to absorb through a guided walkthrough than by reading the design doc cold. Note: `/teach` and other custom skills require the Claude Code CLI. If you're using Cursor or another IDE, open the integrated terminal and run `claude` from there.
4. Once you feel comfortable with the feature, use Claude Code's `/goal` command to have Claude read and cross-reference these files together:

    * The design doc: `TBD`
    * The BDD feature files: `<your-local-path>/insights-behavioral-spec/features/ccx-notification-service/notifications_disabled_rules.feature` and `service_log_disabled_rules.feature`
    * The existing IQE notification tests: `iqe_ccx/tests/test_notifications.py`

5. Ask Claude to propose which IQE tests should be added or modified, and go through its suggestions together. Some BDD scenarios will map directly to IQE tests, while others might need a different approach because of how the real environments work.
6. Once you're happy with the first draft (it’s expected there will be multiple revisions, so let’s start and iterate), you can create follow-up Jira issues for the actual test implementations. Each issue might reference any relevant BDD scenarios or design doc sections so that whoever picks it up has the full context.

Don't hesitate to ask the team if anything about the notification service or the disabled rules behavior is unclear. The design doc is thorough (too thorough, mostly for the agents' sake) so it's easier to get the info from real people…

### Acceptance criteria

- [ ] Follow-up Jira issues created for IQE test implementation
- [ ] Each follow-up issue references the relevant BDD scenario and design doc section

**Totally optional**

- [ ] Notes on which BDD scenarios map cleanly to IQE and which ones need a different testing approach in a real environment
- [ ] If you tried `/teach` or `/goal`, a simple summary of your experience would be nice

### Blocked by

[CCXDEV-16569](https://redhat.atlassian.net/browse/CCXDEV-16569)
