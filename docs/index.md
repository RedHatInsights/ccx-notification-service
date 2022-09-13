---
layout: default
title: Home
nav_order: 0
---

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
