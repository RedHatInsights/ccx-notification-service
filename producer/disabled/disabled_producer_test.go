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

package disabled

// Documentation in literate-programming-style is available at:
// https://redhatinsights.github.io/ccx-notification-service/packages/disabled/disabled_producer_test.html

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDisabledProducer(t *testing.T) {
	p := Producer{}
	_, _, err := p.ProduceMessage([]byte{})
	assert.NoError(t, err, "error producing message")
	assert.NoError(t, p.Close(), "error closing producer")
}
