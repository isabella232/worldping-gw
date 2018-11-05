package publish

import (
	"encoding/binary"
	"flag"
	"time"

	"github.com/Shopify/sarama"
	"github.com/grafana/metrictank/stats"
	"github.com/grafana/worldping-gw/events/msg"
	log "github.com/sirupsen/logrus"
)

var (
	config          *sarama.Config
	producer        sarama.SyncProducer
	kafkaVersionStr string
	topic           string
	codec           string
	enabled         bool
	brokers         []string
	eventsPublished = stats.NewCounter32("events.published")
	messagesSize    = stats.NewMeter32("events.message_size", false)
	publishDuration = stats.NewLatencyHistogram15s32("events.publish")
	sendErrProducer = stats.NewCounter32("events.send_error.producer")
	sendErrOther    = stats.NewCounter32("events.send_error.other")
)

func init() {
	flag.StringVar(&topic, "events-topic", "events", "Kafka topic for events")
	flag.BoolVar(&enabled, "events-publish", false, "enable event publishing")
	flag.StringVar(&codec, "events-compression", "none", "compression: none|gzip|snappy")
	flag.StringVar(&kafkaVersionStr, "events-kafka-version", "0.10.0.0", "Kafka version in semver format. All brokers must be this version or newer.")
}

func getCompression(codec string) sarama.CompressionCodec {
	switch codec {
	case "none":
		return sarama.CompressionNone
	case "gzip":
		return sarama.CompressionGZIP
	case "snappy":
		return sarama.CompressionSnappy
	default:
		log.Fatalf("unknown compression codec %q", codec)
		return 0 // make go compiler happy, needs a return *roll eyes*
	}
}

func Init(broker string) {
	if !enabled {
		return
	}
	kafkaVersion, err := sarama.ParseKafkaVersion(kafkaVersionStr)
	if err != nil {
		log.Fatalf("invalid kafka-version. %s", err)
	}
	// We are looking for strong consistency semantics.
	// Because we don't change the flush settings, sarama will try to produce messages
	// as fast as possible to keep latency low.
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll // Wait for all in-sync replicas to ack the message
	config.Producer.Retry.Max = 10                   // Retry up to 10 times to produce the message
	config.Producer.Compression = getCompression(codec)
	config.Producer.Return.Successes = true
	config.Version = kafkaVersion
	err = config.Validate()
	if err != nil {
		log.Fatalf("failed to validate kafka config. %s", err)
	}

	brokers = []string{broker}

	producer, err = sarama.NewSyncProducer(brokers, config)
	if err != nil {
		log.Fatalf("failed to initialize kafka producer. %s", err)
	}
}

func Publish(events []*msg.ProbeEvent) error {
	if producer == nil {
		log.Debug("droping events as publishing is disabled")
		return nil
	}
	msgSize := 0
	payload := make([]*sarama.ProducerMessage, len(events))
	for i, event := range events {
		id := time.Now().UnixNano()
		data, err := msg.CreateProbeEventMsg(event, id, msg.FormatProbeEventMsgp)
		if err != nil {
			log.Fatalf("Fatal error creating event message: %s", err)
		}
		// partition by organisation: metrics for the same org should go to the same
		// partition/MetricTank (optimize for locality~performance)
		// the extra 4B (now initialized with zeroes) is to later enable a smooth transition
		// to a more fine-grained partitioning scheme where
		// large organisations can go to several partitions instead of just one.
		key := make([]byte, 8)
		binary.LittleEndian.PutUint32(key, uint32(event.OrgId))
		payload[i] = &sarama.ProducerMessage{
			Key:   sarama.ByteEncoder(key),
			Topic: topic,
			Value: sarama.ByteEncoder(data),
		}
		msgSize += len(data)
	}
	messagesSize.Value(msgSize)

	pre := time.Now()
	err := producer.SendMessages(payload)
	if err != nil {
		if errors, ok := err.(sarama.ProducerErrors); ok {
			sendErrProducer.Add(len(errors))
			for i := 0; i < 10 && i < len(errors); i++ {
				log.Errorf("SendMessages ProducerError %d/%d: %s", i, len(errors), errors[i].Error())
			}
		} else {
			sendErrOther.Inc()
			log.Errorf("SendMessages error: %s", err.Error())
		}
		return err
	}
	publishDuration.Value(time.Since(pre))
	eventsPublished.Add(len(payload))

	log.Debugf("published %d events", len(payload))
	return nil
}
