---
layout: default
---

# Description

CCX Notification Service

## Architecture

![architecture_diagram.png](architecture_diagram.png)

[Architecture diagram, full scale](architecture_diagram.png)

## Class diagram

![class_diagram.png](class_diagram.png)

[Class diagram, full scale](class_diagram.png)

## Sequence diagram

![sequence_diagram.png](sequence_diagram.png)

[Sequence diagram, full scale](sequence_diagram.png)

## Database description

* PostgreSQL database is used as a storage.
* Database description is available [here](./db-description/index.html)

## Documentation for source files from this repository

* [ccx_notification_service.go](./packages/ccx_notification_service.html)
* [conf/config.go](./packages/conf/config.html)
* [differ/comparator.go](./packages/differ/comparator.html)
* [differ/content.go](./packages/differ/content.html)
* [differ/differ.go](./packages/differ/differ.html)
* [differ/storage.go](./packages/differ/storage.html)
* [producer/producer](./packages/producer/producer.html)
* [producer/kafka_producer.go](./packages/producer/kafka_producer.html)
* [types/types.go](./packages/types/types.html)

## Documentation for unit tests files for this repository

* [conf/configuration_test.go](./packages/conf/configuration_test.html)
* [differ/comparator_test.go](./packages/differ/comparator_test.html)
* [differ/differ_test.go](./packages/differ/differ_test.html)
* [producer/producer_test.go](./packages/producer/producer_test.html)
* [tests/mocks/Producer.go](./packages/tests/mocks/Producer.html)
* [tests/mocks/Storage.go](./packages/tests/mocks/Storage.html)
