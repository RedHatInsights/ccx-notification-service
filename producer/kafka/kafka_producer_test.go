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

package kafka

// Documentation in literate-programming-style is available at:
// https://redhatinsights.github.io/ccx-notification-service/packages/producer/kafka/kafka_producer_test.html

import (
	"encoding/json"
	"testing"
	"time"

	kafkautils "github.com/RedHatInsights/insights-operator-utils/kafka"

	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/types"
	"github.com/RedHatInsights/insights-operator-utils/tests/helpers"
	"github.com/Shopify/sarama/mocks"
	"github.com/stretchr/testify/assert"

	"github.com/rs/zerolog"
)

var (
	brokerCfg = kafkautils.BrokerConfiguration{
		Addresses: "localhost:9092",
		Topic:     "platform.notifications.ingress",
		Timeout:   time.Duration(30*10 ^ 9),
		Enabled:   true,
	}
)

func init() {
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
}

// Test Producer creation with a non-accessible Kafka broker
func TestNewProducerBadBroker(t *testing.T) {
	const expectedErrorMessage = "client has run out of available brokers to talk to"
	prod, err := New(&conf.ConfigStruct{
		Kafka: kafkautils.BrokerConfiguration{
			Topic:   "whatever",
			Timeout: 0,
			Enabled: true,
		}})
	assert.Nil(t, prod, "producer shouldn't be created with the provided config")
	assert.NotNil(t, err)

	prod, err = New(&conf.ConfigStruct{
		Kafka: brokerCfg,
	})
	assert.Nil(t, prod, "producer shouldn't be created if broker can't be reached")
	assert.ErrorContains(t, err, expectedErrorMessage)
}

// TestProducerClose makes sure it's possible to close the connection
func TestProducerClose(t *testing.T) {
	mockProducer := mocks.NewSyncProducer(t, nil)
	prod := Producer{
		Configuration: brokerCfg,
		Producer:      mockProducer,
	}

	err := prod.Close()
	assert.NoError(t, err, "failed to close Kafka producer")
}

//func TestProducerNew(t *testing.T) {
//	mockBroker := sarama.NewMockBroker(t, 0)
//	defer mockBroker.Close()
//
//	handlerMap := map[string]sarama.MockResponse{
//		"MetadataRequest": sarama.NewMockMetadataResponse(t).
//			SetBroker(mockBroker.Addr(), mockBroker.BrokerID()).
//			SetBroker("", mockBroker.BrokerID()).
//			SetLeader(brokerCfg.Topic, 0, mockBroker.BrokerID()),
//		"OffsetRequest": sarama.NewMockOffsetResponse(t).
//			SetOffset(brokerCfg.Topic, 0, -1, 0).
//			SetOffset(brokerCfg.Topic, 0, -2, 0),
//		"FetchRequest": sarama.NewMockFetchResponse(t, 1),
//		"FindCoordinatorRequest": sarama.NewMockFindCoordinatorResponse(t).
//			SetCoordinator(sarama.CoordinatorGroup, "", mockBroker),
//		"OffsetFetchRequest": sarama.NewMockOffsetFetchResponse(t).
//			SetOffset("", brokerCfg.Topic, 0, 0, "", sarama.ErrNoError),
//	}
//	mockBroker.SetHandlerByMap(handlerMap)
//
//	prod, err := New(&conf.ConfigStruct{
//		Kafka: kafkautils.BrokerConfiguration{
//			Addresses: mockBroker.Addr(),
//			Topic:     brokerCfg.Topic,
//			Timeout:   brokerCfg.Timeout,
//		}})
//	helpers.FailOnError(t, err)
//
//	helpers.FailOnError(t, prod.Close())
//}

