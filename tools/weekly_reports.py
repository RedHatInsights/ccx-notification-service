"""Definition of sequence diagram for the instant reports generation."""

#
#  (C) Copyright 2021  Pavel Tisnovsky
#
#  All rights reserved. This program and the accompanying materials
#  are made available under the terms of the Eclipse Public License v1.0
#  which accompanies this distribution, and is available at
#  http://www.eclipse.org/legal/epl-v10.html
#
#  Contributors:
#      Pavel Tisnovsky
#

import napkin


@napkin.seq_diagram()
def weekly_reports(c):
    """Definition of sequence diagram for weekly reports generation."""
    # first, define all nodes that are able to communicate
    timer = c.object("Timer")
    service = c.object('"ccx-notification-service"')
    db = c.object('"Notification\\ndatabase"')
    notifications = c.object('"Kafka topic\\nplatform.notifications.ingress"')

    with timer:
        c.note("CRON rule")

    with service:
        c.note("CRON job configured and\\ndeployed to OpenShift")

    with db:
        c.note("AWS RDS\\nor regular PostgreSQL")

    with notifications:
        c.note("Notification\\nservice gateway")

    with timer:
        service.notify("tick")

    # processing new results, sending weekly notifications
    with service:
        with db.request("read results", "cluster UUID", "-7 days to today"):
            c.ret("list of results for cluster UUID")
        with c.loop("iterate over all results"):
            with service.filter("new rules", "total_risk_threshold"):
                c.note(
                    "currently any new rule will be reported,\\nbut filtering can be changed in future"
                )
                c.ret("filtered\\nrules")
            with c.group("At least one rule was found"):
                with service.prepare_notification_message(
                    "filtered rules", "weekly report template"
                ):
                    c.ret("notification\\nmessage")
                with notifications.notify("notification message"):
                    c.ret("accepted")
