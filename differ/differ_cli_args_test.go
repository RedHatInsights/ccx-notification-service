/*
Copyright © 2022 Red Hat, Inc.

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

import (
	"os"
	"os/exec"
	"testing"

	"github.com/RedHatInsights/ccx-notification-service/types"
	"github.com/stretchr/testify/assert"
)

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

// Test the checkArgs function when --weekly-reports flag is set
func TestArgsParsingWeeklyReportsFlags(t *testing.T) {
	args := types.CliFlags{
		WeeklyReports: true,
	}
	checkArgs(&args)
	assert.Equal(t, notificationType, types.WeeklyDigest)
}

// Test the checkArgs function when --instant-reports flag is set
func TestArgsParsingInstantReportsFlags(t *testing.T) {
	args := types.CliFlags{
		InstantReports: true,
	}
	checkArgs(&args)
	assert.Equal(t, notificationType, types.InstantNotif)
}
