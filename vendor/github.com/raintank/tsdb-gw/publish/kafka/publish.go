package kafka

import (
	"errors"
	"flag"
	"time"

	"github.com/grafana/metrictank/conf"

	"github.com/Shopify/sarama"
	p "github.com/grafana/metrictank/cluster/partitioner"
	"github.com/grafana/metrictank/stats"
	"github.com/raintank/schema"
	"github.com/raintank/schema/msg"
	"github.com/raintank/tsdb-gw/publish/kafka/keycache"
	"github.com/raintank/tsdb-gw/util"
	log "github.com/sirupsen/logrus"
)

var (
	producer        sarama.SyncProducer
	brokers         []string
	kafkaVersionStr string
	keyCache        *keycache.KeyCache

	partitioner *p.Kafka
	schemasConf string

	publishedMD     = stats.NewCounterRate32("output.kafka.published.metricdata")
	publishedMP     = stats.NewCounterRate32("output.kafka.published.metricpoint")
	publishedMPNO   = stats.NewCounterRate32("output.kafka.published.metricpoint_no_org")
	messagesSize    = stats.NewMeter32("metrics.message_size", false)
	publishDuration = stats.NewLatencyHistogram15s32("metrics.publish")
	sendErrProducer = stats.NewCounterRate32("metrics.send_error.producer")
	sendErrOther    = stats.NewCounterRate32("metrics.send_error.other")

	topic           string
	codec           string
	enabled         bool
	partitionScheme string
	maxMessages     int
	v2              bool
	v2Org           bool
	v2ClearInterval time.Duration
	flushFreq       time.Duration

	bufferPool   = util.NewBufferPool()
	bufferPool33 = util.NewBufferPool33()
)

type mtPublisher struct {
	schemas      *conf.Schemas
	autoInterval bool
}

type Partitioner interface {
	partition(schema.PartitionedMetric) (int32, []byte, error)
}

func init() {
	flag.StringVar(&topic, "metrics-topic", "mdm", "topic for metrics")
	flag.StringVar(&codec, "metrics-kafka-comp", "snappy", "compression: none|gzip|snappy")
	flag.BoolVar(&enabled, "metrics-publish", false, "enable metric publishing")
	flag.StringVar(&partitionScheme, "metrics-partition-scheme", "bySeries", "method used for paritioning metrics. (byOrg|bySeries)")
	flag.DurationVar(&flushFreq, "metrics-flush-freq", time.Millisecond*50, "The best-effort frequency of flushes to kafka")
	flag.IntVar(&maxMessages, "metrics-max-messages", 5000, "The maximum number of messages the producer will send in a single request")
	flag.StringVar(&schemasConf, "schemas-file", "/etc/gw/storage-schemas.conf", "path to carbon storage-schemas.conf file")
	flag.BoolVar(&v2, "v2", true, "enable optimized MetricPoint payload")
	flag.BoolVar(&v2Org, "v2-org", true, "encode org-id in messages")
	flag.DurationVar(&v2ClearInterval, "v2-clear-interval", time.Hour, "interval after which we always resend a full MetricData")
	flag.StringVar(&kafkaVersionStr, "kafka-version", "0.10.0.0", "Kafka version in semver format. All brokers must be this version or newer.")
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

func New(broker string, autoInterval bool) *mtPublisher {
	if !enabled {
		return nil
	}

	kafkaVersion, err := sarama.ParseKafkaVersion(kafkaVersionStr)
	if err != nil {
		log.Fatalf("invalid kafka-version. %s", err)
	}

	mp := mtPublisher{
		autoInterval: autoInterval,
	}

	if autoInterval {
		schemas, err := getSchemas(schemasConf)
		if err != nil {
			log.Fatalf("failed to load schemas config. %s", err)
		}
		mp.schemas = schemas
	}

	partitioner, err = p.NewKafka(partitionScheme)
	if err != nil {
		log.Fatalf("failed to initialize partitioner: %s", err)
	}

	// We are looking for strong consistency semantics.
	// Because we don't change the flush settings, sarama will try to produce messages
	// as fast as possible to keep latency low.
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll // Wait for all in-sync replicas to ack the message
	config.Producer.Retry.Max = 10                   // Retry up to 10 times to produce the message
	config.Producer.Compression = getCompression(codec)
	config.Producer.Return.Successes = true
	config.Producer.Flush.Frequency = flushFreq
	config.Producer.Flush.MaxMessages = maxMessages
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

	if v2 {
		keyCache = keycache.NewKeyCache(v2ClearInterval)
	}

	return &mp
}

func (m *mtPublisher) Publish(metrics []*schema.MetricData) error {
	if producer == nil {
		log.Debugf("dropping %d metrics as publishing is disabled", len(metrics))
		return nil
	}

	if len(metrics) == 0 {
		return nil
	}

	var err error

	payload := make([]*sarama.ProducerMessage, len(metrics))
	pre := time.Now()
	pubMD := 0
	pubMP := 0
	pubMPNO := 0

	for i, metric := range metrics {
		if metric.Interval == 0 {
			if m.autoInterval {
				_, s := m.schemas.Match(metric.Name, 0)
				metric.Interval = s.Retentions[0].SecondsPerPoint
				metric.SetId()
			} else {
				log.Error("interval is 0 but can't deduce interval automatically. this should never happen")
				return errors.New("need to deduce interval but cannot")
			}
		}

		var data []byte
		if v2 {
			var mkey schema.MKey
			mkey, err = schema.MKeyFromString(metric.Id)
			if err != nil {
				return err
			}
			ok := keyCache.Touch(mkey)
			// we've seen this key recently. we can use the optimized format
			if ok {
				data = bufferPool33.Get()
				mp := schema.MetricPoint{
					MKey:  mkey,
					Value: metric.Value,
					Time:  uint32(metric.Time),
				}
				if v2Org {
					data = data[:33]                      // this range will contain valid data
					data[0] = byte(msg.FormatMetricPoint) // store version in first byte
					_, err = mp.Marshal32(data[:1])       // Marshal will fill up space between length and cap, i.e. bytes 2-33
					pubMP++
				} else {
					data = data[:29]                                // this range will contain valid data
					data[0] = byte(msg.FormatMetricPointWithoutOrg) // store version in first byte
					_, err = mp.MarshalWithoutOrg28(data[:1])       // Marshal will fill up space between length and cap, i.e. bytes 2-29
					pubMPNO++
				}
			} else {
				data = bufferPool.Get()
				data, err = metric.MarshalMsg(data)
				if err != nil {
					return err
				}
				pubMD++
			}
		} else {
			data = bufferPool.Get()
			data, err = metric.MarshalMsg(data)
			if err != nil {
				return err
			}
			pubMD++
		}

		key, err := partitioner.GetPartitionKey(metric, nil)
		if err != nil {
			return err
		}
		payload[i] = &sarama.ProducerMessage{
			Key:   sarama.ByteEncoder(key),
			Topic: topic,
			Value: sarama.ByteEncoder(data),
		}

		messagesSize.Value(len(data))
	}

	defer func() {
		var buf []byte
		for _, msg := range payload {
			buf, _ = msg.Value.Encode()
			if cap(buf) == 33 {
				bufferPool33.Put(buf)
			} else {
				bufferPool.Put(buf)
			}
		}
	}()

	err = producer.SendMessages(payload)
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
	publishedMD.Add(pubMD)
	publishedMP.Add(pubMP)
	publishedMPNO.Add(pubMPNO)
	log.Debugf("published %d metrics", pubMD+pubMP)
	return nil
}

func (*mtPublisher) Type() string {
	return "Metrictank"
}
