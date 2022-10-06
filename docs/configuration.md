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

Configuration is done by toml config, default one is `config.toml` in working directory,
but it can be overwritten by `NOTIFICATION_SERVICE_CONFIG_FILE` env var.

Also each key in config can be overwritten by corresponding env var. For example if you have config

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
outside of main config file(like passwords).

### Clowder configuration

In Clowder environment, some configuration options are injected automatically.
Currently Kafka broker configuration is injected this side. To test this
behavior, it is possible to specify path to Clowder-related configuration file
via `AGG_CONFIG` environment variable:

```
export ACG_CONFIG="clowder_config.json"
```

## Service Log configuration

Service Log configuration is in section `[service-log]` in config file

```
[service_log]
enabled = false
offline_token = ""
token_refreshment_url = "https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token"
url = "https://api.openshift.com/api/service_logs/v1/cluster_logs/"
timeout = "15s"
likelihood_threshold = 0
impact_threshold = 0
severity_threshold = 0
total_risk_threshold = 3
event_filter = "totalRisk > totalRiskThreshold"
```

- `enabled` determines whether the notifications service sends messages to Service Log
- `offline_token` is an offline token used to retrieve online token via OpenID Connect
- `token_refreshment_url` is a URL of OpenID Connect API for retrieval of new online token
- `timeout` is a time used as a timeout when sending requests to Service Log API
- `likelihood_threshold`,`impact_threshold`, `severity_threshold` and `total_risk_threshold` are values which can be used in `event_filter` for filtering messages sent to Service Log
- `event_filter` is a condition string used to determine which messages will be sent to Service Log

Please note that for correct functionality of Service Log integration, `dependencies` configuration should be also present.

## Dependencies configuration

Dependencies configuration is in section `[dependencies]` in config file

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

Processing configuration is in section `[processing]` in config file

```
[processing]
filter_allowed_clusters = true
allowed_clusters = ["34c3ecc5-624a-49a5-bab8-4fdc5e51a266", "a7467445-8d6a-43cc-b82c-7007664bdf69", "ee7d2bf4-8933-4a3a-8634-3328fe806e08"]
```

- `filter_allowed_clusters` enables or disabled cluster filtering according to allow list
- `allowed_clusters` contains list of allowed clusters (depends on previous configuration option)
