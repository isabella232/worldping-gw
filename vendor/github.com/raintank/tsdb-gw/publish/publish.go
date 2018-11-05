package publish

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/raintank/tsdb-gw/metrics_client"
	log "github.com/sirupsen/logrus"
	schema "github.com/raintank/schema"
)

var (
	ingestedMetrics = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "gateway",
		Name:      "samples_ingested_total",
		Help:      "Number of samples ingested",
	})
)

type Publisher interface {
	Publish(metrics []*schema.MetricData) error
	Type() string
}

var (
	publisher Publisher

	// Persister allows pushing metrics to the Persistor Service
	Persistor *metrics_client.Client
)

func Init(p Publisher) {
	if p == nil {
		publisher = &nullPublisher{}
	} else {
		publisher = p
	}
	log.Infof("using %s publisher", publisher.Type())
}

func Publish(metrics []*schema.MetricData) error {
	if len(metrics) == 0 {
		return nil
	}
	ingestedMetrics.Add(float64(len(metrics)))
	return publisher.Publish(metrics)
}

// nullPublisher drops all metrics passed through the publish interface
type nullPublisher struct{}

func (*nullPublisher) Publish(metrics []*schema.MetricData) error {
	log.Debugf("publishing not enabled, dropping %d metrics", len(metrics))
	return nil
}

func (*nullPublisher) Type() string {
	return "nullPublisher"
}

func Persist(metrics []*schema.MetricData) error {
	return publisher.Publish(metrics)
}
