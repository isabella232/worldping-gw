package metric_publish

import (
	"flag"
	"time"

	"github.com/Shopify/sarama"
	"github.com/golang/glog"
	p "github.com/grafana/metrictank/cluster/partitioner"
	"github.com/grafana/metrictank/stats"
	"github.com/grafana/worldping-gw/usage"
	"github.com/grafana/worldping-gw/util"
	"gopkg.in/raintank/schema.v1"
)

var (
	config   *sarama.Config
	producer sarama.SyncProducer
	brokers  []string

	metricsPublished = stats.NewCounter32("metrics.published")
	messagesSize     = stats.NewMeter32("metrics.message_size", false)
	publishDuration  = stats.NewLatencyHistogram15s32("metrics.publish")
	sendErrProducer  = stats.NewCounter32("metrics.send_error.producer")
	sendErrOther     = stats.NewCounter32("metrics.send_error.other")

	partitioner     *p.Kafka
	topic           string
	codec           string
	enabled         bool
	partitionScheme string
	flushFreq       time.Duration
	maxMessages     int

	bufferPool = util.NewBufferPool()
)

func init() {
	flag.StringVar(&topic, "metrics-topic", "mdm", "topic for metrics")
	flag.StringVar(&codec, "metrics-kafka-comp", "snappy", "compression: none|gzip|snappy")
	flag.BoolVar(&enabled, "metrics-publish", false, "enable metric publishing")
	flag.StringVar(&partitionScheme, "metrics-partition-scheme", "bySeries", "method used for paritioning metrics. (byOrg|bySeries)")
	flag.DurationVar(&flushFreq, "metrics-flush-freq", time.Millisecond*50, "The best-effort frequency of flushes to kafka")
	flag.IntVar(&maxMessages, "metrics-max-messages", 5000, "The maximum number of messages the producer will send in a single request")
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
		glog.Fatalf("unknown compression codec %q", codec)
		return 0 // make go compiler happy, needs a return *roll eyes*
	}
}

func Init(broker string) {
	if !enabled {
		return
	}
	var err error
	partitioner, err = p.NewKafka(partitionScheme)
	if err != nil {
		glog.Fatalf("failed to initialize partitioner. %s", err)
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
	err = config.Validate()
	if err != nil {
		glog.Fatalf("failed to validate kafka config. %s", err)
	}

	brokers = []string{broker}

	producer, err = sarama.NewSyncProducer(brokers, config)
	if err != nil {
		glog.Fatalf("failed to initialize kafka producer. %s", err)
	}
}

func Publish(metrics []*schema.MetricData) error {
	if producer == nil {
		glog.V(5).Infof("droping %d metrics as publishing is disabled", len(metrics))
		return nil
	}
	if len(metrics) == 0 {
		return nil
	}
	var err error

	payload := make([]*sarama.ProducerMessage, len(metrics))
	pre := time.Now()

	for i, metric := range metrics {
		data := bufferPool.Get()
		data, err = metric.MarshalMsg(data)
		if err != nil {
			return err
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
	// return buffers to the bufferPool
	defer func() {
		var buf []byte
		for _, msg := range payload {
			buf, _ = msg.Value.Encode()
			bufferPool.Put(buf)
		}
	}()
	err = producer.SendMessages(payload)
	if err != nil {
		if errors, ok := err.(sarama.ProducerErrors); ok {
			sendErrProducer.Add(len(errors))
			for i := 0; i < 10 && i < len(errors); i++ {
				glog.Errorf("SendMessages ProducerError %d/%d: %s", i, len(errors), errors[i].Error())
			}
		} else {
			sendErrOther.Inc()
			glog.Errorf("SendMessages error: %s", err.Error())
		}
		return err
	}
	publishDuration.Value(time.Since(pre))
	metricsPublished.Add(len(metrics))
	glog.V(5).Infof("published %d metrics", len(metrics))
	for _, metric := range metrics {
		usage.LogDataPoint(metric.Id)
	}
	return nil
}
