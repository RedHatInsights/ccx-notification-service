// Copyright 2021 Red Hat, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import "time"

// Timestamp represents any timestamp in a form gathered from database
// TODO: need to be improved
type Timestamp time.Time

// OrgID represents organization ID
type OrgID uint32

// ClusterName represents name of cluster in format c8590f31-e97e-4b85-b506-c45ce1911a12
type ClusterName string

// ClusterReport represents cluster report
type ClusterReport string

// DBDriver type for db driver enum
type DBDriver int

const (
	// DBDriverSQLite3 shows that db driver is sqlite
	DBDriverSQLite3 DBDriver = iota
	// DBDriverPostgres shows that db driver is postgres
	DBDriverPostgres
	// DBDriverGeneral general sql(used for mock now)
	DBDriverGeneral
)

type ClusterEntry struct {
	orgID       OrgID
	clusterName ClusterName
}

// RuleContent wraps all the content available for a rule into a single structure.
type RuleContent struct {
	Summary    string                         `json:"summary"`
	Reason     string                         `json:"reason"`
	Resolution string                         `json:"resolution"`
	MoreInfo   string                         `json:"more_info"`
	ErrorKeys  map[string]RuleErrorKeyContent `json:"error_keys"`
	HasReason  bool
}

// RuleErrorKeyContent wraps content of a single error key.
type RuleErrorKeyContent struct {
	Generic   string           `json:"generic"`
	Metadata  ErrorKeyMetadata `json:"metadata"`
	Reason    string           `json:"reason"`
	HasReason bool
}

// ErrorKeyMetadata is a Go representation of the `metadata.yaml`
// file inside of an error key content directory.
type ErrorKeyMetadata struct {
	Condition   string   `yaml:"condition" json:"condition"`
	Description string   `yaml:"description" json:"description"`
	Impact      string   `yaml:"impact" json:"impact"`
	Likelihood  int      `yaml:"likelihood" json:"likelihood"`
	PublishDate string   `yaml:"publish_date" json:"publish_date"`
	Status      string   `yaml:"status" json:"status"`
	Tags        []string `yaml:"tags" json:"tags"`
}

// MissingMandatoryFile is an error raised while parsing, when a mandatory file is missing
type MissingMandatoryFile struct {
	FileName string
}

// CliFlags represents structure holding all command line arguments/flags.
type CliFlags struct {
	instantReports    bool
	weeklyReports     bool
	showVersion       bool
	showAuthors       bool
	showConfiguration bool
}

// Report represents report send in a message consumed from any broker
type Report struct {
	Reports []ReportItem `json:"reports"`
}

// RuleID represents type for rule id
type RuleID string

// ErrorKey represents type for error key
type ErrorKey string

// ReportItem represents a single (hit) rule of the string encoded report
type ReportItem struct {
	Type     string   `json:"type"`
	Module   RuleID   `json:"component"`
	ErrorKey ErrorKey `json:"key"`
}

// GlobalRuleConfig represents the file that contains
// metadata globally applicable to any/all rule content.
type GlobalRuleConfig struct {
	Impact map[string]int `yaml:"impact" json:"impact"`
}
