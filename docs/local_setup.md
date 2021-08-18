---
layout: page
nav_order: 4
---
# Local setup
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

## Prerequisites

This service needs a Kafka broker and PostreSQL database. You can run the required containers using Docker-compose:
```yaml
version: "3"
services:
  db:
    ports:
      - 5432:5432
    image: registry.access.redhat.com/rhscl/postgresql-10-rhel7
    environment:
      - POSTGRESQL_USER=user
      - POSTGRESQL_PASSWORD=password
      - POSTGRESQL_ADMIN_PASSWORD=admin
      - POSTGRESQL_DATABASE=notification
  zookeeper:
    image: confluentinc/cp-zookeeper
    environment:
      - ZOOKEEPER_CLIENT_PORT=32181
      - ZOOKEEPER_SERVER_ID=1
      - ZOOKEEPER_LOG4J_LOGGERS=zookeeper.foo.bar=WARN
      - ZOOKEEPER_LOG4J_ROOT_LOGLEVEL=WARN
      - ZOOKEEPER_TOOLS_LOG4J_LOGLEVEL=ERROR
  kafka:
    image: confluentinc/cp-kafka
    ports:
     - 29092:29092
    depends_on:
     - zookeeper
    environment:
      - KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://kafka:29092
      - KAFKA_BROKER_ID=1
      - KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR=1
      - KAFKA_ZOOKEEPER_CONNECT=zookeeper:32181
      - KAFKA_LOG4J_LOGGERS=kafka.foo.bar=WARN
      - KAFKA_LOG4J_ROOT_LOGLEVEL=WARN
      - KAFKA_TOOLS_LOG4J_LOGLEVEL=ERROR
createtopics:
    image: confluentinc/cp-kafka
    entrypoint:
     - /bin/sh
      - -c
      - "/bin/kafka-topics --bootstrap-server kafka:29092 --create --topic logs --partitions 1; exit 0;"
    depends_on:
     - kafka
```

Apart from this, it is necessary to make some adjustments to `localhost` line at `/etc/hosts`:
```127.0.0.1       localhost kafka```

## Usage

The idea is:
1. Publish messages from `insights-results-aggregator-data` with `insights-results-aggregator-utils` to a Kafka topic.
2. Read those messages with `ccx-notification-writer` and save them to a PostgreSQL database.
3. Consume those messages from `ccx-notification-service` and publish the generated notifications to another topic.

So it is necessary to clone the rest of the repositories before using the `ccx-notification-service`. 

### Insights results aggregator data and utils

It is necessary to clone [insights-results-aggregator-utils](https://github.com/RedHatInsights/insights-results-aggregator-utils) to send the data to Kafka. The example messages are inside [insights-results-aggregator-data](https://github.com/RedHatInsights/insights-results-aggregator-data), so clone it too.

Install [kafkacat](https://rmoff.net/2020/04/20/how-to-install-kafkacat-on-fedora/) and generate some messages with `produce.sh`:

```
cd insights-results-aggregator-data/messages/normal_for_notification_test
/path/to/insights-results-aggregator-utils/input/produce.sh
```

This script will publish all `.json` files in the folder. These messages can be seen inside Kafka using a consumer
```
kafkacat -C -b kafka:29092 -t ccx.ocp.results
```

### Insights content service

Clone [insights-content-service](https://github.com/RedHatInsights/insights-results-aggregator-data) and run `./update_rules_content.sh` in order to generate the folder with the rules. Then run the REST API with:

```
INSIGHTS_CONTENT_SERVICE_CONFIG_FILE=/path/to/insights-content-service/config-devel.toml ./insights-content-service
``` 

This service will be consumed by `ccx-notification-service` just for getting the templates.

### Notification writer

It is necessary to use `ccx-notification-writer` to consume some Kafka messages and write them to the DB:

```
// Initialize the database
CCX_NOTIFICATION_WRITER_CONFIG_FILE=/path/to/ccx-notification-writer/config-devel.toml ./ccx-notification-writer -db-init-migration
CCX_NOTIFICATION_WRITER_CONFIG_FILE=/path/to/ccx-notification-writer/config-devel.toml ./ccx-notification-writer -db-init
// Start the service
CCX_NOTIFICATION_WRITER_CONFIG_FILE=/path/to/ccx-notification-writer/config-devel.toml ./ccx-notification-writer
```

### Notification service

```
NOTIFICATION_SERVICE_CONFIG_FILE=/path/to/ccx-notification-service/config-devel.toml ./ccx-notification-service -instant-reports
```

The `instant-reports` flag doesn't publish anything to the Kafka topic because it doesn't find any difference between the reports in the DB. However, the `weekly-reports` does. If you want to use `instant-reports`, it is necessary to send messages with higher risk.

### Bonus: generating emails

It is possible to reproduce the generation of emails by cloning the [notifications-backend](https://github.com/RedHatInsights/notifications-backend/). You can run it with Quarkus and create a single test like the ones inside [TestOpenshiftAdvisorTemplate](https://github.com/RedHatInsights/notifications-backend/blob/master/backend/src/test/java/com/redhat/cloud/notifications/templates/TestOpenshiftAdvisorTemplate.java#L26). The variable `result` is the generated email. Make sure to fill the [stub action](https://github.com/RedHatInsights/notifications-backend/blob/9ba06e86d69b75a7f3169cf9a950f82b762032ef/backend/src/test/java/com/redhat/cloud/notifications/TestHelpers.java#L226) with the messages produced by the `ccx-notification-service`.

## Troubleshooting

* Make sure to read the contents of `config-devel.toml` at every repositories as it has all the configuration about the connections to the containers (database access, topics, other services endpoints...).

* If you get stuck, you can always drop the tables from the database using 
```
CCX_NOTIFICATION_WRITER_CONFIG_FILE=/path/to/ccx-notification-writer/config-devel.toml ./ccx-notification-writer -db-drop-tables
```
