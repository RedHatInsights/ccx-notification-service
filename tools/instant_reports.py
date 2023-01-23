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
def instant_reports(c):
    """Definition of sequence diagram for instant reports generation."""
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

    # processing new results, sending instant notifications
    with service:
        with db.request("read list of new results"):
            c.ret("list of new results")
        with c.loop("iterate over all new results"):
            with db.request("read new result", "cluster UUID"):
                c.ret("here are new result for cluster UUID")
            with db.request("read reported result", "cluster UUID"):
                c.ret("here are already reported result for cluster UUID")
            with service.find_differences("reported result", "new result"):
                c.ret("new rules")
            with service.filter("new rules", "total_risk_threshold"):
                c.note("currently any new rule\\nwith total_risk>3 is reported")
                c.ret("filtered\\nnew rules")
            with service.prepare_notification_message("filtered new rules"):
                c.ret("notification\\nmessage")
            with c.group("New rules were found"):
                with notifications.notify("notification message"):
                    c.ret("accepted")
