/*
Copyright © 2020 Red Hat, Inc.

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

package main

import (
	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/types"
	"github.com/stretchr/testify/assert"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

var (
	brokerCfg = conf.KafkaConfiguration{
		Address: "localhost:9092",
		Topic:   "test_haberdasher",
		Timeout: time.Duration(30*10 ^ 9),
	}
	// Base UNIX time plus approximately 50 years (not long before year 2020).
	testTimestamp = time.Unix(50*365*24*60*60, 0)
)

func init() {
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
}

// Test the checkArgs function when flag for --show-version is set
func TestArgsParsingShowVersion(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=TestArgsParsingShowVersion")
	if os.Getenv("SHOW_VERSION_FLAG") == "1" {
		args := types.CliFlags{
			ShowVersion: true,
		}
		checkArgs(&args)
	}
	cmd.Env = append(os.Environ(), "SHOW_VERSION_FLAG=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		t.Fatalf("checkArgs exited with exit status %d, wanted exit status 0", e.ExitCode())
	}
}

// Test the checkArgs function when flag for --show-authors is set
func TestArgsParsingShowAuthors(t *testing.T) {
	if os.Getenv("SHOW_AUTHORS_FLAG") == "1" {
		args := types.CliFlags{
			ShowAuthors: true,
		}
		checkArgs(&args)
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestArgsParsingShowAuthors")
	cmd.Env = append(os.Environ(), "SHOW_AUTHORS_FLAG=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		t.Fatalf("checkArgs exited with exit status %d, wanted exit status 0", e.ExitCode())
	}
}

// Test the checkArgs function when no flag is set
func TestArgsParsingNoFlags(t *testing.T) {
	if os.Getenv("EXEC_NO_FLAG") == "1" {
		args := types.CliFlags{}
		checkArgs(&args)
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestArgsParsingNoFlags")
	cmd.Env = append(os.Environ(), "EXEC_NO_FLAG=1")
	err := cmd.Run()
	assert.NotNil(t, err)
	if exitError, ok := err.(*exec.ExitError); ok {
		assert.Equal(t, ExitStatusConfiguration, exitError.ExitCode())
		return
	}
	t.Fatalf("checkArgs didn't behave properly. Wanted exit status %d but got error\n %v", ExitStatusConfiguration, err)
}

// Test the checkArgs function when --instant-reports flag is set
func TestArgsParsingInstantReportsFlags(t *testing.T) {
	args := types.CliFlags{
		InstantReports: true,
	}
	checkArgs(&args)
	assert.Equal(t, notificationType, types.InstantNotif)
}

// Test the checkArgs function when --weekly-reports flag is set
func TestArgsParsingWeeklyReportsFlags(t *testing.T) {
	args := types.CliFlags{
		WeeklyReports: true,
	}
	checkArgs(&args)
	assert.Equal(t, notificationType, types.WeeklyDigest)
}

//---------------------------------------------------------------------------------------
func TestToJsonEscapedStringValidJson(t *testing.T) {
	type testStruct struct {
		Name         string
		Items        []string
		Id           int64
		privateField string // An unexported field is not encoded, so our method would not include it in output.
		Created      time.Time
	}

	tested := testStruct{
		Name:         "Random Name",
		Items:        []string{"Apple", "Dell", "Orange", "Piña"},
		Id:           999,
		privateField: "a random field that nobody can see cuz I don't wanna",
		Created:      time.Date(2021, time.Month(5), 25, 15, 15, 15, 15, time.Local),
	}

	expectedEscapedJsonString := "{\"Name\":\"Random Name\",\"Items\":[\"Apple\",\"Dell\",\"Orange\",\"Piña\"],\"Id\":999,\"Created\":\"2021-05-25T15:15:15.000000015+02:00\"}"

	assert.Equal(t, expectedEscapedJsonString, toJsonEscapedString(tested))
}

func TestToJsonEscapedStringInvalidJson(t *testing.T) {
	type testStruct struct {
		Id           int64
		InvalidField chan int //A value that cannot be represented in JSON
	}

	tested := testStruct{
		Id:           999,
		InvalidField: make(chan int),
	}

	expectedEscapedJsonString := ""

	assert.Equal(t, expectedEscapedJsonString, toJsonEscapedString(tested))
}

//---------------------------------------------------------------------------------------
func TestGenerateInstantReportNotificationMessage(t *testing.T) {
	clusterURI := "the_cluster_uri_in_ocm"
	accountID := "a_stringified_account_id"
	clusterID := "the_displayed_cluster_ID"

	notificationMsg := generateInstantNotificationMessage(clusterURI, accountID, clusterID)

	assert.NotEmpty(t, notificationMsg, "the generated notification message is empty")
	assert.Empty(t, notificationMsg.Events, "the generated notification message should not have any events")
	assert.Equal(t, types.InstantNotif.String(), notificationMsg.EventType, "the generated notification message should be for instant notifications")
	assert.Equal(t, "{\"display_name\":\"the_displayed_cluster_ID\",\"host_url\":\"the_cluster_uri_in_ocm\"}", notificationMsg.Context, "Notification context is different from expected.")
	assert.Equal(t, notificationBundleName, notificationMsg.Bundle, "Generated notifications should indicate 'openshift' as bundle")
	assert.Equal(t, notificationApplicationName, notificationMsg.Application, "Generated notifications should indicate 'openshift' as application name")
	assert.Equal(t, accountID, notificationMsg.AccountID, "Generated notifications does not have expected account ID")
}

func TestAppendEventsToExistingInstantReportNotificationMsg(t *testing.T) {
	clusterURI := "the_cluster_uri_in_ocm"
	accountID := "a_stringified_account_id"
	clusterID := "the_displayed_cluster_ID"
	notificationMsg := generateInstantNotificationMessage(clusterURI, accountID, clusterID)

	assert.Empty(t, notificationMsg.Events, "the generated notification message should not have any events")

	ruleURI := "a_given_uri/{rule}/details"
	ruleName := "a_given_rule_name"
	publishDate := "the_date_of_today"
	totalRisk := 1
	appendEventToNotificationMessage(ruleURI, &notificationMsg, ruleName, totalRisk, publishDate)
	assert.Equal(t, len(notificationMsg.Events), 1, "the notification message should have 1 event")
	assert.Equal(t, notificationMsg.Events[0].Metadata, types.EventMetadata{}, "All notification messages should have empty metadata")

	ruleURI = "a_given_uri_without_expected_format" // the function only does a basic strings.Replace, no check for if it went well or not
	appendEventToNotificationMessage(ruleURI, &notificationMsg, ruleName, totalRisk, publishDate)
	assert.Equal(t, len(notificationMsg.Events), 2, "the notification message should have 2 events")
	assert.Equal(t, notificationMsg.Events[1].Metadata, types.EventMetadata{}, "All notification messages should have empty metadata")
}

//---------------------------------------------------------------------------------------
func TestUpdateDigestNotificationCounters(t *testing.T) {
	digest := types.Digest{}
	expectedDigest := types.Digest{}
	updateDigestNotificationCounters(&digest, 1)
	assert.Equal(t, expectedDigest, digest, "No field in digest should be incremented if totalRisk == 1")
	updateDigestNotificationCounters(&digest, 2)
	assert.Equal(t, expectedDigest, digest, "No field in digest should be incremented if totalRisk == 2")
	updateDigestNotificationCounters(&digest, 3)
	expectedDigest.ImportantNotifications++
	assert.Equal(t, expectedDigest, digest, "ImportantNotifications field should be incremented if totalRisk == 3")
	updateDigestNotificationCounters(&digest, 4)
	expectedDigest.CriticalNotifications++
	assert.Equal(t, expectedDigest, digest, "CriticalNotifications field should be incremented if totalRisk == 4")
	updateDigestNotificationCounters(&digest, 5)
	assert.Equal(t, expectedDigest, digest, "No field in digest should be incremented if 3 < totalRisk or totalRisk > 4")
}

func TestGetNotificationDigestForCurrentAccount(t *testing.T) {
	emptyDigest := types.Digest{}
	existingDigest := types.Digest{
		ClustersAffected:       23,
		CriticalNotifications:  2,
		ImportantNotifications: 4,
		Recommendations:        8,
		Incidents:              0,
	}

	notificationsByAccount := map[types.AccountNumber]types.Digest{
		12345: existingDigest,
	}
	assert.Equal(t, emptyDigest, getNotificationDigestForCurrentAccount(notificationsByAccount, 100))
	assert.Equal(t, existingDigest, getNotificationDigestForCurrentAccount(notificationsByAccount, 12345))
}

func TestGenerateWeeklyDigestNotificationMessage(t *testing.T) {
	advisorURI := "the_uri_to_advisor_tab_in_ocm"
	accountID := "a_stringified_account_id"
	digest := types.Digest{}

	notificationMsg := generateWeeklyNotificationMessage(advisorURI, accountID, digest)

	assert.NotEmpty(t, notificationMsg, "the generated notification message is empty")
	assert.NotEmpty(t, notificationMsg.Events, "the generated notification message should have 1 event with digest's content")
	assert.Equal(t, types.WeeklyDigest.String(), notificationMsg.EventType, "the generated notification message should be for weekly digest")
	assert.Equal(t, notificationBundleName, notificationMsg.Bundle, "Generated notifications should indicate 'openshift' as bundle")
	assert.Equal(t, notificationApplicationName, notificationMsg.Application, "Generated notifications should indicate 'openshift' as application name")
	assert.Equal(t, accountID, notificationMsg.AccountID, "Generated notifications does not have expected account ID")
	assert.Equal(t, "{\"advisor_url\":\"the_uri_to_advisor_tab_in_ocm\"}", notificationMsg.Context, "Notification context is different from expected.")

	assert.Equal(t, 1, len(notificationMsg.Events)) //Only one event, always, with the digest's content
	expectedEvent := types.Event{
		Metadata: types.EventMetadata{},
		Payload:  "{\"total_clusters\":\"0\",\"total_critical\":\"0\",\"total_important\":\"0\",\"total_incidents\":\"0\",\"total_recommendations\":\"0\"}",
	}
	assert.Equal(t, expectedEvent, notificationMsg.Events[0])
}
