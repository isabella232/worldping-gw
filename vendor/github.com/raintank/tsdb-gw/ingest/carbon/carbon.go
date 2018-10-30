package carbon

import (
	"bytes"
	"flag"
	"sync"
	"time"

	"github.com/grafana/metrictank/conf"
	"github.com/grafana/metrictank/stats"
	"github.com/graphite-ng/carbon-relay-ng/input"
	m20 "github.com/metrics20/go-metrics20/carbon20"
	"github.com/raintank/schema"
	"github.com/raintank/tsdb-gw/auth"
	"github.com/raintank/tsdb-gw/publish"
	"github.com/raintank/tsdb-gw/util"
	log "github.com/sirupsen/logrus"
)

var (
	metricsReceived          = stats.NewCounterRate32("metrics.carbon.received")
	metricsValid             = stats.NewCounterRate32("metrics.carbon.valid")
	metricsRejected          = stats.NewCounterRate32("metrics.carbon.rejected")
	metricsFailed            = stats.NewCounterRate32("metrics.carbon.failed")
	metricsDroppedBufferFull = stats.NewCounterRate32("metrics.carbon.dropped_buffer_full")
	metricsDroppedAuthFail   = stats.NewCounterRate32("metrics.carbon.dropped_auth_fail")

	carbonConnections = stats.NewGauge32("carbon.connections")

	Enabled           bool
	addr              string
	concurrency       int
	bufferSize        int
	flushInterval     time.Duration
	nonBlockingBuffer bool
	authPlugin        string

	metricPool = util.NewMetricDataPool()
)

func init() {
	flag.BoolVar(&Enabled, "carbon-enabled", false, "enable carbon input")
	flag.StringVar(&addr, "carbon-addr", "0.0.0.0:2003", "listen address for carbon input")
	flag.StringVar(&authPlugin, "carbon-auth-plugin", "file", "auth plugin to use. (grafana|file)")
	flag.DurationVar(&flushInterval, "carbon-flush-interval", time.Second, "maximum time between flushs to kafka")
	flag.IntVar(&concurrency, "carbon-concurrency", 1, "number of goroutines for handling metrics")
	flag.IntVar(&bufferSize, "carbon-buffer-size", 100000, "number of metrics to hold in an input buffer. Once this buffer fills metrics will be dropped")
	flag.BoolVar(&nonBlockingBuffer, "carbon-non-blocking-buffer", false, "dont block trying to write to the input buffer, just drop metrics.")
}

type Carbon struct {
	listener         *input.Listener
	schemas          *conf.Schemas
	buf              chan []byte
	flushWg          sync.WaitGroup
	authPlugin       auth.AuthPlugin
	requirePublisher bool
}

func InitCarbon(requirePublisher bool) *Carbon {
	if !Enabled {
		return &Carbon{}
	}

	c := &Carbon{
		authPlugin:       auth.GetAuthPlugin(authPlugin),
		requirePublisher: requirePublisher,
		buf:              make(chan []byte, bufferSize),
	}
	// note that we use our Carbon ingest plugin directly as Dispatcher
	c.listener = input.NewListener(addr, 2*time.Minute, input.NewPlain(c))
	c.listener.HandleConn = handleConn
	err := c.listener.Start()
	if err != nil {
		log.Fatal(err)
	}
	c.flushWg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go c.flush()
	}
	return c
}

func (c *Carbon) Stop() {
	if !Enabled {
		return
	}
	// note: this will only return when the handler is done too,
	// which means invocations of Dispatch() will be done too.
	c.listener.Stop()
	close(c.buf)
	c.flushWg.Wait()
}

// IncNumInvalid does not apply for plain text, so is a no-op.
func (c *Carbon) IncNumInvalid() {
}

func (c *Carbon) Dispatch(buf []byte) {
	if len(buf) == 0 {
		return
	}
	buf_copy := make([]byte, len(buf))
	copy(buf_copy, buf)
	metricsReceived.Inc()
	if nonBlockingBuffer {
		select {
		case c.buf <- buf_copy:
		default:
			metricsDroppedBufferFull.Inc()
			log.Debugln("metric dropped due to full buffer")
			// maybe we should just close the connection here
		}
	} else {
		c.buf <- buf_copy
	}
}

func (c *Carbon) flush() {
	defer c.flushWg.Done()
	buf := make([]*schema.MetricData, 0)

	ticker := time.NewTicker(flushInterval)
	for {
		select {
		case <-ticker.C:
			err := publish.Publish(buf)
			if err != nil {
				log.Errorf("failed to publish metrics. %s", err)
				metricsFailed.Add(len(buf))
				continue
			}
			metricsValid.Add(len(buf))
			for _, m := range buf {
				metricPool.Put(m)
			}
			buf = buf[0:0]
		case b, ok := <-c.buf:
			if !ok {
				return
			}
			_, _, _, err := m20.ValidatePacket(b, m20.StrictLegacy, m20.NoneM20)
			if err != nil {
				log.Debugf("packet rejected with error. %s - %s", err, b)
				metricsRejected.Inc()
				continue
			}

			parts := bytes.SplitN(b, []byte("."), 2)
			user, err := c.authPlugin.Auth("api_key", string(parts[0]))
			if err != nil {
				log.Debugf("invalid auth key. %s, reason: %v", parts[0], err)
				metricsDroppedAuthFail.Inc()
				continue
			}
			if c.requirePublisher && !user.Role.IsPublisher() {
				log.Debugf("invalid auth key. %s, reason: user does not have permissions to publish", parts[0])
				metricsDroppedAuthFail.Inc()
				continue
			}
			md, err := parseMetric(parts[1], c.schemas, user.ID)
			if err != nil {
				log.Errorf("could not parse metric %q: %s", string(parts[1]), err)
				metricsRejected.Inc()
				continue
			}
			buf = append(buf, md)
		}
	}
}
