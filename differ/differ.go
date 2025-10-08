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

// Package differ contains core of CCX Notification Service. Differ itself is
// implemented there, together with storage and comparator implementations.
package differ

// Generated documentation is available at:
// https://pkg.go.dev/github.com/RedHatInsights/ccx-notification-service/differ
//
// Documentation in literate-programming-style is available at:
// https://redhatinsights.github.io/ccx-notification-service/packages/differ/d.html

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/RedHatInsights/ccx-notification-service/ocmclient"
	"github.com/RedHatInsights/ccx-notification-service/producer/servicelog"

	"github.com/RedHatInsights/ccx-notification-service/producer/kafka"

	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/producer"
	"github.com/RedHatInsights/ccx-notification-service/types"
	"github.com/RedHatInsights/insights-operator-utils/evaluator"
	"github.com/RedHatInsights/insights-operator-utils/logger"
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

// Total risk values
const (
	// TotalRiskLow is the numerical representation of 'Low' total risk
	TotalRiskLow = 1
	// TotalRiskModerate is the numerical representation of 'Moderate' total risk
	TotalRiskModerate = iota + 1
	// TotalRiskImportant   is the numerical representation of 'Important  ' total risk
	TotalRiskImportant
	// TotalRiskCritical is the numerical representation of 'Critical' total risk
	TotalRiskCritical
	// TotalRiskMax is the highest total risk handled
	TotalRiskMax = TotalRiskCritical
	// TotalRiskMin is the lowest total risk handled
	TotalRiskMin = TotalRiskLow
)

// Messages
const (
	serviceName                 = "CCX Notification Service"
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
	reportWithHighImpactMessage = "Report with impact higher than configured threshold detected"
	invalidJSONContent          = "The provided content cannot be encoded as JSON."
	metricsPushFailedMessage    = "Couldn't push prometheus metrics"
	tagsNotSetMessage           = "Tags for tag filter not set"
	evaluationErrorMessage      = "Evaluation error"
	serviceLogSendErrorMessage  = "Sending entry to service log failed for this report"
	renderReportsFailedMessage  = "Rendering reports failed for this cluster"
	ReportNotFoundError         = "report for rule ID %v and error key %v has not been found"
	destinationNotSet           = "No known event destination configured. Aborting."
	onlyOneDestinationAllowed   = "Only one integration should be enabled (Kafka / Service log. Review your config."
	configurationProblem        = "Configuration problem"
	noEquivalentSeverityMessage = "No Service log severity defined for given total risk. Creating event with Info severity"
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
)

// Constants for notification context expected fields
const (
	notificationContextDisplayName = "display_name"
	notificationContextHostURL     = "host_url"
)

// Constants used to filter events
const (
	DefaultTotalRiskThreshold = 2
	DefaultEventFilter        = "totalRisk >= totalRiskThreshold"
)

// Constants used for creating Service Log entries - there is a length limit on text fields in Service Log,
// which will return an error status code in case this limit is exceeded
const (
	serviceLogSummaryMaxLength     = 255
	serviceLogDescriptionMaxLength = 4000

	ServiceLogSeverityInfo     = "Info"
	ServiceLogSeverityWarning  = "Warning"
	ServiceLogSeverityMajor    = "Major"
	ServiceLogSeverityCritical = "Critical"
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

// Differ is the struct that holds all the dependencies and configuration of this service
type Differ struct {
	Storage            Storage
	Notifier           producer.Producer
	NotificationType   types.EventType
	Target             types.EventTarget
	PreviouslyReported types.NotifiedRecordsPerCluster
	CoolDown           string
	Thresholds         EventThresholds
	Filter             string
	FilterByTag        bool
	TagsSet            types.TagsSet
}

// TODO: same way we have a Differ struct now, we should have a struct
// holding the details of each notification target instead of global
// variables.
var (
	notificationType      types.EventType
	notificationEventURLs NotificationURLs
	serviceLogSeverityMap map[int]string
)

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
	return types.RuleName(ruleID[strings.LastIndex(string(ruleID), ".")+1:])
}

