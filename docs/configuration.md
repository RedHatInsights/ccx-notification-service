---
layout: page
nav_order: 3
---
# Configuration
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

Configuration is done by toml config, default one is `config.toml` in working
directory, but it can be overwritten by `NOTIFICATION_SERVICE_CONFIG_FILE`
environment variable.

Also each key in configuration file can be overwritten by corresponding
environment variable. For example if you have the following configuration:

```toml
[storage]
db_driver = "sqlite3"
sqlite_datasource = "./aggregator.db"
pg_username = "user"
pg_password = "password"
pg_host = "localhost"
pg_port = 5432
pg_db_name = "aggregator"
pg_params = ""
```

and environment variables

```shell
NOTIFICATION_SERVICE__STORAGE__DB_DRIVER="postgres"
NOTIFICATION_SERVICE__STORAGE__PG_PASSWORD="your secret password"
```

the actual driver will be postgres with password "your secret password"

It's very useful for deploying docker containers and keeping some of your configuration
outside of main configuration file(like passwords).

### Clowder configuration

In Clowder environment, some configuration options are injected automatically.
Currently Kafka broker configuration is injected this side. To test this
behavior, it is possible to specify path to Clowder-related configuration file
via `AGG_CONFIG` environment variable:

```
export ACG_CONFIG="clowder_config.json"
```

## Service Log configuration

Service Log configuration is in section `[service-log]` in configuration file

```
[service_log]
enabled = false
client_id = "CLIENT_ID"
client_secret = "CLIENT_SECRET"
token_url = ""
url = "https://api.openshift.com/api/service_logs/v1/cluster_logs/"
timeout = "15s"
likelihood_threshold = 0
impact_threshold = 0
severity_threshold = 0
total_risk_threshold = 3
event_filter = "totalRisk > totalRiskThreshold"
rule_details_uri = "https://console.redhat.com/openshift/insights/advisor/recommendations/{module}|{error_key}"
tag_filter_enabled = false
tags = ["osd_customer"]
```

- `enabled` determines whether the notifications service sends messages to Service Log
- `client_id` is a client ID used for access token retrieval
- `client_secret` is a client secret used for access token retrieval
- `token_url` is a token refreshment API endpoint (optional, otherwise set to default one)
- `timeout` is a time used as a timeout when sending requests to Service Log API
- `likelihood_threshold`,`impact_threshold`, `severity_threshold` and `total_risk_threshold` are values which can be used in `event_filter` for filtering messages sent to Service Log
- `event_filter` is a condition string used to determine which messages will be sent to Service Log
- `rule_details_uri` URI to a page with detailed information about rule. Please note that it is not a true URI, but a template to be interpolated with real module name and error key
- `tag_filter_enabled` is set to `true` if filtering by rule tag should be performed
- `tags` contains a list of tags used by filter (if enabled). Empty list is supported.

Please note that for correct functionality of Service Log integration, `dependencies` configuration should be also present.

## Dependencies configuration

Dependencies configuration is in section `[dependencies]` in configuration file

```
[dependencies]
content_server = "localhost:8082" #provide in deployment env or as secret
content_endpoint = "/api/v1/content" #provide in deployment env or as secret
template_renderer_server = "localhost:8083" #provide in deployment env or as secret
template_renderer_endpoint = "/rendered_reports" #provide in deployment env or as secret
```

- `content_server` is an address of the [Content service API](https://github.com/RedHatInsights/insights-content-service)
- `content_endpoint` is a REST API path to for retrieval of rule content
- `template_renderer_server` is an address of the Content template renderer
- `template_renderer_endpoint` is a REST API path for rendering content templates based on report details

## Processing configuration

Processing configuration is in section `[processing]` in configuration file

```
[processing]
filter_allowed_clusters = true
allowed_clusters = ["34c3ecc5-624a-49a5-bab8-4fdc5e51a266", "a7467445-8d6a-43cc-b82c-7007664bdf69", "ee7d2bf4-8933-4a3a-8634-3328fe806e08"]
filter_blocked_clusters = false
blocked_clusters = ["bbbbbbbb-0000-0000-0000-000000000000", "bbbbbbbb-1111-1111-1111-111111111111", "bbbbbbbb-2222-2222-2222-222222222222"]
```

- `filter_allowed_clusters` enables or disables cluster filtering according to allow list
- `allowed_clusters` contains list of allowed clusters (depends on previous configuration option)
- `filter_blocked_clusters` enables or disables cluster filtering according to block list
- `blocked_clusters` contains list of disabled clusters (depends on previous configuration option)

Please note that it is possible to use either allow list or block list, or both, if really needed.


## Logging configuration

Logging configuration is specified in section named `[logging]` in configuration file

```
[logging]
debug = true
log_level = "info"
```

- `debug` if set enables debug/developer mode logging which uses colors instead of JSON format
- `log_level` specifies filter for log messages with lower levels

### Log levels

1. `debug` (default one)
1. `info`
1. `warn` or `warning`
1. `error`
1. `fatal`
