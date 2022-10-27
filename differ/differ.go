/*
Copyright © 2021, 2022 Red Hat, Inc.

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

// Generated documentation is available at:
// https://pkg.go.dev/github.com/RedHatInsights/ccx-notification-service/differ
//
// Documentation in literate-programming-style is available at:
// https://redhatinsights.github.io/ccx-notification-service/packages/differ/differ.html

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/RedHatInsights/ccx-notification-service/ocmclient"
	"github.com/RedHatInsights/ccx-notification-service/producer/disabled"
	"github.com/RedHatInsights/ccx-notification-service/producer/servicelog"

	"github.com/RedHatInsights/ccx-notification-service/producer/kafka"

	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/producer"
	"github.com/RedHatInsights/ccx-notification-service/types"
	"github.com/RedHatInsights/insights-operator-utils/evaluator"
)

// Configuration-related constants
const (
	configFileEnvVariableName = "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"
	defaultConfigFileName     = "config"
)

// Exit codes
const (
	// ExitStatusOK means that the tool finished with success
	ExitStatusOK = iota
	// ExitStatusConfiguration is an error code related to program configuration
	ExitStatusConfiguration
	// ExitStatusError is a general error code
	ExitStatusError
	// ExitStatusStorageError is returned in case of any consumer-related error
	ExitStatusStorageError
	// ExitStatusFetchContentError is returned in case content cannot be fetch correctly
	ExitStatusFetchContentError
	// ExitStatusKafkaBrokerError is for kafka broker connection establishment errors
	ExitStatusKafkaBrokerError
	// ExitStatusKafkaProducerError is for kafka event production failures
	ExitStatusKafkaProducerError
	// ExitStatusKafkaConnectionNotClosedError is raised when connection cannot be closed
	ExitStatusKafkaConnectionNotClosedError
	// ExitStatusCleanerError is raised when clean operation is not successful
	ExitStatusCleanerError
	// ExitStatusMetricsError is raised when prometheus metrics cannot be pushed
	ExitStatusMetricsError
	// ExitStatusEventFilterError is raised when event filter is not set correctly
	ExitStatusEventFilterError
	// ExitStatusServiceLogError is raised when Service Log notifier cannot be initialized
	ExitStatusServiceLogError
)

// Messages
const (
	serviceName                 = "CCX Notification Service"
	versionMessage              = "Notification service version 1.0"
	authorsMessage              = "Pavel Tisnovsky, Papa Bakary Camara, Red Hat Inc."
	separator                   = "------------------------------------------------------------"
	operationFailedMessage      = "Operation failed"
	clusterEntryMessage         = "cluster entry"
	organizationIDAttribute     = "org id"
	AccountNumberAttribute      = "account number"
	typeAttribute               = "type"
	clusterAttribute            = "cluster"
	ruleAttribute               = "rule"
	likelihoodAttribute         = "likelihood"
	impactAttribute             = "impact"
	errorKeyAttribute           = "error key"
	numberOfEventsAttribute     = "number of events"
	clustersAttribute           = "clusters"
	totalRiskAttribute          = "totalRisk"
	errorStr                    = "Error:"
	reportWithHighImpactMessage = "Report with high impact detected"
	differentReportMessage      = "Different report from the last one"
	invalidJSONContent          = "The provided content cannot be encoded as JSON."
	contextToEscapedStringError = "Notification message will not be generated as context couldn't be converted to escaped string."
	metricsPushFailedMessage    = "Couldn't push prometheus metrics"
	eventFilterNotSetMessage    = "Event filter not set"
	evaluationErrorMessage      = "Evaluation error"
	serviceLogSendErrorMessage  = "Sending entry to service log failed for this report"
	renderReportsFailedMessage  = "Rendering reports failed for this cluster"
	ReportNotFoundError         = "report for rule ID %v and error key %v has not been found"
	destinationNotSet           = "No known event destination configured. Aborting."
)

// Constants for notification message top level fields
const (
	notificationBundleName      = "openshift"
	notificationApplicationName = "advisor"
)

// Constants for notification event expected fields
const (
	// INSTANT NOTIFICATION PAYLOAD FIELDS
	notificationPayloadRuleDescription = "rule_description"
	notificationPayloadRuleURL         = "rule_url"
	notificationPayloadTotalRisk       = "total_risk"
	notificationPayloadPublishDate     = "publish_date"
	// WEEKLY NOTIFICATION PAYLOAD FIELDS
	notificationPayloadTotalClusters        = "total_clusters"
	notificationPayloadTotalRecommendations = "total_recommendations"
	notificationPayloadTotalCritical        = "total_critical"
	notificationPayloadTotalImportant       = "total_important"
	notificationPayloadTotalIncidents       = "total_incidents"
)

// Constants for notification context expected fields
const (
	notificationContextDisplayName = "display_name"
	notificationContextHostURL     = "host_url"
	notificationContextAdvisorURL  = "advisor_url"
)

// Constants used to filter events
const (
	DefaultTotalRiskThreshold  = 3
	DefaultLikelihoodThreshold = 0
	DefaultImpactThreshold     = 0
	DefaultSeverityThreshold   = 0
	DefaultEventFilter         = "totalRisk >= totalRiskThreshold"
)

// Constants used for creating Service Log entries - there is a length limit on text fields in Service Log,
// which will return an error status code in case this limit is exceeded
const (
	serviceLogSummaryMaxLength     = 255
	serviceLogDescriptionMaxLength = 4000
)

// EventThresholds structure contains all threshold values for event filter
// evaluator
type EventThresholds struct {
	TotalRisk  int
	Likelihood int
	Impact     int
	Severity   int
}

// EventValue structure contains all event values for event filter evaluator
type EventValue struct {
	TotalRisk  int
	Likelihood int
	Impact     int
	Severity   int
}

// NotificationURLs structure contains all the URLs that are inserted in the notifications
type NotificationURLs struct {
	ClusterDetails  string
	RuleDetails     string
	InsightsAdvisor string
}

var (
	notificationType      types.EventType
	kafkaNotifier         producer.Producer
	serviceLogNotifier    producer.Producer
	notificationEventURLs NotificationURLs
	kafkaEventThresholds  EventThresholds = EventThresholds{
		TotalRisk:  DefaultTotalRiskThreshold,
		Likelihood: DefaultLikelihoodThreshold,
		Impact:     DefaultImpactThreshold,
		Severity:   DefaultSeverityThreshold,
	}
	serviceLogEventThresholds EventThresholds = EventThresholds{
		TotalRisk:  DefaultTotalRiskThreshold,
		Likelihood: DefaultLikelihoodThreshold,
		Impact:     DefaultImpactThreshold,
		Severity:   DefaultSeverityThreshold,
	}
	previouslyReported = types.NotifiedRecordsPerClusterByTarget{
		types.NotificationBackendTarget: types.NotifiedRecordsPerCluster{},
		types.ServiceLogTarget:          types.NotifiedRecordsPerCluster{},
	}
	kafkaEventFilter      string = DefaultEventFilter
	serviceLogEventFilter string = DefaultEventFilter
)

// showVersion function displays version information.
func showVersion() {
	fmt.Println(versionMessage)
}

// showAuthors function displays information about authors.
func showAuthors() {
	fmt.Println(authorsMessage)
}

// showConfiguration function displays actual configuration.
func showConfiguration(config conf.ConfigStruct) {
	brokerConfig := conf.GetKafkaBrokerConfiguration(config)
	log.Info().
		Bool("Enabled", brokerConfig.Enabled).
		Str("Address", brokerConfig.Address).
		Str("SecurityProtocol", brokerConfig.SecurityProtocol).
		Str("SaslMechanism", brokerConfig.SaslMechanism).
		Str("Topic", brokerConfig.Topic).
		Str("Timeout", brokerConfig.Timeout.String()).
		Int("Likelihood threshold", brokerConfig.LikelihoodThreshold).
		Int("Impact threshold", brokerConfig.ImpactThreshold).
		Int("Severity threshold", brokerConfig.SeverityThreshold).
		Int("Total risk threshold", brokerConfig.TotalRiskThreshold).
		Str("Event filter", brokerConfig.EventFilter).
		Msg("Broker configuration")

	serviceLogConfig := conf.GetServiceLogConfiguration(config)
	log.Info().
		Bool("Enabled", serviceLogConfig.Enabled).
		Str("ClientID", serviceLogConfig.ClientID).
		Int("Likelihood threshold", brokerConfig.LikelihoodThreshold).
		Int("Impact threshold", brokerConfig.ImpactThreshold).
		Int("Severity threshold", brokerConfig.SeverityThreshold).
		Int("Total risk threshold", serviceLogConfig.TotalRiskThreshold).
		Str("Event filter", serviceLogConfig.EventFilter).
		Str("OCM URL", serviceLogConfig.URL).
		Msg("ServiceLog configuration")

	storageConfig := conf.GetStorageConfiguration(config)
	log.Info().
		Str("Driver", storageConfig.Driver).
		Str("DB Name", storageConfig.PGDBName).
		Str("Username", storageConfig.PGUsername). // password is omitted on purpose
		Str("Host", storageConfig.PGHost).
		Int("Port", storageConfig.PGPort).
		Bool("LogSQLQueries", storageConfig.LogSQLQueries).
		Str("Parameters", storageConfig.PGParams).
		Msg("Storage configuration")

	loggingConfig := conf.GetLoggingConfiguration(config)
	log.Info().
		Str("Level", loggingConfig.LogLevel).
		Bool("Pretty colored debug logging", loggingConfig.Debug).
		Msg("Logging configuration")

	notificationConfig := conf.GetNotificationsConfiguration(config)
	log.Info().
		Str("Insights Advisor URL", notificationConfig.InsightsAdvisorURL).
		Str("Cluster details URI", notificationConfig.ClusterDetailsURI).
		Str("Rule details URI", notificationConfig.RuleDetailsURI).
		Str("Cooldown", notificationConfig.Cooldown).
		Msg("Notifications configuration")

	metricsConfig := conf.GetMetricsConfiguration(config)

	// Authentication token and metrics groups values are omitted on
	// purpose
	log.Info().
		Str("Namespace", metricsConfig.Namespace).
		Str("Subsystem", metricsConfig.Subsystem).
		Str("Push Gateway", metricsConfig.GatewayURL).
		Int("Retries", metricsConfig.Retries).
		Str("Retry after", metricsConfig.RetryAfter.String()).
		Msg("Metrics configuration")

	processingConfig := conf.GetProcessingConfiguration(config)
	log.Info().
		Bool("Filter allowed clusters", processingConfig.FilterAllowedClusters).
		Strs("List of allowed clusters", processingConfig.AllowedClusters).
		Bool("Filter blocked clusters", processingConfig.FilterBlockedClusters).
		Strs("List of blocked clusters", processingConfig.BlockedClusters).
		Msg("Processing configuration")
}

func calculateTotalRisk(impact, likelihood int) int {
	return (impact + likelihood) / 2
}

// ccx_rules_ocp.external.rules.cluster_wide_proxy_auth_check.report
// ->
// cluster_wide_proxy_auth_check
func moduleToRuleName(module types.ModuleName) types.RuleName {
	result := strings.TrimSuffix(string(module), ".report")
	return ruleIDToRuleName(types.RuleID(result))
}

// ccx_rules_ocp.external.rules.cluster_wide_proxy_auth_check
// ->
// cluster_wide_proxy_auth_check
func ruleIDToRuleName(ruleID types.RuleID) types.RuleName {
	return types.RuleName(string(ruleID)[strings.LastIndex(string(ruleID), ".")+1:])
}

func findRuleByNameAndErrorKey(
	ruleContent types.RulesMap, ruleName types.RuleName, errorKey types.ErrorKey) (
	likelihood int, impact int, totalRisk int, description string) {
	rc := ruleContent[string(ruleName)]
	ek := rc.ErrorKeys
	val := ek[string(errorKey)]
	likelihood = val.Metadata.Likelihood
	description = val.Metadata.Description
	impact = val.Metadata.Impact.Impact
	totalRisk = calculateTotalRisk(likelihood, impact)
	return
}

// evaluateFilterExpression function tries to evaluate event filter expression
// based on provided threshold values and actual reccomendation values
func evaluateFilterExpression(eventFilter string, thresholds EventThresholds, eventValue EventValue) (int, error) {

	// values to be passed into expression evaluator
	values := make(map[string]int)
	values["likelihoodThreshold"] = thresholds.Likelihood
	values["impactThreshold"] = thresholds.Impact
	values["severityThreshold"] = thresholds.Severity
	values["totalRiskThreshold"] = thresholds.TotalRisk
	values["likelihood"] = eventValue.Likelihood
	values["impact"] = eventValue.Impact
	values["severity"] = eventValue.Severity
	values["totalRisk"] = eventValue.TotalRisk

	// try to evaluate event filter expression
	return evaluator.Evaluate(eventFilter, values)
}

func findRenderedReport(reports []types.RenderedReport, ruleName types.RuleName, errorKey types.ErrorKey) (types.RenderedReport, error) {
	for _, report := range reports {
		reportRuleName := ruleIDToRuleName(report.RuleID)
		if reportRuleName == ruleName && report.ErrorKey == errorKey {
			return report, nil
		}
	}
	return types.RenderedReport{}, fmt.Errorf(ReportNotFoundError, ruleName, errorKey)
}

func createServiceLogEntry(report types.RenderedReport, cluster types.ClusterEntry) types.ServiceLogEntry {
	logEntry := types.ServiceLogEntry{
		ClusterUUID: cluster.ClusterName,
		Description: report.Reason,
		ServiceName: serviceName,
		Summary:     report.Description,
	}

	// It is necessary to truncate the fields because of Service Log limitations
	if len(logEntry.Summary) > serviceLogSummaryMaxLength {
		logEntry.Summary = logEntry.Summary[:serviceLogSummaryMaxLength]
	}

	if len(logEntry.Description) > serviceLogDescriptionMaxLength {
		logEntry.Description = logEntry.Description[:serviceLogDescriptionMaxLength]
	}

	return logEntry
}

func produceEntriesToServiceLog(configuration *conf.ConfigStruct, cluster types.ClusterEntry,
	ruleContent types.RulesMap, reports []types.ReportItem) (totalMessages int, err error) {
	renderedReports, err := renderReportsForCluster(
		conf.GetDependenciesConfiguration(*configuration), cluster.ClusterName,
		reports, ruleContent)
	if err != nil {
		log.Err(err).
			Str("cluster name", string(cluster.ClusterName)).
			Msg(renderReportsFailedMessage)
		return
	}
	for _, r := range reports {
		module := r.Module
		ruleName := moduleToRuleName(module)
		errorKey := r.ErrorKey

		likelihood, impact, totalRisk, _ := findRuleByNameAndErrorKey(ruleContent, ruleName, errorKey)
		eventValue := EventValue{
			Likelihood: likelihood,
			Impact:     impact,
			TotalRisk:  totalRisk,
		}

		// try to evaluate event filter expression
		result, err := evaluateFilterExpression(serviceLogEventFilter,
			serviceLogEventThresholds, eventValue)

		if err != nil {
			log.Err(err).Msg(evaluationErrorMessage)
			continue
		}

		if result > 0 {
			log.Warn().
				Str(typeAttribute, r.Type).
				Str(ruleAttribute, string(ruleName)).
				Str(errorKeyAttribute, string(errorKey)).
				Int(likelihoodAttribute, likelihood).
				Int(impactAttribute, impact).
				Int(totalRiskAttribute, totalRisk).
				Msg(reportWithHighImpactMessage)
			if !shouldNotify(cluster, r, types.ServiceLogTarget) {
				continue
			}
			// if new report differs from the older one -> send notification
			log.Info().
				Str(clusterName, string(cluster.ClusterName)).
				Msg(differentReportMessage)
			ReportWithHighImpact.Inc()

			renderedReport, err := findRenderedReport(renderedReports, ruleName, errorKey)
			if err != nil {
				log.Err(err).Msgf("Output from content template renderer does not contain "+
					"result for cluster %s, rule %s and error key %s", cluster.ClusterName, ruleName, errorKey)
				continue
			}

			logEntry := createServiceLogEntry(renderedReport, cluster)

			msgBytes, err := json.Marshal(logEntry)
			if err != nil {
				log.Error().Err(err).Msg(invalidJSONContent)
				continue
			}

			log.Info().
				Str(clusterAttribute, string(cluster.ClusterName)).
				Msg("Producing service log message")
			_, _, err = serviceLogNotifier.ProduceMessage(msgBytes)
			if err != nil {
				log.Err(err).
					Str(clusterAttribute, string(cluster.ClusterName)).
					Str(ruleAttribute, string(ruleName)).
					Str(errorKeyAttribute, string(errorKey)).
					Msg(serviceLogSendErrorMessage)
				return totalMessages, err
			}
			totalMessages++
		}
	}

	return totalMessages, nil
}

func produceEntriesToKafka(cluster types.ClusterEntry, ruleContent types.RulesMap,
	reportItems []types.ReportItem, storage Storage, report types.ClusterReport) (int, error) {

	notificationMsg := generateInstantNotificationMessage(
		&notificationEventURLs.ClusterDetails,
		fmt.Sprint(cluster.AccountNumber),
		fmt.Sprint(cluster.OrgID),
		string(cluster.ClusterName))
	notifiedAt := types.Timestamp(time.Now())

	for _, r := range reportItems {
		module := r.Module
		ruleName := moduleToRuleName(module)
		errorKey := r.ErrorKey
		likelihood, impact, totalRisk, description := findRuleByNameAndErrorKey(ruleContent, ruleName, errorKey)
		eventValue := EventValue{
			Likelihood: likelihood,
			Impact:     impact,
			TotalRisk:  totalRisk,
		}

		// try to evaluate event filter expression
		result, err := evaluateFilterExpression(kafkaEventFilter,
			kafkaEventThresholds, eventValue)

		if err != nil {
			log.Err(err).Msg(evaluationErrorMessage)
			continue
		}

		if result > 0 {
			log.Warn().
				Str(typeAttribute, r.Type).
				Str(ruleAttribute, string(ruleName)).
				Str(errorKeyAttribute, string(errorKey)).
				Int(likelihoodAttribute, likelihood).
				Int(impactAttribute, impact).
				Int(totalRiskAttribute, totalRisk).
				Msg(reportWithHighImpactMessage)
			if !shouldNotify(cluster, r, types.NotificationBackendTarget) {
				continue
			}
			// if new report differs from the older one -> send notification
			log.Info().
				Str(clusterName, string(cluster.ClusterName)).
				Msg(differentReportMessage)
			ReportWithHighImpact.Inc()
			notificationPayloadURL := generateNotificationPayloadURL(&notificationEventURLs.RuleDetails, string(cluster.ClusterName), module, errorKey)
			appendEventToNotificationMessage(notificationPayloadURL, &notificationMsg, description, totalRisk, time.Time(cluster.UpdatedAt).UTC().Format(time.RFC3339Nano))
		}
	}

	if len(notificationMsg.Events) == 0 {
		updateNotificationRecordSameState(storage, cluster, report, notifiedAt, types.NotificationBackendTarget)
		return 0, nil
	}

	log.Info().
		Str(clusterAttribute, string(cluster.ClusterName)).
		Int(numberOfEventsAttribute, len(notificationMsg.Events)).
		Msg("Producing instant notification")

	msgBytes, err := json.Marshal(notificationMsg)
	if err != nil {
		log.Error().Err(err).Msg(invalidJSONContent)
		return -1, err
	}
	_, offset, err := kafkaNotifier.ProduceMessage(msgBytes)
	if err != nil {
		log.Error().
			Str(errorStr, err.Error()).
			Msg("Couldn't send notification message to kafka topic.")
		updateNotificationRecordErrorState(storage, err, cluster, report, notifiedAt, types.NotificationBackendTarget)
		return -1, err
	}

	if offset != -1 {
		// update the database if any message is sent (not a DisabledProducer)
		log.Debug().Msg("notifier is not disabled so DB is updated")
		updateNotificationRecordSentState(storage, cluster, report, notifiedAt, types.NotificationBackendTarget)
		return len(notificationMsg.Events), nil
	}
	return 0, nil
}

func processReportsByCluster(config conf.ConfigStruct, ruleContent types.RulesMap, storage Storage, clusters []types.ClusterEntry) {
	notifiedIssues := 0
	clustersCount := len(clusters)
	skippedEntries := 0
	emptyEntries := 0

	for i, cluster := range clusters {
		log.Info().
			Int("#", i).
			Int("of", clustersCount).
			Int(organizationIDAttribute, int(cluster.OrgID)).
			Int(AccountNumberAttribute, int(cluster.AccountNumber)).
			Str(clusterAttribute, string(cluster.ClusterName)).
			Msg(clusterEntryMessage)

		report, err := storage.ReadReportForClusterAtTime(cluster.OrgID, cluster.ClusterName, cluster.UpdatedAt)
		if err != nil {
			ReadReportForClusterErrors.Inc()
			skippedEntries++
			log.Err(err).Msg(operationFailedMessage)
			continue
		}

		var deserialized types.Report
		err = json.Unmarshal([]byte(report), &deserialized)
		if err != nil {
			DeserializeReportErrors.Inc()
			skippedEntries++
			log.Err(err).Msg("Deserialization error - Couldn't create report object")
			continue
		}

		if len(deserialized.Reports) == 0 {
			log.Info().Msgf("No reports in notification database for cluster %s", cluster.ClusterName)
			emptyEntries++
			continue
		}

		if conf.GetServiceLogConfiguration(config).Enabled {
			notifiedAt := types.Timestamp(time.Now())
			newNotifiedIssues, err := produceEntriesToServiceLog(&config, cluster, ruleContent, deserialized.Reports)
			updateNotificationRecordState(storage, cluster, report, newNotifiedIssues, notifiedAt, types.ServiceLogTarget, err)
			notifiedIssues += newNotifiedIssues
		}

		if !conf.GetKafkaBrokerConfiguration(config).Enabled {
			continue
		}
		newNotifiedIssues, err := produceEntriesToKafka(cluster, ruleContent, deserialized.Reports, storage, report)
		if err != nil {
			log.Err(err).
				Str(clusterAttribute, string(cluster.ClusterName)).
				Msg("Unable to send the notification message to Kafka")
			continue
		}
		notifiedIssues += newNotifiedIssues
	}
	log.Info().Msgf("Number of reports not retrieved/deserialized: %d", skippedEntries)
	log.Info().Msgf("Number of empty reports skipped: %d", emptyEntries)
	log.Info().Msgf("Number of high impact issues notified: %d", notifiedIssues)
}

// getNotificationDigestForCurrentAccount function returns new digest object if none has been previously created for given account
func getNotificationDigestForCurrentAccount(notificationsByAccount map[types.AccountNumber]types.Digest, accountNumber types.AccountNumber) (digest types.Digest) {
	if _, ok := notificationsByAccount[accountNumber]; !ok {
		log.Info().Msgf("Creating notification digest for account %d", accountNumber)
		digest = types.Digest{}
	} else {
		log.Info().Msgf("Modifying notification digest for account %d", accountNumber)
		digest = notificationsByAccount[accountNumber]
	}
	return
}

// updateDigestNotificationCounters function increments number of important or critical notification detected in a given weekly digest
func updateDigestNotificationCounters(digest *types.Digest, totalRisk int) {
	if totalRisk == 3 {
		log.Warn().Int(totalRiskAttribute, totalRisk).Msg("Important report detected. Adding to weekly digest")
		digest.ImportantNotifications++
	} else if totalRisk == 4 {
		log.Warn().Int(totalRiskAttribute, totalRisk).Msg("Critical report detected. Adding to weekly digest")
		digest.CriticalNotifications++
	}
}

// processAllReportsFromCurrentWeek function creates weekly digest with for all the clusters corresponding to each user account
func processAllReportsFromCurrentWeek(ruleContent types.RulesMap, storage Storage, clusters []types.ClusterEntry) {
	digestByAccount := map[types.AccountNumber]types.Digest{}
	digest := types.Digest{}

	for i, cluster := range clusters {
		log.Info().
			Int("#", i).
			Int(organizationIDAttribute, int(cluster.OrgID)).
			Int(AccountNumberAttribute, int(cluster.AccountNumber)).
			Str(clusterAttribute, string(cluster.ClusterName)).
			Msg(clusterEntryMessage)

		digest = getNotificationDigestForCurrentAccount(digestByAccount, cluster.AccountNumber)
		digest.ClustersAffected++

		report, _, err := storage.ReadReportForCluster(cluster.OrgID, cluster.ClusterName)
		if err != nil {
			log.Err(err).Msg(operationFailedMessage)
			os.Exit(ExitStatusStorageError)
		}

		var deserialized types.Report
		err = json.Unmarshal([]byte(report), &deserialized)
		if err != nil {
			log.Err(err).Msg("Deserialization error - Couldn't create report object")
			os.Exit(ExitStatusStorageError)
		}

		numReports := len(deserialized.Reports)
		if numReports == 0 {
			log.Info().Msgf("No reports in notification database for cluster %s", cluster.ClusterName)
			continue
		}
		digest.Recommendations += numReports

		for i, r := range deserialized.Reports {
			moduleName := r.Module
			ruleName := moduleToRuleName(moduleName)
			errorKey := r.ErrorKey
			likelihood, impact, totalRisk, _ := findRuleByNameAndErrorKey(ruleContent, ruleName, errorKey)

			log.Info().
				Int("#", i).
				Str("type", r.Type).
				Str(ruleAttribute, string(ruleName)).
				Str(errorKeyAttribute, string(errorKey)).
				Int("likelihood", likelihood).
				Int("impact", impact).
				Int(totalRiskAttribute, totalRisk).
				Msg("Report")
			updateDigestNotificationCounters(&digest, totalRisk)
		}
		digestByAccount[cluster.AccountNumber] = digest
	}

	for account, digest := range digestByAccount {
		if digest.Recommendations == 0 {
			log.Info().Msgf("No issues to notify to account %d", account)
			continue
		}

		log.Info().
			Int("account number", int(account)).
			Int("total recommendations", digest.Recommendations).
			Int("clusters affected", digest.ClustersAffected).
			Int("critical notifications", digest.CriticalNotifications).
			Int("important notifications", digest.ImportantNotifications).
			Msg("Producing weekly notification for ")

		notification := generateWeeklyNotificationMessage(&notificationEventURLs.InsightsAdvisor, fmt.Sprint(account), digest)
		msgBytes, err := json.Marshal(notification)
		if err != nil {
			log.Error().Err(err).Msg(invalidJSONContent)
			continue
		}
		_, _, err = kafkaNotifier.ProduceMessage(msgBytes)
		if err != nil {
			log.Error().
				Str(errorStr, err.Error()).
				Msg("Couldn't produce kafka event.")
			os.Exit(ExitStatusKafkaProducerError)
		}
	}
}

// processClusters function creates desired notification messages for all the clusters obtained from the database
func processClusters(config conf.ConfigStruct, ruleContent types.RulesMap,
	storage Storage, clusters []types.ClusterEntry) {
	if notificationType == types.InstantNotif {
		processReportsByCluster(config, ruleContent, storage, clusters)
	} else if notificationType == types.WeeklyDigest {
		processAllReportsFromCurrentWeek(ruleContent, storage, clusters)
	}
}

// setupKafkaProducer function creates a Kafka producer using the provided configuration
func setupKafkaProducer(config conf.ConfigStruct) {
	// broker enable/disable is very important information, let's inform
	// admins about the state
	if !conf.GetKafkaBrokerConfiguration(config).Enabled {
		kafkaNotifier = &disabled.Producer{}
		log.Info().Msg("Broker config for Notification Service is disabled")
		return
	}
	log.Info().Msg("Broker config for Notification Service is enabled")

	kafkaProducer, err := kafka.New(config)
	if err != nil {
		ProducerSetupErrors.Inc()
		log.Error().
			Str(errorStr, err.Error()).
			Msg("Couldn't initialize Kafka producer with the provided config.")
		os.Exit(ExitStatusKafkaBrokerError)
	}
	kafkaNotifier = kafkaProducer
	log.Info().Msg("Kafka producer ready")
}

func setupServiceLogProducer(config conf.ConfigStruct) {
	// broker enable/disable is very important information, let's inform
	// admins about the state
	serviceLogConfig := conf.GetServiceLogConfiguration(config)
	if !serviceLogConfig.Enabled {
		serviceLogNotifier = &disabled.Producer{}
		log.Info().Msg("Service Log config for Notification Service is disabled")
		return
	}
	log.Info().Msg("Service Log config for Notification Service is enabled")

	conn, err := ocmclient.NewOCMClient(serviceLogConfig.ClientID, serviceLogConfig.ClientSecret,
		serviceLogConfig.URL, serviceLogConfig.TokenURL)
	if err != nil {
		log.Error().Err(err).Msg("got error while setting up the connection to OCM API gateway")
		return
	}
	serviceLogProducer, err := servicelog.New(serviceLogConfig, conn)
	if err != nil {
		ProducerSetupErrors.Inc()
		log.Error().
			Str(errorStr, err.Error()).
			Msg("Couldn't initialize Service Log producer with the provided config.")
		os.Exit(ExitStatusServiceLogError)
	}
	serviceLogNotifier = serviceLogProducer
}

// generateInstantNotificationMessage function generates a notification message
// container with no events for a given account+cluster
func generateInstantNotificationMessage(
	clusterURI *string, accountID, orgID, clusterID string) (
	notification types.NotificationMessage) {
	events := []types.Event{}
	context := toJSONEscapedString(types.NotificationContext{
		notificationContextDisplayName: clusterID,
		notificationContextHostURL:     strings.Replace(*clusterURI, "{cluster_id}", clusterID, 1),
	})
	if context == "" {
		log.Error().Msg(contextToEscapedStringError)
		return
	}

	notification = types.NotificationMessage{
		Bundle:      notificationBundleName,
		Application: notificationApplicationName,
		EventType:   types.InstantNotif.ToString(),
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		AccountID:   accountID,
		OrgID:       orgID,
		Events:      events,
		Context:     context,
	}
	return
}

// generateWeeklyNotificationMessage function generates a notification message with one event based on the provided digest
func generateWeeklyNotificationMessage(advisorURI *string, accountID string, digest types.Digest) (notification types.NotificationMessage) {
	context := toJSONEscapedString(types.NotificationContext{
		notificationContextAdvisorURL: *advisorURI,
	})
	if context == "" {
		log.Error().Msg(contextToEscapedStringError)
		return
	}

	payload := toJSONEscapedString(types.EventPayload{
		notificationPayloadTotalClusters:        fmt.Sprint(digest.ClustersAffected),
		notificationPayloadTotalRecommendations: fmt.Sprint(digest.Recommendations),
		notificationPayloadTotalIncidents:       fmt.Sprint(digest.Incidents),
		notificationPayloadTotalCritical:        fmt.Sprint(digest.CriticalNotifications),
		notificationPayloadTotalImportant:       fmt.Sprint(digest.ImportantNotifications),
	})
	if payload == "" {
		log.Error().Msg("Notification message will not be generated as payload couldn't be converted to escaped string.")
		return
	}

	events := []types.Event{
		{
			// The insights Notifications backend expects this
			// field to be an empty object in the received JSON
			Metadata: types.EventMetadata{},
			// The insights Notifications backend expects to
			// receive the payload as a string with all its fields
			// as escaped strings
			Payload: payload,
		},
	}

	notification = types.NotificationMessage{
		Bundle:      notificationBundleName,
		Application: notificationApplicationName,
		EventType:   types.WeeklyDigest.ToString(),
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		AccountID:   accountID,
		Events:      events,
		Context:     context,
	}
	return
}

func generateNotificationPayloadURL(
	ruleURI *string, clusterID string, module types.ModuleName, errorKey types.ErrorKey) (
	notificationPayloadURL string) {
	parsedModule := strings.ReplaceAll(string(module), ".", "|")
	replacer := strings.NewReplacer("{cluster_id}", clusterID, "{module}", parsedModule, "{error_key}", string(errorKey))
	notificationPayloadURL = replacer.Replace(*ruleURI)
	return
}

// appendEventToNotificationMessage function adds a new event to the given notification message after constructing the payload string
func appendEventToNotificationMessage(notificationPayloadURL string, notification *types.NotificationMessage, ruleDescription string, totalRisk int, publishDate string) {

	payload := toJSONEscapedString(types.EventPayload{
		notificationPayloadRuleDescription: ruleDescription,
		notificationPayloadRuleURL:         notificationPayloadURL,
		notificationPayloadTotalRisk:       fmt.Sprint(totalRisk),
		notificationPayloadPublishDate:     publishDate,
	})
	if payload == "" {
		log.Error().Msg(contextToEscapedStringError)
		return
	}
	event := types.Event{
		// The insights Notifications backend expects this field to be
		// an empty object in the received JSON
		Metadata: types.EventMetadata{},
		// The insights Notifications backend expects to receive the
		// payload as a string with all its fields as escaped strings
		Payload: payload,
	}
	notification.Events = append(notification.Events, event)
}

// toJSONEscapedString function turns any valid JSON to a string-escaped string
func toJSONEscapedString(i interface{}) string {
	b, err := json.Marshal(i)
	if err != nil {
		log.Err(err).Msg(invalidJSONContent)
	}
	s := string(b)
	return s
}

// checkArgs function handles command line options passed to the process
func checkArgs(args *types.CliFlags) {
	switch {
	case args.ShowVersion:
		showVersion()
		os.Exit(ExitStatusOK)
	case args.ShowAuthors:
		showAuthors()
		os.Exit(ExitStatusOK)
	case args.ShowConfiguration:
		// config not loaded yet, just skip the rest of function for
		// now
		return
	case args.PrintNewReportsForCleanup,
		args.PerformNewReportsCleanup,
		args.PrintOldReportsForCleanup,
		args.PerformOldReportsCleanup:
		// DB only operations, no need for additional args
		return
	default:
	}

	// check if report type is specified on command line
	if !args.InstantReports && !args.WeeklyReports {
		log.Error().Msg("Type of report needs to be specified on command line")
		os.Exit(ExitStatusConfiguration)
	}

	if args.InstantReports {
		notificationType = types.InstantNotif
	} else {
		notificationType = types.WeeklyDigest
	}
}

func setupNotificationTypes(storage *DBStorage) {
	err := getNotificationTypes(storage)
	if err != nil {
		log.Err(err).Msg("Read notification types")
		os.Exit(ExitStatusStorageError)
	}
}

func setupNotificationStates(storage *DBStorage) {
	err := getStates(storage)
	if err != nil {
		log.Err(err).Msg("Read states")
		os.Exit(ExitStatusStorageError)
	}
}

// registerMetrics registers metrics using the provided namespace, if any
func registerMetrics(metricsConfig conf.MetricsConfiguration) {
	if metricsConfig.Namespace != "" {
		log.Info().Str("namespace", metricsConfig.Namespace).Msg("Setting metrics namespace")
		AddMetricsWithNamespaceAndSubsystem(
			metricsConfig.Namespace,
			metricsConfig.Subsystem)
	}
}

func closeStorage(storage *DBStorage) error {
	err := storage.Close()
	if err != nil {
		log.Err(err).Msg(operationFailedMessage)
		return err
	}
	return nil
}

func closeNotifier(notifier producer.Producer) error {
	err := notifier.Close()
	if err != nil {
		log.Err(err).Msg(operationFailedMessage)
		return err
	}
	return nil
}

func pushMetrics(metricsConf conf.MetricsConfiguration) {
	err := PushCollectedMetrics(metricsConf)
	if err != nil {
		log.Err(err).Msg(metricsPushFailedMessage)
		if metricsConf.RetryAfter == 0 || metricsConf.Retries == 0 {
			os.Exit(ExitStatusMetricsError)
		}
		for i := metricsConf.Retries; i > 0; i-- {
			time.Sleep(metricsConf.RetryAfter)
			log.Info().Msgf("Push metrics. Retrying (%d/%d attempts left)", i, metricsConf.Retries)
			err = PushCollectedMetrics(metricsConf)
			if err == nil {
				log.Info().Msg("Metrics pushed successfully. Terminating notification service successfully.")
				return
			}
			log.Err(err).Msg(metricsPushFailedMessage)
		}
		os.Exit(ExitStatusMetricsError)
	}
	log.Info().Msg("Metrics pushed successfully. Terminating notification service successfully.")
}

func deleteOperationSpecified(cliFlags types.CliFlags) bool {
	return cliFlags.PrintNewReportsForCleanup ||
		cliFlags.PerformNewReportsCleanup ||
		cliFlags.PrintOldReportsForCleanup ||
		cliFlags.PerformOldReportsCleanup
}

func assertNotificationDestination(config conf.ConfigStruct) {
	if !conf.GetKafkaBrokerConfiguration(config).Enabled && !conf.GetServiceLogConfiguration(config).Enabled {
		log.Error().Msg(destinationNotSet)
		os.Exit(ExitStatusConfiguration)
	}
}

func retrievePreviouslyReportedForEventTarget(storage *DBStorage, cooldown string, clusters []types.ClusterEntry, target types.EventTarget) {
	log.Info().Msg("Reading previously reported issues for given cluster list...")
	var err error
	previouslyReported[target], err = storage.ReadLastNotifiedRecordForClusterList(clusters, cooldown, target)
	if err != nil {
		ReadReportedErrors.Inc()
		log.Err(err).Msg(operationFailedMessage)
		os.Exit(ExitStatusStorageError)
	}
	log.Info().Int("target", int(target)).Int("retrieved", len(previouslyReported[target])).Msg("Done reading previously reported issues still in cooldown")
}

func retrievePreviouslyReportedReports(config conf.ConfigStruct, storage *DBStorage, clusters []types.ClusterEntry) {
	cooldown := conf.GetNotificationsConfiguration(config).Cooldown
	if conf.GetKafkaBrokerConfiguration(config).Enabled {
		retrievePreviouslyReportedForEventTarget(storage, cooldown, clusters, types.NotificationBackendTarget)
	}
	if conf.GetServiceLogConfiguration(config).Enabled {
		retrievePreviouslyReportedForEventTarget(storage, cooldown, clusters, types.ServiceLogTarget)
	}
}

func startDiffer(config conf.ConfigStruct, storage *DBStorage, verbose bool) {
	log.Info().Msg("Differ started")
	log.Info().Msg(separator)

	if verbose {
		showConfiguration(config)
	}

	assertNotificationDestination(config)
	registerMetrics(conf.GetMetricsConfiguration(config))
	log.Info().Msg(separator)
	log.Info().Msg("Getting rule content and impacts from content service")

	ruleContent, err := fetchAllRulesContent(conf.GetDependenciesConfiguration(config))
	if err != nil {
		FetchContentErrors.Inc()
		os.Exit(ExitStatusFetchContentError)
	}

	log.Info().Msg(separator)
	log.Info().Msg("Read cluster list")

	notifConfig := conf.GetNotificationsConfiguration(config)

	setupNotificationURLs(notifConfig)
	setupFiltersAndThresholds(config)
	setupNotificationStates(storage)
	setupNotificationTypes(storage)

	clusters, err := storage.ReadClusterList()
	if err != nil {
		ReadClusterListErrors.Inc()
		log.Err(err).Msg(operationFailedMessage)
		os.Exit(ExitStatusStorageError)
	}

	// filter clusters according to allow list and block list
	clusters, statistic := filterClusterList(clusters, conf.GetProcessingConfiguration(config))
	log.Info().
		Int("On input", statistic.Input).
		Int("Allowed", statistic.Allowed).
		Int("Blocked", statistic.Blocked).
		Int("Filtered", statistic.Filtered).
		Msg("Filter cluster list")

	entries := len(clusters)
	if entries == 0 {
		log.Info().Msg("Differ finished")
		os.Exit(ExitStatusOK)
	}
	log.Info().Int(clustersAttribute, entries).Msg("Read cluster list: done")

	log.Info().Msg(separator)
	retrievePreviouslyReportedReports(config, storage, clusters)
	log.Info().Msg(separator)

	log.Info().Msg("Preparing Kafka producer")
	setupKafkaProducer(config)
	log.Info().Msg(separator)
	log.Info().Msg("Preparing Service Log producer")
	setupServiceLogProducer(config)
	log.Info().Msg("Service Log producer ready")
	log.Info().Msg(separator)
	log.Info().Msg("Checking new issues for all new reports")
	processClusters(config, ruleContent, storage, clusters)
	log.Info().Int(clustersAttribute, entries).Msg("Process Clusters Entries: done")
	closeDiffer(storage)
	log.Info().Msg("Differ finished. Pushing metrics to the configured prometheus gateway.")
	pushMetrics(conf.GetMetricsConfiguration(config))
	log.Info().Msg(separator)
}

func setupNotificationURLs(config conf.NotificationsConfiguration) {
	notificationEventURLs.ClusterDetails = config.ClusterDetailsURI
	notificationEventURLs.RuleDetails = config.RuleDetailsURI
	notificationEventURLs.InsightsAdvisor = config.InsightsAdvisorURL
}

func closeDiffer(storage *DBStorage) {
	log.Info().Msg(separator)
	err := closeStorage(storage)
	if err != nil {
		defer os.Exit(ExitStatusStorageError)
	}
	log.Info().Msg(separator)
	err = closeNotifier(kafkaNotifier)
	if err != nil {
		defer os.Exit(ExitStatusKafkaBrokerError)
	}
	log.Info().Msg(separator)
	err = closeNotifier(serviceLogNotifier)
	if err != nil {
		defer os.Exit(ExitStatusServiceLogError)
	}
	log.Info().Msg(separator)
}

func setupFiltersAndThresholds(config conf.ConfigStruct) {
	kafkaEventThresholds.Likelihood = conf.GetKafkaBrokerConfiguration(config).LikelihoodThreshold
	kafkaEventThresholds.Impact = conf.GetKafkaBrokerConfiguration(config).ImpactThreshold
	kafkaEventThresholds.Severity = conf.GetKafkaBrokerConfiguration(config).SeverityThreshold
	kafkaEventThresholds.TotalRisk = conf.GetKafkaBrokerConfiguration(config).TotalRiskThreshold
	kafkaEventFilter = conf.GetKafkaBrokerConfiguration(config).EventFilter

	if kafkaEventFilter == "" {
		err := fmt.Errorf("Configuration problem")
		log.Err(err).Msg(eventFilterNotSetMessage)
		os.Exit(ExitStatusEventFilterError)
	}

	serviceLogEventThresholds.Likelihood = conf.GetServiceLogConfiguration(config).LikelihoodThreshold
	serviceLogEventThresholds.Impact = conf.GetServiceLogConfiguration(config).ImpactThreshold
	serviceLogEventThresholds.Severity = conf.GetServiceLogConfiguration(config).SeverityThreshold
	serviceLogEventThresholds.TotalRisk = conf.GetServiceLogConfiguration(config).TotalRiskThreshold
	serviceLogEventFilter = conf.GetServiceLogConfiguration(config).EventFilter

	if serviceLogEventFilter == "" {
		err := fmt.Errorf("Configuration problem")
		log.Err(err).Msg(eventFilterNotSetMessage)
		os.Exit(ExitStatusEventFilterError)
	}
}

// Run function is entry point to the differ
func Run() {
	var cliFlags types.CliFlags

	// define and parse all command line options
	flag.BoolVar(&cliFlags.InstantReports, "instant-reports", false, "create instant reports")
	flag.BoolVar(&cliFlags.WeeklyReports, "weekly-reports", false, "create weekly reports")
	flag.BoolVar(&cliFlags.ShowVersion, "show-version", false, "show version and exit")
	flag.BoolVar(&cliFlags.ShowAuthors, "show-authors", false, "show authors and exit")
	flag.BoolVar(&cliFlags.ShowConfiguration, "show-configuration", false, "show configuration and exit")
	flag.BoolVar(&cliFlags.PrintNewReportsForCleanup, "print-new-reports-for-cleanup", false, "print new reports to be cleaned up")
	flag.BoolVar(&cliFlags.PerformNewReportsCleanup, "new-reports-cleanup", false, "perform new reports clean up")
	flag.BoolVar(&cliFlags.PrintOldReportsForCleanup, "print-old-reports-for-cleanup", false, "print old reports to be cleaned up")
	flag.BoolVar(&cliFlags.PerformOldReportsCleanup, "old-reports-cleanup", false, "perform old reports clean up")
	flag.BoolVar(&cliFlags.CleanupOnStartup, "cleanup-on-startup", false, "perform database clean up on startup")
	flag.BoolVar(&cliFlags.Verbose, "verbose", false, "verbose logs")
	flag.StringVar(&cliFlags.MaxAge, "max-age", "", "max age for displaying/cleaning old records")
	flag.Parse()
	checkArgs(&cliFlags)

	// config has exactly the same structure as *.toml file
	config, err := conf.LoadConfiguration(configFileEnvVariableName, defaultConfigFileName)
	if err != nil {
		log.Err(err).Msg("Load configuration")
		os.Exit(ExitStatusConfiguration)
	}

	// configuration is loaded, so it would be possible to display it if
	// asked by user
	if cliFlags.ShowConfiguration {
		showConfiguration(config)
		os.Exit(ExitStatusOK)
	}

	// override default value by one read from configuration file
	if cliFlags.MaxAge == "" {
		cliFlags.MaxAge = config.Cleaner.MaxAge
	}

	if config.Logging.Debug {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	// prepare the storage
	storageConfiguration := conf.GetStorageConfiguration(config)
	storage, err := NewStorage(storageConfiguration)
	if err != nil {
		StorageSetupErrors.Inc()
		log.Err(err).Msg(operationFailedMessage)
		os.Exit(ExitStatusStorageError)
	}

	if deleteOperationSpecified(cliFlags) {
		err := PerformCleanupOperation(storage, cliFlags)
		if err != nil {
			os.Exit(ExitStatusCleanerError)
		} else {
			os.Exit(ExitStatusOK)
		}
	}

	// perform database cleanup on startup if specified on command line
	if cliFlags.CleanupOnStartup {
		err := PerformCleanupOnStartup(storage, cliFlags)
		if err != nil {
			os.Exit(ExitStatusCleanerError)
		}
		// if previous operation is correct, just continue
	}

	startDiffer(config, storage, cliFlags.Verbose)
}
