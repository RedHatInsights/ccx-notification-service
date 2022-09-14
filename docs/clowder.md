---
layout: page
nav_order: 4
---

# Clowder configuration

As the rest of the services deployed in the Console RedHat platform, the
CCX Notification Writer should update its configuration using the relevant
values extracted from the Clowder configuration file.

For a general overview about Clowder configuration file and how it is used
in the external data pipeline, please refer to
[Clowder configuration](https://ccx.pages.redhat.com/ccx-docs/customer/clowder.html)
in CCX Docs.

## How can we deal with the Clowder configuration file?

As the service is implemented in Golang, we take advantage of using
[app-common-go](https://github.com/RedHatInsights/app-common-go/).
It provides to the service all the needed values for configuring Kafka
access and the topics name mapping.

The `conf` package is taking care of reading the data structures provided
by `app-common-go` library and merge Clowder configuration with the service
configuration.

# CCX Notification Service specific relevant values

As the CCX Notification Service reads data from a database and, depending
on the configuration, is able to send messages to a Kafka topic. It will
need to take care of the Kafka access related values, topic names mapping
and the database access values.
