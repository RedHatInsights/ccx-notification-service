/*
Copyright © 2021 Red Hat, Inc.

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

package types

// Generated documentation is available at:
// https://pkg.go.dev/github.com/RedHatInsights/ccx-notification-service/types
//
// Documentation in literate-programming-style is available at:
// https://redhatinsights.github.io/ccx-notification-service/packages/types/types.html

import (
	"encoding/json"
	"fmt"
	"time"

	types "github.com/RedHatInsights/insights-results-types"
)

// Timestamp represents any timestamp in a form gathered from database
// TODO: need to be improved
type Timestamp time.Time

// KafkaOffset is a data type representing offset in Kafka topic.
type KafkaOffset int64

// OrgID data type represents organization ID.
type OrgID uint32

// AccountNumber represents account number for a given report.
type AccountNumber uint32

// ClusterName data type represents name of cluster in format
// c8590f31-e97e-4b85-b506-c45ce1911a12 (ie. in UUID format).
type ClusterName string

// ClusterReport represents cluster report
type ClusterReport string

// DBDriver type for db driver enum
type DBDriver int

// NotificationTypeID represents ID value in `notifiation_types` table.
type NotificationTypeID int

// StateID represents ID value in `states` table.
type StateID int

// EventTarget matches the backend int code in the database where the notifications are sent (CCXDEV-8767)
type EventTarget int8

// ProducerMessage is a byte array (typically converted JSON) to be processed by one of the Producer implementations
type ProducerMessage []byte

const (
	// DBDriverSQLite3 shows that db driver is sqlite
	DBDriverSQLite3 DBDriver = iota
	// DBDriverPostgres shows that db driver is postgres
	DBDriverPostgres
	// DBDriverGeneral general sql(used for mock now)
	DBDriverGeneral
	// NotificationBackendTarget matches the notification backend int code in the database (CCXDEV-8767)
	NotificationBackendTarget EventTarget = 1
	// ServiceLogTarget matches the service log int code in the database (CCXDEV-8767)
	ServiceLogTarget EventTarget = 2
)

// ClusterEntry represents the entries retrieved from the DB
type ClusterEntry struct {
	OrgID         OrgID
	AccountNumber AccountNumber
	ClusterName   ClusterName
	KafkaOffset   KafkaOffset
	UpdatedAt     Timestamp
}

// NotificationType represents one record from `notification_types` table.
type NotificationType struct {
	ID        NotificationTypeID
	Value     string
	Frequency string
	Comment   string
}

// NotificationTypes contains all IDs for all possible notification types
type NotificationTypes struct {
	Instant NotificationTypeID
	Weekly  NotificationTypeID
}

// State represents one record from `states` table.
type State struct {
	ID      StateID
	Value   string
	Comment string
}

// States contains all IDs for all possible states
type States struct {
	SameState       StateID
	SentState       StateID
	LowerIssueState StateID
	ErrorState      StateID
}

// RulesMap contains a map of RuleContent objects accesible indexed by rule names
type RulesMap map[string]types.RuleContent

// MissingMandatoryFile is an error raised while parsing, when a mandatory file is missing
type MissingMandatoryFile struct {
	FileName string
}

func (err MissingMandatoryFile) Error() string {
	return fmt.Sprintf("Missing required file: %s", err.FileName)
}

// CliFlags represents structure holding all command line arguments/flags.
type CliFlags struct {
	InstantReports            bool
	WeeklyReports             bool
	ShowVersion               bool
	ShowAuthors               bool
	ShowConfiguration         bool
	PrintNewReportsForCleanup bool
	PerformNewReportsCleanup  bool
	PrintOldReportsForCleanup bool
	PerformOldReportsCleanup  bool
	CleanupOnStartup          bool
	Verbose                   bool
	MaxAge                    string
}

// Report represents report send in a message consumed from any broker
type Report struct {
	Reports []ReportItem `json:"reports"`
}

// RuleID represents type for rule id
type RuleID string

// RuleName represents type for rule name
type RuleName string

// ModuleName represents type for module name
type ModuleName string

// ErrorKey represents type for error key
type ErrorKey string

// ReportItem represents a single (hit) rule of the string encoded report
type ReportItem struct {
	Type     string          `json:"type"`
	Module   ModuleName      `json:"component"`
	ErrorKey ErrorKey        `json:"key"`
	Details  json.RawMessage `json:"details"`
}

// EventType represents the allowed event types in notification messages
type EventType int

// Event types as enum
const (
	InstantNotif EventType = iota
	WeeklyDigest
)

// Event types string representation
const (
	eventTypeInstant = "new-recommendation"
	eventTypeWeekly  = "weekly-digest"
)

// ToString function returns string representation of given event type
func (e EventType) ToString() string {
	return [...]string{eventTypeInstant, eventTypeWeekly}[e]
}

// EventMetadata represents the metadata of the sent payload.
// It is expected to be an empty struct as of today
type EventMetadata map[string]interface{}

// EventPayload is a JSON string containing all the data required
// by the app to compose the various messages (Email, webhook, ...).
type EventPayload map[string]string

// Event is a structure containing the payload and its metadata.
type Event struct {
	Metadata EventMetadata `json:"metadata"`
	Payload  string        `json:"payload"`
}

// Digest is a structure containing the counters for weekly digest
type Digest struct {
	ClustersAffected       int
	CriticalNotifications  int
	ImportantNotifications int
	Recommendations        int
	Incidents              int // We don't have this info, AFAIK
}

// NotificationContext represents the extra information
// that is common to all the events that are sent in
// this message as a JSON string (escaped)
type NotificationContext map[string]interface{}

// NotificationMessage represents content of messages
// sent to the notification platform topic in Kafka.
type NotificationMessage struct {
	Bundle      string  `json:"bundle"`
	Application string  `json:"application"`
	EventType   string  `json:"event_type"`
	Timestamp   string  `json:"timestamp"`
	AccountID   string  `json:"account_id"`
	OrgID       string  `json:"org_id"`
	Events      []Event `json:"events"`
	Context     string  `json:"context"`
}

// NotificationRecord structure represents one record stored in `reported` table.
type NotificationRecord struct {
	OrgID              OrgID
	AccountNumber      AccountNumber
	ClusterName        ClusterName
	UpdatedAt          Timestamp
	NotificationTypeID NotificationTypeID
	StateID            StateID
	Report             ClusterReport
	NotifiedAt         Timestamp
	ErrorLog           string
	EventTarget        EventTarget // this is the event_type_id (CCXDEV-8767)
}

// ClusterOrgKey is a slice with two items: an organization ID and a cluster UUID
type ClusterOrgKey struct {
	OrgID       OrgID
	ClusterName ClusterName
}

// NotifiedRecordsPerCluster maps a string representation of ClusterOrgKey to a NotificationRecord
type NotifiedRecordsPerCluster map[ClusterOrgKey]NotificationRecord

// NotifiedRecordsPerClusterByTarget let us split the notified records by their target (CCXDEV-8767)
type NotifiedRecordsPerClusterByTarget map[EventTarget]NotifiedRecordsPerCluster
