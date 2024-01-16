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

// Package kafka contains an implementation of Producer interface that can be
// used to produce (that is send) messages to properly configured Kafka broker.
package kafka

// Generated documentation is available at:
// https://pkg.go.dev/github.com/RedHatInsights/ccx-notification-service/producer
//
// Documentation in literate-programming-style is available at:
// https://redhatinsights.github.io/ccx-notification-service/packages/producer/kafka/kafka_producer.html

import (
	"github.com/RedHatInsights/ccx-notification-service/types"
	"github.com/RedHatInsights/insights-operator-utils/kafka"
	"strings"

	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/Shopify/sarama"
	"github.com/rs/zerolog/log"
)

// Producer is an implementation of Producer interface
type Producer struct {
	Configuration kafka.BrokerConfiguration
	Producer      sarama.SyncProducer
}

// New constructs new implementation of Producer interface
func New(config *conf.ConfigStruct) (*Producer, error) {
	kafkaConfig := conf.GetKafkaBrokerConfiguration(config)

	saramaConfig, err := kafka.SaramaConfigFromBrokerConfig(&kafkaConfig)
	if err != nil {
		log.Error().Err(err).Msg("Unable to create a valid Kafka configuration")
	}
	saramaConfig.Producer.Return.Successes = true

	producer, err := sarama.NewSyncProducer(strings.Split(kafkaConfig.Addresses, ","), saramaConfig)
	if err != nil {
		log.Error().Str("Kafka address", kafkaConfig.Addresses).Err(err).Msg("Unable to start Kafka producer")
		return nil, err
	}

	return &Producer{
		Configuration: kafkaConfig,
		Producer:      producer,
	}, nil
}

// ProduceMessage produces message to selected topic. That function returns
// partition ID and offset of new message or an error value in case of any
// problem on broker side.
func (producer *Producer) ProduceMessage(msg types.ProducerMessage) (partitionID int32, offset int64, err error) {
	// no-op when producer is disabled
	// (this logic allows us to enable/disable producer on the fly
	if !producer.Configuration.Enabled {
		return
	}

	producerMsg := &sarama.ProducerMessage{
		Topic: producer.Configuration.Topic,
		Value: sarama.ByteEncoder(msg),
	}

	partitionID, offset, err = producer.Producer.SendMessage(producerMsg)
	if err != nil {
		log.Error().Err(err).Msg("failed to produce message to Kafka")
	} else {
		log.Info().Int("partition", int(partitionID)).Int("offset", int(offset)).Msg("message sent")
	}
	return
}

// Close allow the Sarama producer to be gracefully closed
func (producer *Producer) Close() error {
	log.Info().Msg("Shutting down kafka producer")
	if err := producer.Producer.Close(); err != nil {
		log.Error().Err(err).Msg("unable to close Kafka producer")
		return err
	}

	return nil
}
