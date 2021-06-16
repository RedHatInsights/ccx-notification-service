"""Definition of sequence diagram for the CCX Notification Writer service."""

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
def sequence_diagram(c):
    """Definition of sequence diagram."""
    # first, define all nodes that are able to communicate
    cluster = c.object('Cluster')
    ingress = c.object('"Ingress\\nservice"')
    storage_broker = c.object('"Insights-storage-broker\\nservice"')
    s3 = c.object('"S3 bucket"')
    buckit = c.object('"Kafka topic\\nplatform.upload.buckit"')
    ccx_data_pipeline = c.object('"ccx-data-pipeline\\nservice"')
    results = c.object('"Kafka topic\\nccx.ocp.results"')
    writer = c.object('"ccx-notification-writer\\nservice"')
    db = c.object('"Notification\\ndatabase"')
    service = c.object('"ccx-notification-service"')
    notifications = c.object('"Kafka topic\\nplatform.notifications.ingress"')

    # sending data from customer cluster to S3 and
    # Kafka topic platform.upload.buckit
    with cluster:
        with ingress.send("data from IO"):
            with storage_broker.send("data from IO"):
                with s3.store("data from IO"):
                    c.ret("stored, path to object=XYZ")
                c.ret("success, path to object=XYZ")
            with buckit.produce("new data", "cluster=UUID", "path=XYZ"):
                c.ret("stored")
            c.ret("accept")

    # applying OCP results to data, storing them into
    # Kafka topic ccx.ocp.results
    with c.loop():
        with ccx_data_pipeline:
            with buckit.request("get message"):
                c.ret("cluster=UUID", "path=XYZ")
            with s3.read_object("path=XYZ"):
                c.ret("data from IO")
            with ccx_data_pipeline.process("data from IO"):
                c.ret("rule results")
            with results.store("rule results"):
                c.ret("stored")

    # consuming messages, storing them in Notification DB
    with c.loop():
        with writer:
            with results.request("get results"):
                c.ret("here are rule results")
            with writer.validate("rule resuls"):
                c.ret("validated")
            with db.store("rule results"):
                c.ret("stored")

    # processing new results, sending notifications
    with c.loop():
        with service:
            with db.request("read new results"):
                c.ret("here are new results")
            with db.request("read reported results"):
                c.ret("here are already reported results")
            with service.find_diff("reported results", "new results"):
                c.ret("diff")
            with notifications.notify("new reports"):
                c.ret("accepted")
