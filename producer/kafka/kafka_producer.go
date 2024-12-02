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
	"crypto/sha512"
	"strings"

	"github.com/RedHatInsights/ccx-notification-service/types"

	"github.com/RedHatInsights/ccx-notification-service/conf"
	tlsutils "github.com/RedHatInsights/insights-operator-utils/tls"
	"github.com/Shopify/sarama"
	"github.com/rs/zerolog/log"
)

// Producer is an implementation of Producer interface
type Producer struct {
	Configuration conf.KafkaConfiguration
	Producer      sarama.SyncProducer
}

// New constructs new implementation of Producer interface
func New(config *conf.ConfigStruct) (*Producer, error) {
	kafkaConfig := conf.GetKafkaBrokerConfiguration(config)

	saramaConfig, err := saramaConfigFromBrokerConfig(&kafkaConfig)
	if err != nil {
		log.Error().Err(err).Msg("Unable to create a valid Kafka configuration")
	}

	producer, err := sarama.NewSyncProducer(strings.Split(kafkaConfig.Addresses, ","), saramaConfig)
	if err != nil {
		log.Error().Str("Kafka address", kafkaConfig.Addresses).Err(err).Msg("unable to start a Kafka producer")
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

func saramaConfigFromBrokerConfig(cfg *conf.KafkaConfiguration) (*sarama.Config, error) {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Version = sarama.V3_6_0_0

	if strings.Contains(cfg.SecurityProtocol, "SSL") {
		saramaConfig.Net.TLS.Enable = true
	}
	if strings.EqualFold(cfg.SecurityProtocol, "SSL") && cfg.CertPath != "" {
		tlsConfig, err := tlsutils.NewTLSConfig(cfg.CertPath)
		if err != nil {
			log.Error().Msgf("Unable to load TLS config for %s cert", cfg.CertPath)
			return nil, err
		}
		saramaConfig.Net.TLS.Config = tlsConfig
	} else if strings.HasPrefix(cfg.SecurityProtocol, "SASL_") {
		log.Info().Msg("Configuring SASL authentication")
		saramaConfig.Net.SASL.Enable = true
		saramaConfig.Net.SASL.User = cfg.SaslUsername
		saramaConfig.Net.SASL.Password = cfg.SaslPassword
		saramaConfig.Net.SASL.Mechanism = sarama.SASLMechanism(cfg.SaslMechanism)

		if strings.EqualFold(cfg.SaslMechanism, sarama.SASLTypeSCRAMSHA512) {
			log.Info().Msg("Configuring SCRAM-SHA512")
			saramaConfig.Net.SASL.Handshake = true
			saramaConfig.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
				return &SCRAMClient{HashGeneratorFcn: sha512.New}
			}
		}
	}

	saramaConfig.Producer.Return.Successes = true
	return saramaConfig, nil
}