func findRuleByNameAndErrorKey(
	ruleContent types.RulesMap, ruleName types.RuleName, errorKey types.ErrorKey) (
	likelihood int, impact int, totalRisk int, description string, tags types.TagsSet) {
	rc := ruleContent[string(ruleName)]
	ek := rc.ErrorKeys
	val := ek[string(errorKey)]
	likelihood = val.Metadata.Likelihood
	description = val.Metadata.Description
	impact = val.Metadata.Impact.Impact
	totalRisk = calculateTotalRisk(likelihood, impact)
	tags = types.MakeSetOfTags(val.Metadata.Tags)
	return
}

// evaluateFilterExpression function tries to evaluate event filter expression
// based on provided threshold values and actual recommendation values
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

func setServiceLogSeverityMap() {
	serviceLogSeverityMap = make(map[int]string, 4)
	serviceLogSeverityMap[TotalRiskLow] = ServiceLogSeverityInfo
	serviceLogSeverityMap[TotalRiskModerate] = ServiceLogSeverityWarning
	serviceLogSeverityMap[TotalRiskImportant] = ServiceLogSeverityMajor
	serviceLogSeverityMap[TotalRiskCritical] = ServiceLogSeverityCritical
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

func createServiceLogEntry(report *types.RenderedReport, cluster types.ClusterEntry, createdBy, username, severity string) types.ServiceLogEntry {
	logEntry := types.ServiceLogEntry{
		ClusterUUID: cluster.ClusterName,
		Description: report.Reason,
		ServiceName: serviceName,
		Severity:    severity,
		Summary:     report.Description,
		CreatedBy:   createdBy,
		Username:    username,
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

// evaluateTagFilter checks if processed rule contains all required tags, for
// example tag "osd_customer".
func evaluateTagFilter(filterEnabled bool, tagsSet, reportItemTags types.TagsSet) bool {
	if !filterEnabled {
		return true
	}

	for neededTag := range tagsSet {
		if _, ok := reportItemTags[neededTag]; !ok {
			return false
		}
	}

	return true
}

func (d *Differ) getReportsWithIssuesToNotify(reports types.ReportContent, cluster types.ClusterEntry, ruleContent types.RulesMap) (reportsWithIssues types.ReportContent) {
	reportsWithIssues = make(types.ReportContent, 0, len(reports))

	for _, r := range reports {
		ruleName := moduleToRuleName(r.Module)
		errorKey := r.ErrorKey

		likelihood, impact, totalRisk, _, tags := findRuleByNameAndErrorKey(ruleContent, ruleName, errorKey)
		eventValue := EventValue{
			Likelihood: likelihood,
			Impact:     impact,
			TotalRisk:  totalRisk,
		}

		//TODO: Duplicated
		// try to evaluate event filter expression
		result, err := evaluateFilterExpression(d.Filter,
			d.Thresholds, eventValue)
		if err != nil {
			log.Err(err).Msg(evaluationErrorMessage)
			continue
		}

		// check if rule contains expected tag(s) if filtering by tags is enabled
		ruleTagCondition := evaluateTagFilter(d.FilterByTag, d.TagsSet, tags)

		// send message to target only if message pass both filters
		if result > 0 && ruleTagCondition {
			if !d.ShouldNotify(cluster, r) {
				NotificationNotSentSameState.Inc()
				continue
			}
			log.Debug().
				Str(typeAttribute, r.Type).
				Str(ruleAttribute, string(ruleName)).
				Str(errorKeyAttribute, string(errorKey)).
				Int(likelihoodAttribute, likelihood).
				Int(impactAttribute, impact).
				Int(totalRiskAttribute, totalRisk).
				Msg(reportWithHighImpactMessage)

			r.TotalRisk = eventValue.TotalRisk
			reportsWithIssues = append(reportsWithIssues, r)
		}
	}
	return
}

func getServiceLogSeverity(totalRisk int) string {
	if totalRisk >= TotalRiskMin && totalRisk <= TotalRiskMax {
		return serviceLogSeverityMap[totalRisk]
	}
	log.Warn().Int("total_risk", totalRisk).Msg(noEquivalentSeverityMessage)
	return ServiceLogSeverityInfo
}

func (d *Differ) createAndSendServiceLogEntry(configuration *conf.ConfigStruct, renderedReport *types.RenderedReport,
	totalRisk int, cluster types.ClusterEntry) error {
	// we need to pass the correct "created_by" and "username" attributes
	// to ServiceLog REST API
	serviceLogConfiguration := conf.GetServiceLogConfiguration(configuration)
	createdBy := serviceLogConfiguration.CreatedBy
	username := serviceLogConfiguration.Username

	logEntry := createServiceLogEntry(renderedReport, cluster, createdBy, username, getServiceLogSeverity(totalRisk))

	msgBytes, err := json.Marshal(logEntry)
	if err != nil {
		log.Error().Err(err).Msg(invalidJSONContent)
		return nil
	}

	log.Debug().
		Str(clusterAttribute, string(cluster.ClusterName)).
		Str("message", string(msgBytes)).
		Msg("Producing service log message")
	_, _, err = d.Notifier.ProduceMessage(msgBytes)
	if err != nil {
		NotificationNotSentErrorState.Inc()
		log.Err(err).
			Str(clusterAttribute, string(cluster.ClusterName)).
			Str(ruleAttribute, string(renderedReport.RuleID)).
			Str(errorKeyAttribute, string(renderedReport.ErrorKey)).
			Msg(serviceLogSendErrorMessage)
		return err
	}
	NotificationSent.Inc()
	return nil
}

// ProduceEntriesToServiceLog sends an entry to the service log integration
// for each issue found in the given reports
func (d *Differ) ProduceEntriesToServiceLog(configuration *conf.ConfigStruct, cluster types.ClusterEntry,
	rules types.Rules, ruleContent types.RulesMap, reports types.ReportContent) (totalMessages int, err error) {

	//TODO: Use pointer when passing around clusterEntry
	reportsToRender := d.getReportsWithIssuesToNotify(reports, cluster, ruleContent)

	if len(reportsToRender) != 0 {
		dependenciesConfiguration := conf.GetDependenciesConfiguration(configuration)
		renderedReports, err := renderReportsForCluster(
			&dependenciesConfiguration, cluster.ClusterName,
			reportsToRender, rules)

		if err != nil {
			log.Err(err).
				Str("cluster name", string(cluster.ClusterName)).
				Msg(renderReportsFailedMessage)
			return totalMessages, err
		}

		for _, r := range reportsToRender {
			ruleName := moduleToRuleName(r.Module)
			errorKey := r.ErrorKey

			ReportWithHighImpact.Inc()
			renderedReport, err := findRenderedReport(renderedReports, ruleName, errorKey)

			if err != nil {
				log.Err(err).Msgf("Output from content template renderer does not contain "+
					"result for cluster %s, rule %s and error key %s", cluster.ClusterName, ruleName, errorKey)
				continue
			}

			addDetailedInfoURLToRenderedReport(&renderedReport, &configuration.ServiceLog.RuleDetailsURI)

			if err = d.createAndSendServiceLogEntry(configuration, &renderedReport, r.TotalRisk, cluster); err != nil {
				continue
			}

			totalMessages++
		}
	}

	return totalMessages, nil
}

func (d *Differ) produceEntriesToKafka(cluster types.ClusterEntry, ruleContent types.RulesMap,
	reportItems types.ReportContent, report types.ClusterReport) (int, error) {

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
		likelihood, impact, totalRisk, description, tags := findRuleByNameAndErrorKey(ruleContent, ruleName, errorKey)
		eventValue := EventValue{
			Likelihood: likelihood,
			Impact:     impact,
			TotalRisk:  totalRisk,
		}

		// try to evaluate event filter expression
		result, err := evaluateFilterExpression(d.Filter,
			d.Thresholds, eventValue)
		if err != nil {
			log.Err(err).Msg(evaluationErrorMessage)
			continue
		}

		// check if rule contains expected tag(s) if filtering by tags is enabled
		ruleTagCondition := evaluateTagFilter(d.FilterByTag, d.TagsSet, tags)

		// send message to target only if message pass both filters
		if result > 0 && ruleTagCondition {
			if !d.ShouldNotify(cluster, r) {
				continue
			}
			log.Debug().
				Str(typeAttribute, r.Type).
				Str(ruleAttribute, string(ruleName)).
				Str(errorKeyAttribute, string(errorKey)).
				Int(likelihoodAttribute, likelihood).
				Int(impactAttribute, impact).
				Int(totalRiskAttribute, totalRisk).
				Msg(reportWithHighImpactMessage)
			ReportWithHighImpact.Inc()
			notificationPayloadURL := generateNotificationPayloadURL(&notificationEventURLs.RuleDetails, string(cluster.ClusterName), module, errorKey)
			appendEventToNotificationMessage(notificationPayloadURL, &notificationMsg, description, totalRisk, time.Time(cluster.UpdatedAt).UTC().Format(time.RFC3339Nano))
		}
	}

	if len(notificationMsg.Events) == 0 {
		updateNotificationRecordSameState(d.Storage, cluster, report, notifiedAt, types.NotificationBackendTarget)
		return 0, nil
	}

	log.Debug().
		Str(clusterAttribute, string(cluster.ClusterName)).
		Int(numberOfEventsAttribute, len(notificationMsg.Events)).
		Msg("Producing instant notification")

	msgBytes, err := json.Marshal(notificationMsg)
	if err != nil {
		log.Error().Err(err).Msg(invalidJSONContent)
		return -1, err
	}
	_, offset, err := d.Notifier.ProduceMessage(msgBytes)
	if err != nil {
		log.Error().
			Str(errorStr, err.Error()).
			Msg("Couldn't send notification message to kafka topic.")
		updateNotificationRecordErrorState(d.Storage, err, cluster, report, notifiedAt, types.NotificationBackendTarget)
		return -1, err
	}

	if offset != -1 {
		// update the database if any message is sent (not a DisabledProducer)
		log.Debug().Msg("notifier is not disabled so DB is updated")
		updateNotificationRecordSentState(d.Storage, cluster, report, notifiedAt, types.NotificationBackendTarget)
		return len(notificationMsg.Events), nil
	}
	return 0, nil
}

// checkReadError function checks whether reading from read_errors table was
// successful. This log gives important context as the insertion into the
// read_error will fail if there's already a row but we couldn't read it for
// whatever reason.
func checkReadError(err error) {
	if err != nil {
		log.Err(err).Msg("read_errors read access error")
	}
}

// checkWriteError function checks whether writing into read_errors table was
// successful
func checkWriteError(err error) {
	if err != nil {
		log.Err(err).Msg("read_errors write access error")
	}
}

func (d *Differ) processReportsByCluster(config *conf.ConfigStruct, ruleContent types.RulesMap, clusters []types.ClusterEntry) {
	notifiedIssues := 0
	clustersCount := len(clusters)
	skippedEntries := 0
	emptyEntries := 0

	var rules types.Rules
	if conf.GetServiceLogConfiguration(config).Enabled {
		setServiceLogSeverityMap()
		rules = getAllContentFromMap(ruleContent)
	}

	for i, cluster := range clusters {
		log.Debug().
			Int("#", i).
			Int("of", clustersCount).
			Int(organizationIDAttribute, int(cluster.OrgID)).
			Int(AccountNumberAttribute, int(cluster.AccountNumber)).
			Str(clusterAttribute, string(cluster.ClusterName)).
			Msg(clusterEntryMessage)

		timer := TimeOperation("read_report_for_cluster")
		report, err := d.Storage.ReadReportForClusterAtTime(cluster.OrgID, cluster.ClusterName, cluster.UpdatedAt)
		timer()
		if err != nil {
			// is the problem reported already?
			reportedAlready, readErr := d.Storage.ReadErrorExists(cluster.OrgID, cluster.ClusterName, cluster.UpdatedAt)
			checkReadError(readErr)

			// if the error is reported already, skip to next one
			if reportedAlready {
				log.Debug().
					Int(organizationIDAttribute, int(cluster.OrgID)).
					Str(clusterAttribute, string(cluster.ClusterName)).
					Time("since", time.Time(cluster.UpdatedAt)).
					Msg("Read error already exists")
				continue
			}
			// if not reported, process the error
			ReadReportForClusterErrors.Inc()
			skippedEntries++
			log.Err(err).Msg(operationFailedMessage)
			writeErr := d.Storage.WriteReadError(cluster.OrgID, cluster.ClusterName, time.Time(cluster.UpdatedAt), err)
			checkWriteError(writeErr)
			continue
		}

		var deserialized types.Report
		err = json.Unmarshal([]byte(report), &deserialized)
		if err != nil {
			DeserializeReportErrors.Inc()
			skippedEntries++
			log.Err(err).Msg("Deserialization error - Couldn't create report object")
			log.Debug().Bytes("bytes", []byte(report)).Msg("Data to be deserialized")
			continue
		}

		if len(deserialized.Reports) == 0 {
			log.Debug().Msgf("No reports in notification database for cluster %s", cluster.ClusterName)
			emptyEntries++
			continue
		}

		if conf.GetServiceLogConfiguration(config).Enabled {
			notifiedAt := types.Timestamp(time.Now())
			timer = TimeOperation("send_notification_servicelog")
			newNotifiedIssues, err := d.ProduceEntriesToServiceLog(config, cluster, rules, ruleContent, deserialized.Reports)
			timer()
			timer = TimeOperation("update_notification_record_state_servicelog")
			updateNotificationRecordState(d.Storage, cluster, report, newNotifiedIssues, notifiedAt, types.ServiceLogTarget, err)
			timer()
			notifiedIssues += newNotifiedIssues
		}

		if !conf.GetKafkaBrokerConfiguration(config).Enabled {
			continue
		}
		timer = TimeOperation("send_notification_kafka")
		newNotifiedIssues, err := d.produceEntriesToKafka(cluster, ruleContent, deserialized.Reports, report)
		timer()
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

// ProcessClusters function creates desired notification messages for all the
// clusters obtained from the database
func (d *Differ) ProcessClusters(config *conf.ConfigStruct, ruleContent types.RulesMap,
	clusters []types.ClusterEntry) {
	if d.NotificationType == types.InstantNotif {
		d.processReportsByCluster(config, ruleContent, clusters)
	}
}

// SetupKafkaProducer function creates a Kafka producer using the provided configuration
func (d *Differ) SetupKafkaProducer(config *conf.ConfigStruct) error {
	kafkaProducer, err := kafka.New(config)
	if err != nil {
		ProducerSetupErrors.Inc()
		log.Error().
			Str(errorStr, err.Error()).
			Msg("Couldn't initialize Kafka producer with the provided config.")
		return &KafkaBrokerError{}
	}
	d.Notifier = kafkaProducer
	log.Info().Msg("Kafka producer ready")
	return nil
}

func (d *Differ) setupServiceLogProducer(config *conf.ConfigStruct) error {
	serviceLogConfig := conf.GetServiceLogConfiguration(config)
	conn, err := ocmclient.NewOCMClient(serviceLogConfig.ClientID, serviceLogConfig.ClientSecret,
		serviceLogConfig.URL, serviceLogConfig.TokenURL)
	if err != nil {
		log.Error().Err(err).Msg("got error while setting up the connection to OCM API gateway")
		return nil
	}
	serviceLogProducer, err := servicelog.New(&serviceLogConfig, conn)
	if err != nil {
		ProducerSetupErrors.Inc()
		log.Error().
			Str(errorStr, err.Error()).
			Msg("Couldn't initialize Service Log producer with the provided config.")
		return &ServiceLogError{Msg: err.Error()}
	}
	d.Notifier = serviceLogProducer
	log.Info().Msg("Service Log producer ready")
	return nil
}

// generateInstantNotificationMessage function generates a notification message
// container with no events for a given account+cluster
func generateInstantNotificationMessage(
	clusterURI *string, accountID, orgID, clusterID string) (
	notification types.NotificationMessage) {
	var events []types.Event
	notificationContext := types.NotificationContext{
		notificationContextDisplayName: clusterID,
		notificationContextHostURL:     strings.Replace(*clusterURI, "{cluster_id}", clusterID, 1),
	}

	notification = types.NotificationMessage{
		Bundle:      notificationBundleName,
		Application: notificationApplicationName,
		EventType:   types.InstantNotif.ToString(),
		Timestamp:   time.Now().UTC().Format(time.RFC3339Nano),
		AccountID:   accountID,
		OrgID:       orgID,
		Events:      events,
		Context:     notificationContext,
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

	payload := types.EventPayload{
		notificationPayloadRuleDescription: ruleDescription,
		notificationPayloadRuleURL:         notificationPayloadURL,
		notificationPayloadTotalRisk:       fmt.Sprint(totalRisk),
		notificationPayloadPublishDate:     publishDate,
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

func setupNotificationTypes(storage Storage) error {
	err := getNotificationTypes(storage)
	if err != nil {
		log.Err(err).Msg("Read notification types")
		return &StatusStorageError{}
	}
	return nil
}

func setupNotificationStates(storage Storage) error {
	err := getStates(storage)
	if err != nil {
		log.Err(err).Msg("Read states")
		return &StatusStorageError{}
	}
	return nil
}

// registerMetrics registers metrics using the provided namespace, if any
func registerMetrics(metricsConfig *conf.MetricsConfiguration) {
	if metricsConfig.Namespace == "" {
		return
	}
	log.Info().Str("namespace", metricsConfig.Namespace).Msg("Setting metrics namespace")
	AddMetricsWithNamespaceAndSubsystem(
		metricsConfig.Namespace,
		metricsConfig.Subsystem)
}

func closeStorage(storage Storage) error {
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

func pushMetrics(metricsConf *conf.MetricsConfiguration) error {
	if metricsConf.Namespace == "" || metricsConf.GatewayAuthToken == "" {
		log.Debug().Msg("No metrics configuration detected. Metrics will not be pushed")
		return nil
	}
	err := PushCollectedMetrics(metricsConf)
	if err != nil {
		log.Err(err).Msg(metricsPushFailedMessage)
		if metricsConf.RetryAfter == 0 || metricsConf.Retries == 0 {
			log.Warn().Msg("Metrics push failed, but continuing execution.")
			return nil
		}
		for i := metricsConf.Retries; i > 0; i-- {
			time.Sleep(metricsConf.RetryAfter)
			log.Info().Msgf("Push metrics. Retrying (%d/%d attempts left)", i, metricsConf.Retries)
			err = PushCollectedMetrics(metricsConf)
			if err == nil {
				log.Info().Msg("Metrics pushed successfully. Terminating notification service successfully.")
				return nil
			}
			log.Err(err).Msg(metricsPushFailedMessage)
		}
		log.Warn().Msg("All metric push attempts failed, but continuing execution.")
		return nil
	}
	log.Info().Msg("Metrics pushed successfully. Terminating notification service successfully.")
	return nil
}

func assertNotificationDestination(config *conf.ConfigStruct) error {
	if !conf.GetKafkaBrokerConfiguration(config).Enabled && !conf.GetServiceLogConfiguration(config).Enabled {
		log.Error().Msg(destinationNotSet)
		return &StatusConfiguration{}
	}
	if conf.GetKafkaBrokerConfiguration(config).Enabled && conf.GetServiceLogConfiguration(config).Enabled {
		log.Error().Msg(onlyOneDestinationAllowed)
		return &StatusConfiguration{}
	}
	return nil
}

// RetrievePreviouslyReportedForEventTarget reads previously reported issues
func (d *Differ) RetrievePreviouslyReportedForEventTarget(cooldown string, target types.EventTarget, clusters []types.ClusterEntry) error {
	log.Info().Msg("Reading previously reported issues for given cluster list...")
	var err error
	timer := TimeOperation("read_last_notified_record_for_cluster_list")
	d.PreviouslyReported, err = d.Storage.ReadLastNotifiedRecordForClusterList(clusters, cooldown, target)
	timer()
	if err != nil {
		ReadReportedErrors.Inc()
		log.Err(err).Msg(operationFailedMessage)
		return &StatusStorageError{}
	}
	log.Info().Int("target", int(target)).Int("retrieved", len(d.PreviouslyReported)).Msg("Done reading previously reported issues still in cool down")
	return nil
}

func (d *Differ) start(config *conf.ConfigStruct) error {
	SetServiceStatus(StatusStarting)
	defer SetServiceStatus(StatusInactive)

	log.Info().Msg("Differ started")
	log.Info().Msg(separator)

	metricsConfiguration := conf.GetMetricsConfiguration(config)
	registerMetrics(&metricsConfiguration)
	log.Info().Msg(separator)
	log.Info().Msg("Getting rule content and impacts from content service")

	SetServiceStatus(StatusFetchingContent)
	dependenciesConfiguration := conf.GetDependenciesConfiguration(config)
	timer := TimeOperation("fetch_content")
	ruleContent, err := fetchAllRulesContent(&dependenciesConfiguration)
	timer()
	if err != nil {
		FetchContentErrors.Inc()
		return &FetchStatusContentError{}
	}

	log.Info().Msg(separator)
	log.Info().Msg("Read cluster list")

	notifConfig := conf.GetNotificationsConfiguration(config)

	setupNotificationURLs(notifConfig)
	if err := setupNotificationStates(d.Storage); err != nil {
		return err
	}
	if err := setupNotificationTypes(d.Storage); err != nil {
		return err
	}
	go PushMetricsInLoop(context.Background(), &metricsConfiguration)

	SetServiceStatus(StatusReadingClusters)

	timer = TimeOperation("read_cluster_list")
	clusters, err := d.Storage.ReadClusterList()
	timer()
	if err != nil {
		ReadClusterListErrors.Inc()
		log.Err(err).Msg(operationFailedMessage)
		return &StatusStorageError{}
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
		return nil
	}
	log.Info().Int(clustersAttribute, entries).Msg("Read cluster list: done")
	log.Info().Msg(separator)

	SetServiceStatus(StatusReadingPreviouslyReported)
	timer = TimeOperation("read_previously_reported")
	err = d.RetrievePreviouslyReportedForEventTarget(d.CoolDown, d.Target, clusters)
	timer()
	if err != nil {
		return err
	}
	log.Info().Msg(separator)
	log.Info().Msg("Checking new issues for all new reports")

	SetServiceStatus(StatusProcessingClusters)
	timer = TimeOperation("process_clusters")
	d.ProcessClusters(config, ruleContent, clusters)
	timer()
	log.Info().Int(clustersAttribute, entries).Msg("Process Clusters Entries: done")
	if err := d.close(); err != nil {
		return err
	}
	log.Info().Msg("Differ finished. Pushing metrics to the configured prometheus gateway.")
	if err := pushMetrics(&metricsConfiguration); err != nil {
		return err
	}
	log.Info().Msg(separator)
	return nil
}

func setupNotificationURLs(config conf.NotificationsConfiguration) {
	notificationEventURLs.ClusterDetails = config.ClusterDetailsURI
	notificationEventURLs.RuleDetails = config.RuleDetailsURI
	notificationEventURLs.InsightsAdvisor = config.InsightsAdvisorURL
}

func errorForTarget(t types.EventTarget) error {
	if t == types.NotificationBackendTarget {
		return &KafkaBrokerError{}
	}
	if t == types.ServiceLogTarget {
		return &ServiceLogError{}
	}
	return nil
}

func (d *Differ) close() (err error) {
	log.Info().Msg(separator)
	if err = closeStorage(d.Storage); err != nil {
		err = &StatusStorageError{}
	}
	log.Info().Msg(separator)
	errNotifier := closeNotifier(d.Notifier)
	if errNotifier != nil {
		return errorForTarget(d.Target)
	}
	log.Info().Msg(separator)
	return
}

// SetupFiltersAndThresholds function setup both techniques that can be used to
// filter messages sent to targets (Notification backend and ServiceLog at this moment):
//  1. filter based on likelihood, impact, severity, and total risk
//  2. filter based on rule type that's identified by tags
func (d *Differ) SetupFiltersAndThresholds(config *conf.ConfigStruct) error {
	kafkaBrokerConfiguration := conf.GetKafkaBrokerConfiguration(config)
	if kafkaBrokerConfiguration.Enabled {
		d.Thresholds = EventThresholds{
			TotalRisk:  kafkaBrokerConfiguration.TotalRiskThreshold,
			Likelihood: kafkaBrokerConfiguration.LikelihoodThreshold,
			Impact:     kafkaBrokerConfiguration.ImpactThreshold,
			Severity:   kafkaBrokerConfiguration.SeverityThreshold,
		}
		if kafkaBrokerConfiguration.EventFilter == "" {
			d.Filter = DefaultEventFilter
		} else {
			d.Filter = kafkaBrokerConfiguration.EventFilter
		}
		// filtering by tags
		d.FilterByTag = kafkaBrokerConfiguration.TagFilterEnabled
		d.TagsSet = kafkaBrokerConfiguration.TagsSet

		// check if tags set is provided via configuration if filtering is enabled
		if d.FilterByTag && d.TagsSet == nil {
			err := &StatusEventFilterError{Msg: configurationProblem}
			log.Err(err).Msg(tagsNotSetMessage)
			return err
		}
		return nil
	}

	serviceLogConfiguration := conf.GetServiceLogConfiguration(config)
	if serviceLogConfiguration.Enabled {
		d.Thresholds = EventThresholds{
			TotalRisk:  serviceLogConfiguration.TotalRiskThreshold,
			Likelihood: serviceLogConfiguration.LikelihoodThreshold,
			Impact:     serviceLogConfiguration.ImpactThreshold,
			Severity:   serviceLogConfiguration.SeverityThreshold,
		}
		if serviceLogConfiguration.EventFilter == "" {
			d.Filter = DefaultEventFilter
		} else {
			d.Filter = serviceLogConfiguration.EventFilter
		}
		// filtering by tags
		d.FilterByTag = serviceLogConfiguration.TagFilterEnabled
		d.TagsSet = serviceLogConfiguration.TagsSet

		// check if tags set is provided via configuration if filtering is enabled
		if d.FilterByTag && d.TagsSet == nil {
			err := &StatusEventFilterError{Msg: configurationProblem}
			log.Err(err).Msg(tagsNotSetMessage)
			return err
		}
		return nil
	}
	return nil
}

// Run function is entry point to the differ
func Run(config conf.ConfigStruct, cliFlags types.CliFlags) int {

	defer logger.CloseZerolog()

	if cliFlags.InstantReports {
		notificationType = types.InstantNotif
	}

	// prepare the storage
	storageConfiguration := conf.GetStorageConfiguration(&config)
	storage, err := NewStorage(&storageConfiguration)
	if err != nil {
		StorageSetupErrors.Inc()
		log.Err(err).Msg(operationFailedMessage)
		return ExitStatusStorageError
	}

	if deleteOperationSpecified(cliFlags) {
		err := PerformCleanupOperation(storage, cliFlags)
		if err != nil {
			return ExitStatusCleanerError
		}
		return ExitStatusOK
	}

	// perform database cleanup on startup if specified on command line
	if cliFlags.CleanupOnStartup {
		err := PerformCleanupOnStartup(storage, cliFlags)
		if err != nil {
			return ExitStatusCleanerError
		}
		// if previous operation is correct, just continue
	}

	d, err := New(&config, storage)
	if err != nil {
		return selectError(err)
	}

	err = d.start(&config)
	return selectError(err)
}

// New constructs new implementation of Differ interface
func New(config *conf.ConfigStruct, storage Storage) (*Differ, error) {
	if err := assertNotificationDestination(config); err != nil {
		return nil, err
	}
	d := Differ{
		Storage:            storage,
		NotificationType:   notificationType,
		PreviouslyReported: make(types.NotifiedRecordsPerCluster),
		Thresholds:         EventThresholds{},
	}
	if conf.GetKafkaBrokerConfiguration(config).Enabled {
		d.Target = types.NotificationBackendTarget
		if err := d.SetupKafkaProducer(config); err != nil {
			return nil, err
		}
		d.CoolDown = conf.GetKafkaBrokerConfiguration(config).Cooldown
	} else if conf.GetServiceLogConfiguration(config).Enabled {
		d.Target = types.ServiceLogTarget
		if err := d.setupServiceLogProducer(config); err != nil {
			return nil, err
		}
		d.CoolDown = conf.GetServiceLogConfiguration(config).Cooldown
	}
	err := d.SetupFiltersAndThresholds(config)
	if err != nil {
		return nil, err
	}

	return &d, nil
}

func selectError(err error) int {
	if err == nil {
		return ExitStatusOK
	}

	switch err.(type) {
	case *FetchStatusContentError:
		return ExitStatusFetchContentError
	case *StatusStorageError:
		return ExitStatusStorageError
	case *KafkaBrokerError:
		return ExitStatusKafkaBrokerError
	case *ServiceLogError:
		return ExitStatusServiceLogError
	case *StatusMetricsError:
		return ExitStatusMetricsError
	case *StatusConfiguration:
		return ExitStatusConfiguration
	case *StatusEventFilterError:
		return ExitStatusEventFilterError
	}

	return ExitStatusOK
}
