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

// Package producer contains functions that can be used to produce (that is
// send) messages to properly configured Kafka broker.
package producer

// Generated documentation is available at:
// https://pkg.go.dev/github.com/RedHatInsights/ccx-notification-service/producer
//
// Documentation in literate-programming-style is available at:
// https://redhatinsights.github.io/ccx-notification-service/packages/producer/kafka_producer.html

import (
	"encoding/json"
	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/types"

	"github.com/Shopify/sarama"
	"github.com/rs/zerolog/log"
)

// KafkaProducer is an implementation of Producer interface
type KafkaProducer struct {
	Configuration conf.KafkaConfiguration
	Producer      sarama.SyncProducer
}

// New constructs new implementation of Producer interface
func New(brokerCfg conf.KafkaConfiguration) (*KafkaProducer, error) {
	//TODO: Think about a tolerant config for the producer
	producer, err := sarama.NewSyncProducer([]string{brokerCfg.Address}, nil)
	if err != nil {
		log.Error().Err(err).Msg("unable to create a new Kafka producer")
		return nil, err
	}

	return &KafkaProducer{
		Configuration: brokerCfg,
		Producer:      producer,
	}, nil
}

// ProduceMessage produces message to selected topic. That function returns
// partition ID and offset of new message or an error value in case of any
// problem on broker side.
func (producer *KafkaProducer) ProduceMessage(msg types.NotificationMessage) (partitionID int32, offset int64, err error) {
	// no-op when producer is disabled
	// (this logic allows us to enable/disable producer on the fly
	if !producer.Configuration.Enabled {
		return
	}

	jsonBytes, err := json.Marshal(msg)

	if err != nil {
		log.Error().Err(err).Msg("Couldn't turn notification message into valid JSON")
		return -1, -1, err
	}

	producerMsg := &sarama.ProducerMessage{
		Topic: producer.Configuration.Topic,
		Value: sarama.ByteEncoder(jsonBytes),
	}

	partitionID, offset, err = producer.Producer.SendMessage(producerMsg)
	if err != nil {
		log.Error().Err(err).Msg("failed to produce message to Kafka")
	} else {
		log.Info().Msgf("message sent to partition %d at offset %d\n", partitionID, offset)
	}
	return
}

// Close allow the Sarama producer to be gracefully closed
func (producer *KafkaProducer) Close() error {
	log.Info().Msg("Shutting down kafka producer")
	if err := producer.Producer.Close(); err != nil {
		log.Error().Err(err).Msg("unable to close Kafka producer")
		return err
	}

	return nil
}