func TestProducerSendEmptyNotificationMessage(t *testing.T) {
	mockProducer := mocks.NewSyncProducer(t, nil)
	mockProducer.ExpectSendMessageAndSucceed()

	kafkaProducer := Producer{
		Configuration: brokerCfg,
		Producer:      mockProducer,
	}

	msgBytes, err := json.Marshal(types.NotificationMessage{})
	helpers.FailOnError(t, err)

	_, _, err = kafkaProducer.ProduceMessage(msgBytes)
	assert.NoError(t, err, "Couldn't produce message with given broker configuration")
	helpers.FailOnError(t, kafkaProducer.Close())
}

func TestProducerSendNotificationMessageNoEvents(t *testing.T) {
	mockProducer := mocks.NewSyncProducer(t, nil)
	mockProducer.ExpectSendMessageAndSucceed()

	kafkaProducer := Producer{
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

	msgBytes, err := json.Marshal(msg)
	helpers.FailOnError(t, err)

	_, _, err = kafkaProducer.ProduceMessage(msgBytes)
	assert.NoError(t, err, "Couldn't produce message with given broker configuration")
	helpers.FailOnError(t, kafkaProducer.Close())
}

func TestProducerSendNotificationMessageSingleEvent(t *testing.T) {
	mockProducer := mocks.NewSyncProducer(t, nil)
	mockProducer.ExpectSendMessageAndSucceed()

	kafkaProducer := Producer{
		Configuration: brokerCfg,
		Producer:      mockProducer,
	}

	events := []types.Event{
		{
			Metadata: types.EventMetadata{},
			Payload:  types.EventPayload{"rule_id": "a unique ID", "what happened": "something baaad happened", "error_code": "3"},
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

	msgBytes, err := json.Marshal(msg)
	helpers.FailOnError(t, err)

	_, _, err = kafkaProducer.ProduceMessage(msgBytes)
	assert.NoError(t, err, "Couldn't produce message with given broker configuration")
	helpers.FailOnError(t, kafkaProducer.Close())
}

func TestProducerSendNotificationMessageMultipleEvents(t *testing.T) {
	mockProducer := mocks.NewSyncProducer(t, nil)
	mockProducer.ExpectSendMessageAndSucceed()

	kafkaProducer := Producer{
		Configuration: brokerCfg,
		Producer:      mockProducer,
	}

	events := []types.Event{
		{
			Metadata: types.EventMetadata{},
			Payload:  types.EventPayload{"rule_id": "a unique ID", "what happened": "something baaad happened", "error_code": "3"},
		},
		{
			Metadata: types.EventMetadata{},
			Payload:  types.EventPayload{"rule_id": "a unique ID", "what happened": "something baaad happened", "error_code": "3", "more_random_data": "why not..."},
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

	msgBytes, err := json.Marshal(msg)
	helpers.FailOnError(t, err)

	_, _, err = kafkaProducer.ProduceMessage(msgBytes)
	assert.NoError(t, err, "Couldn't produce message with given broker configuration")
	helpers.FailOnError(t, kafkaProducer.Close())
}

/*
// This sends to real kafka broker, just to make sure =)
func TestProducerSend(t *testing.T) {

	producer, err := sarama.NewSyncProducer([]string{brokerCfg.Address}, nil)
	kafkaProducer := KafkaProducer{
		Configuration: brokerCfg,
		Producer:  producer   ,
	}

	events := []types.Event{
		{
			Metadata: nil,
			Payload: types.EventPayload{"rule_id": "a unique ID", "what happened": "something baaad happened", "error_code":"3"},
		},
		{
			Metadata: nil,
			Payload: types.EventPayload{"rule_id": "a unique ID", "what happened": "something baaad happened", "error_code":"3"},
		},
	}

	msg := types.NotificationMessage{
		Bundle:      "openshift",
		Application: "advisor",
		EventType:   "critical",
		Timestamp:   time.Now().UTC().Format(time.RFC3339Nano),
		AccountID:   "000001",
		Events:      events,
		Context:     "no_context",
	}

	_, _, err = kafkaProducer.ProduceMessage(msg)
	assert.NoError(t, err, "Couldn't produce message with given broker configuration")
	helpers.FailOnError(t, kafkaProducer.Close())
}
*/
