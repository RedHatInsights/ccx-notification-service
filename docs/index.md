---
layout: default
title: Home
nav_order: 0
---


## Notification templates

Notification templates used to send e-mails etc. to customers are stored in different repository:
[https://github.com/RedHatInsights/notifications-backend/](https://github.com/RedHatInsights/notifications-backend/)

Templates used by this notification service are available at:
[https://github.com/RedHatInsights/notifications-backend/tree/master/backend/src/main/resources/templates/AdvisorOpenshift](https://github.com/RedHatInsights/notifications-backend/tree/master/backend/src/main/resources/templates/AdvisorOpenshift)

## Sequence diagram for the whole pipeline - notification service integration

![sequence_diagram.png](images/sequence_diagram.png)

[Sequence diagram, full scale](images/sequence_diagram.png)

## Sequence diagram for the whole pipeline - Service Log integration

![sequence_diagram_service_log.png](images/sequence_diagram_service_log.png)

[Sequence diagram, full scale](images/sequence_diagram._service_logpng)

## Sequence diagram for instant reports

![instant_reports.png](images/instant_reports.png)

[Full scale](images/instant_reports.png)

## Sequence diagram for weekly reports

![weekly_reports.png](images/weekly_reports.png)

[Full scale](images/weekly_reports.png)

## Sequence diagram for CCX Notification Writer service

[Sequence diagram for CCX Notification Writer service](sequence_diagram_notification_writer.png)

## Cooldown mechanism

The cooldown mechanism is used to filter the previously reported issues so that they are not continuously sent to the customers. It works by defining a miminum amount of time that must elapse between two notifications. That cooldown time is applied to all the issues processed during an iteration.

### Data flow of the notification service without cooldown

See steps 9 to 12 of the [data flow section](#data-flow)

### Data flow of the notification service with cooldown

1. The latest entry for each distinct cluster in the `new_reports` table is consumed by the `ccx-notification-service`.
1. Results stored in `reported` table within the cooldown time are retrieved. Therefore all the reported issues that are not older than the configured cooldown are cached in a `previouslyReported` map by the service in each iteration.
1. When checking for new issues in the report, the `ccx-notification-service` looks up each issue in the `previouslyReported` map, and if found, that issue is considered to still be in cooldown and is not processed further. If not found, the processing of the issue continues.
1. If changes (new issues) has been found between the previous report and the new one, a notification message is sent into Kafka topic named `platform.notifications.ingress`. The expected format of the message can be found [here](https://core-platform-apps.pages.redhat.com/notifications-docs/dev/user-guide/send-notification.html#_kafka).
1. New issues is also sent to Service Log via REST API. To use the Service Log API, the `ccx-notification-service` uses the credentials stored in [vault](https://vault.devshift.net/ui/vault/secrets/insights/show/secrets/insights-prod/ccx-data-pipeline-prod/ccx-notification-service-auth).
1. The newest result is stored into `reported` table to be used in the next `ccx-notification-service` iteration.

### Configuring the cooldown mechanism

The cooldown mechanism can be configured by specifying the `cooldown` field under the `notifications` configuration in the [config.toml](../config.toml) file or by setting the `CCX_NOTIFICATION_SERVICE__NOTIFICATIONS__COOLDOWN` environment variable.

The value set is used directly within an SQL query, so the expected format is an integer followed by a valid SQL epoch time units (year[s] month[s] day[s] hour[s] minute[s] second[s])
