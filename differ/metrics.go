/*
Copyright © 2021, 2022, 2023 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package differ

// File metrics contains all metrics that needs to be exposed to Prometheus and
// indirectly to Grafana.

// Generated documentation is available at:
// https://pkg.go.dev/github.com/RedHatInsights/ccx-notification-service/
//
// Documentation in literate-programming-style is available at:
// https://redhatinsights.github.io/ccx-notification-service/packages/differ/metrics.html

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/rs/zerolog/log"

	"github.com/RedHatInsights/ccx-notification-service/conf"
)

// Metrics names
const (
	FetchContentErrorsName            = "fetch_content_errors"
	ReadClusterListErrorsName         = "read_cluster_list_errors"
	ReadReportedErrorsName            = "read_reported_errors"
	ProducerSetupErrorsName           = "producer_setup_errors"
	StorageSetupErrorsName            = "storage_setup_errors"
	ReadReportForClusterErrorsName    = "read_report_for_cluster_errors"
	DeserializeReportErrorsName       = "deserialize_report_errors"
	ReportWithHighImpactName          = "report_with_high_impact"
	NotificationNotSentSameStateName  = "notification_not_sent_same_state"
	NotificationNotSentErrorStateName = "notification_not_sent_error_state"
	NotificationSentName              = "notification_sent"
	NoSeverityTotalRiskName           = "total_risk_no_severity"
)

// Metrics helps
const (
	FetchContentErrorsHelp            = "The total number of errors during fetch from content service"
	ReadClusterListErrorsHelp         = "The total number of errors when reading cluster list from new_reports table"
	ReadReportedErrorsHelp            = "The total number of errors when reading previously reported reports fpr given clusters from reported table"
	ProducerSetupErrorsHelp           = "The total number of errors when setting up Kafka producer"
	StorageSetupErrorsHelp            = "The total number of errors when setting up storage connection"
	ReadReportForClusterErrorsHelp    = "The total number of errors when getting latest report for a given cluster ID"
	DeserializeReportErrorsHelp       = "The total number of errors when deserializing a report retrieved from the new_reports table"
	ReportWithHighImpactHelp          = "The total number of reports with total risk higher than the configured threshold"
	NotificationNotSentSameStateHelp  = "The total number of notifications not sent because we parsed the same report"
	NotificationNotSentErrorStateHelp = "The total number of notifications not sent because of a Kafka producer error"
	NotificationSentHelp              = "The total number of notifications sent"
	NoSeverityTotalRiskHelp           = "The total number of times we handled a total risk that does not have an equivalent service log severity level"
)

// PushGatewayClient is a simple wrapper over http.Client so that prometheus
// can do HTTP requests with the given authentication header
type PushGatewayClient struct {
	AuthToken string

	httpClient http.Client
}

// Do is a simple wrapper over http.Client.Do method that includes
// the authentication header configured in the PushGatewayClient instance
func (pgc *PushGatewayClient) Do(request *http.Request) (*http.Response, error) {
	if pgc.AuthToken != "" {
		log.Debug().Msg("Adding authorization header to HTTP request")
		request.Header.Set("Authorization", "Basic "+pgc.AuthToken)
	} else {
		log.Debug().Msg("No authorization token provided. Making HTTP request without credentials.")
	}
	log.Debug().Str("request", request.URL.String()).Str("method", request.Method).Msg("Pushing metrics to Prometheus push gateway")
	resp, err := pgc.httpClient.Do(request)
	if resp != nil {
		log.Debug().Int("code", resp.StatusCode).Msg("Returned status code")
	}
	return resp, err
}

// FetchContentErrors shows number of errors during fetch from content service
var FetchContentErrors = promauto.NewCounter(prometheus.CounterOpts{
	Name: FetchContentErrorsName,
	Help: FetchContentErrorsHelp,
})

// ReadClusterListErrors shows number of errors when reading cluster list from new_reports table
var ReadClusterListErrors = promauto.NewCounter(prometheus.CounterOpts{
	Name: ReadClusterListErrorsName,
	Help: ReadClusterListErrorsHelp,
})

// ReadReportedErrors shows number of errors when getting previously notified reports from reported table
var ReadReportedErrors = promauto.NewCounter(prometheus.CounterOpts{
	Name: ReadReportedErrorsName,
	Help: ReadReportedErrorsHelp,
})

// ProducerSetupErrors shows number of errors when setting up Kafka producer
var ProducerSetupErrors = promauto.NewCounter(prometheus.CounterOpts{
	Name: ProducerSetupErrorsName,
	Help: ProducerSetupErrorsHelp,
})

// StorageSetupErrors shows number of errors when setting up storage
var StorageSetupErrors = promauto.NewCounter(prometheus.CounterOpts{
	Name: StorageSetupErrorsName,
	Help: StorageSetupErrorsHelp,
})

// ReadReportForClusterErrors shows number of errors when getting latest report for a given cluster
var ReadReportForClusterErrors = promauto.NewCounter(prometheus.CounterOpts{
	Name: ReadReportForClusterErrorsName,
	Help: ReadReportForClusterErrorsHelp,
})

// DeserializeReportErrors shows number of errors when deserializing a report retrieved from the new_reports table
var DeserializeReportErrors = promauto.NewCounter(prometheus.CounterOpts{
	Name: DeserializeReportErrorsName,
	Help: DeserializeReportErrorsHelp,
})

// ReportWithHighImpact shows number of reports with total risk higher than the configured threshold
var ReportWithHighImpact = promauto.NewCounter(prometheus.CounterOpts{
	Name: ReportWithHighImpactName,
	Help: ReportWithHighImpactHelp,
})

// NotificationNotSentSameState shows number of notifications not sent because we parsed the same report
var NotificationNotSentSameState = promauto.NewCounter(prometheus.CounterOpts{
	Name: NotificationNotSentSameStateName,
	Help: NotificationNotSentSameStateHelp,
})

// NotificationNotSentErrorState shows number of notifications not sent because of a Kafka producer error
var NotificationNotSentErrorState = promauto.NewCounter(prometheus.CounterOpts{
	Name: NotificationNotSentErrorStateName,
	Help: NotificationNotSentErrorStateHelp,
})

// NotificationSent shows number notifications sent to the configured Kafka topic
var NotificationSent = promauto.NewCounter(prometheus.CounterOpts{
	Name: NotificationSentName,
	Help: NotificationSentHelp,
})

// NoSeverityTotalRisk shows how many times a total risk not mapped to a service log severity is received
var NoSeverityTotalRisk = promauto.NewCounter(prometheus.CounterOpts{
	Name: NoSeverityTotalRiskName,
	Help: NoSeverityTotalRiskHelp,
})

// AddMetricsWithNamespaceAndSubsystem register the desired metrics using a given namespace
func AddMetricsWithNamespaceAndSubsystem(namespace, subsystem string) {
	// exposed metrics

	// Unregister all metrics and registrer them again
	prometheus.Unregister(FetchContentErrors)
	prometheus.Unregister(ReadClusterListErrors)
	prometheus.Unregister(ProducerSetupErrors)
	prometheus.Unregister(StorageSetupErrors)
	prometheus.Unregister(ReadReportForClusterErrors)
	prometheus.Unregister(DeserializeReportErrors)
	prometheus.Unregister(ReportWithHighImpact)
	prometheus.Unregister(NotificationNotSentSameState)
	prometheus.Unregister(NotificationNotSentErrorState)
	prometheus.Unregister(NotificationSent)
	prometheus.Unregister(NoSeverityTotalRisk)

	// FetchContentErrors shows number of errors during fetch from content service
	FetchContentErrors = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      FetchContentErrorsName,
		Help:      FetchContentErrorsHelp,
	})

	// ReadClusterListErrors shows number of errors when reading cluster list from new_reports table
	ReadClusterListErrors = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      ReadClusterListErrorsName,
		Help:      ReadClusterListErrorsHelp,
	})

	// ProducerSetupErrors shows number of errors when setting up Kafka producer
	ProducerSetupErrors = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      ProducerSetupErrorsName,
		Help:      ProducerSetupErrorsHelp,
	})

	// StorageSetupErrors shows number of errors when setting up storage
	StorageSetupErrors = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      StorageSetupErrorsName,
		Help:      StorageSetupErrorsHelp,
	})

	// ReadReportForClusterErrors shows number of errors when getting latest report for a given cluster
	ReadReportForClusterErrors = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      ReadReportForClusterErrorsName,
		Help:      ReadReportForClusterErrorsHelp,
	})

	// DeserializeReportErrors shows number of errors when deserializing a report retrieved from the new_reports table
	DeserializeReportErrors = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      DeserializeReportErrorsName,
		Help:      DeserializeReportErrorsHelp,
	})

	// ReportWithHighImpact shows number of reports with total risk higher than the configured threshold
	ReportWithHighImpact = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      ReportWithHighImpactName,
		Help:      ReportWithHighImpactHelp,
	})

	// NotificationNotSentSameState shows number of notifications not sent because we parsed the same report
	NotificationNotSentSameState = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      NotificationNotSentSameStateName,
		Help:      NotificationNotSentSameStateHelp,
	})

	// NotificationNotSentErrorState shows number of notifications not sent because of a Kafka producer error
	NotificationNotSentErrorState = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      NotificationNotSentErrorStateName,
		Help:      NotificationNotSentErrorStateHelp,
	})

	// NotificationSent shows number notifications sent to the configured Kafka topic
	NotificationSent = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      NotificationSentName,
		Help:      NotificationSentHelp,
	})

	// NoSeverityTotalRisk shows  how many times a total risk not mapped to a service log severity is received
	NoSeverityTotalRisk = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      NoSeverityTotalRiskName,
		Help:      NoSeverityTotalRiskHelp,
	})
}

// PushCollectedMetrics function pushes the metrics to the configured prometheus push
// gateway
func PushCollectedMetrics(metricsConf *conf.MetricsConfiguration) error {
	client := PushGatewayClient{metricsConf.GatewayAuthToken, http.Client{}}

	// Creates a pusher to the gateway "$PUSHGW_URL/metrics/job/$(job_name)
	return push.New(metricsConf.GatewayURL, metricsConf.Job).
		Collector(FetchContentErrors).
		Collector(ReadClusterListErrors).
		Collector(ReadReportedErrors).
		Collector(ProducerSetupErrors).
		Collector(StorageSetupErrors).
		Collector(ReadReportForClusterErrors).
		Collector(DeserializeReportErrors).
		Collector(ReportWithHighImpact).
		Collector(NotificationNotSentSameState).
		Collector(NotificationNotSentErrorState).
		Collector(NotificationSent).
		Collector(NoSeverityTotalRisk).
		Client(&client).
		Push()
}

// PushMetricsInLoop pushes the metrics in a loop until context is done
func PushMetricsInLoop(ctx context.Context, metricsConf *conf.MetricsConfiguration) {
	if metricsConf.Namespace != "" && metricsConf.GatewayAuthToken != "" {
		log.Info().Msgf("Metrics will be pushed in loop each %f seconds", metricsConf.GatewayTimeBetweenPush.Seconds())

		ticker := time.NewTicker(metricsConf.GatewayTimeBetweenPush)
		for {
			select {
			case <-ticker.C:
				log.Debug().Msg("Pushing metrics")
				err := PushCollectedMetrics(metricsConf)
				if err != nil {
					log.Error().Err(err).Msg("Error pushing the metrics in loop")
				}
				log.Debug().Msg("Metrics pushed")
			case <-ctx.Done():
				return
			}
		}
	}
}
