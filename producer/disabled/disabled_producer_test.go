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

package disabled

import (
	"encoding/json"
	"github.com/RedHatInsights/insights-operator-utils/tests/helpers"
	"testing"

	"github.com/RedHatInsights/ccx-notification-service/types"
	"github.com/stretchr/testify/assert"
)

func TestDisabledProducer(t *testing.T) {
	p := Producer{}

	msgBytes, err := json.Marshal(types.NotificationMessage{})
	helpers.FailOnError(t, err)

	_, _, err = p.ProduceMessage(msgBytes)
	assert.NoError(t, err, "error producing message")
	assert.NoError(t, p.Close(), "error closing producer")
}
