---
layout: page
nav_order: 2
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
