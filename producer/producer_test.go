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

package producer

import (
	"errors"
	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/types"
	"github.com/RedHatInsights/insights-operator-utils/tests/helpers"
	"github.com/Shopify/sarama"
	"github.com/Shopify/sarama/mocks"
	"github.com/stretchr/testify/assert"
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

// Test Producer creation with a non accessible Kafka broker
func TestNewProducerBadBroker(t *testing.T) {
	const expectedErr = "kafka: client has run out of available brokers to talk to (Is your cluster reachable?)"

	_, err := New(brokerCfg)
	assert.EqualError(t, err, expectedErr)
}

// TestProducerClose makes sure it's possible to close the connection
func TestProducerClose(t *testing.T) {
	mockProducer := mocks.NewSyncProducer(t, nil)
	prod := KafkaProducer{
		Configuration: brokerCfg,
		Producer:      mockProducer,
	}

	err := prod.Close()
	assert.NoError(t, err, "failed to close Kafka producer")
}

func TestProducerNew(t *testing.T) {
	mockBroker := sarama.NewMockBroker(t, 0)
	defer mockBroker.Close()

	handlerMap := map[string]sarama.MockResponse{
		"MetadataRequest": sarama.NewMockMetadataResponse(t).
			SetBroker(mockBroker.Addr(), mockBroker.BrokerID()).
			SetLeader(brokerCfg.Topic, 0, mockBroker.BrokerID()),
		"OffsetRequest": sarama.NewMockOffsetResponse(t).
			SetOffset(brokerCfg.Topic, 0, -1, 0).
			SetOffset(brokerCfg.Topic, 0, -2, 0),
		"FetchRequest": sarama.NewMockFetchResponse(t, 1),
		"FindCoordinatorRequest": sarama.NewMockFindCoordinatorResponse(t).
			SetCoordinator(sarama.CoordinatorGroup, "", mockBroker),
		"OffsetFetchRequest": sarama.NewMockOffsetFetchResponse(t).
			SetOffset("", brokerCfg.Topic, 0, 0, "", sarama.ErrNoError),
	}
	mockBroker.SetHandlerByMap(handlerMap)

	prod, err := New(
		conf.KafkaConfiguration{
			Address: mockBroker.Addr(),
			Topic:   brokerCfg.Topic,
			Timeout: brokerCfg.Timeout,
		})
	helpers.FailOnError(t, err)

	helpers.FailOnError(t, prod.Close())
}

func TestProducerSendEmptyNotificationMessage(t *testing.T) {
	mockProducer := mocks.NewSyncProducer(t, nil)
	mockProducer.ExpectSendMessageAndSucceed()

	kafkaProducer := KafkaProducer{
		Configuration: brokerCfg,
		Producer:      mockProducer,
	}

	_, _, err := kafkaProducer.ProduceMessage(types.NotificationMessage{})
	assert.NoError(t, err, "Couldn't produce message with given broker configuration")
	helpers.FailOnError(t, kafkaProducer.Close())
}

func TestProducerSendNotificationMessageNoEvents(t *testing.T) {
	mockProducer := mocks.NewSyncProducer(t, nil)
	mockProducer.ExpectSendMessageAndSucceed()

	kafkaProducer := KafkaProducer{
		Configuration: brokerCfg,
		Producer:      mockProducer,
	}

	msg := types.NotificationMessage{
		Bundle:      "openshift",
		Application: "advisor",
		EventType:   "critical",
		Timestamp:   time.Now().UTC().Format(time.RFC3339Nano),
		AccountID:   "000000",
		Events:      nil,
		Context:     nil,
	}

	_, _, err := kafkaProducer.ProduceMessage(msg)
	assert.NoError(t, err, "Couldn't produce message with given broker configuration")
	helpers.FailOnError(t, kafkaProducer.Close())
}

func TestProducerSendNotificationMessageSingleEvent(t *testing.T) {
	mockProducer := mocks.NewSyncProducer(t, nil)
	mockProducer.ExpectSendMessageAndSucceed()

	kafkaProducer := KafkaProducer{
		Configuration: brokerCfg,
		Producer:      mockProducer,
	}

	events := []types.Event{
		{
			Metadata: nil,
			Payload: map[string]interface{}{
				"rule_id":       "a unique ID",
				"what happened": "something baaad happened",
				"error_code":    3,
			},
		},
	}

	msg := types.NotificationMessage{
		Bundle:      "openshift",
		Application: "advisor",
		EventType:   "critical",
		Timestamp:   time.Now().UTC().Format(time.RFC3339Nano),
		AccountID:   "000001",
		Events:      events,
		Context:     nil,
	}

	_, _, err := kafkaProducer.ProduceMessage(msg)
	assert.NoError(t, err, "Couldn't produce message with given broker configuration")
	helpers.FailOnError(t, kafkaProducer.Close())
}

func TestProducerSendNotificationMessageMultipleEvents(t *testing.T) {
	mockProducer := mocks.NewSyncProducer(t, nil)
	mockProducer.ExpectSendMessageAndSucceed()

	kafkaProducer := KafkaProducer{
		Configuration: brokerCfg,
		Producer:      mockProducer,
	}

	events := []types.Event{
		{
			Metadata: nil,
			Payload: map[string]interface{}{
				"rule_id":       "a unique ID",
				"what happened": "something baaad happened",
				"error_code":    3,
			},
		},
		{
			Metadata: nil,
			Payload: map[string]interface{}{
				"rule_id":          "another unique ID",
				"what happened":    "something less terrible happened",
				"error_code":       4,
				"more_random_data": "why not...",
			},
		},
	}

	msg := types.NotificationMessage{
		Bundle:      "openshift",
		Application: "advisor",
		EventType:   "critical",
		Timestamp:   time.Now().UTC().Format(time.RFC3339Nano),
		AccountID:   "000001",
		Events:      events,
		Context:     nil,
	}

	_, _, err := kafkaProducer.ProduceMessage(msg)
	assert.NoError(t, err, "Couldn't produce message with given broker configuration")
	helpers.FailOnError(t, kafkaProducer.Close())
}

func TestProducerSendNotificationMessageEventContentNotValidJson(t *testing.T) {
	const jsonParseErrorMessage = "json: unsupported type: chan int" //TODO: Using a mocked function for json.Marshall that throws an error is cooler than this =)

	mockProducer := mocks.NewSyncProducer(t, nil)
	mockProducer.ExpectSendMessageAndFail(errors.New(jsonParseErrorMessage))

	kafkaProducer := KafkaProducer{
		Configuration: brokerCfg,
		Producer:      mockProducer,
	}

	events := []types.Event{
		{
			Metadata: nil,
			Payload: map[string]interface{}{
				"rule_id":       "a unique ID",
				"what happened": "something baaad happened",
				"error_code":    3,
			},
		},
		{
			Metadata: map[string]interface{}{
				"foo": make(chan int), //A value that cannot be represented in JSON
			},
			Payload: map[string]interface{}{
				"rule_id":       "a unique ID",
				"what happened": "something baaad happened",
				"error_code":    3,
			},
		},
	}

	msg := types.NotificationMessage{
		Bundle:      "openshift",
		Application: "advisor",
		EventType:   "critical",
		Timestamp:   time.Now().UTC().Format(time.RFC3339Nano),
		AccountID:   "000001",
		Events:      events,
		Context:     nil,
	}

	_, _, err := kafkaProducer.ProduceMessage(msg)

	assert.EqualError(t, err, jsonParseErrorMessage)
}

/* This sends to real kafka broker, just to make sure =)
func TestProducerSend(t *testing.T) {

	producer, err := sarama.NewSyncProducer([]string{brokerCfg.Address}, nil)
	kafkaProducer := KafkaProducer{
		Configuration: brokerCfg,
		Producer:  producer   ,
	}

	events := []types.Event{
		{
			Metadata: nil,
			Payload: map[string]interface{}{
				"rule_id":       "a unique ID",
				"what happened": "something baaad happened",
				"error_code":    3,
			},
		},
		{
			Metadata: nil,
			Payload: map[string]interface{}{
				"rule_id":       "another unique ID",
				"what happened": "something less terrible happened",
				"error_code":    4,
				"more_random_data": "why not...",
			},
		},
	}

	msg := types.NotificationMessage{
		Bundle:      "openshift",
		Application: "advisor",
		EventType:   "critical",
		Timestamp:   time.Now().UTC().Format(time.RFC3339Nano),
		AccountID:   "000001",
		Events:      events,
		Context:     nil,
	}

	_, _, err = kafkaProducer.ProduceMessage(msg)
	assert.NoError(t, err, "Couldn't produce message with given broker configuration")
	helpers.FailOnError(t, kafkaProducer.Close())
}
*/
