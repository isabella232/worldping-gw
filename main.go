package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"

	"github.com/golang/glog"
	"github.com/grafana/globalconf"
	"github.com/grafana/worldping-gw/api"
	"github.com/grafana/worldping-gw/elasticsearch"
	"github.com/grafana/worldping-gw/event_publish"
	"github.com/grafana/worldping-gw/graphite"
	"github.com/grafana/worldping-gw/util"
	"github.com/raintank/metrictank/stats"
)

var (
	GitHash     = "(none)"
	showVersion = flag.Bool("version", false, "print version string")
	confFile    = flag.String("config", "/etc/worldping/gw.ini", "configuration file path")

	broker = flag.String("kafka-tcp-addr", "localhost:9092", "kafka tcp address for metrics")

	statsEnabled    = flag.Bool("stats-enabled", false, "enable sending graphite messages for instrumentation")
	statsPrefix     = flag.String("stats-prefix", "worldping-gw.stats.default.$hostname", "stats prefix (will add trailing dot automatically if needed)")
	statsAddr       = flag.String("stats-addr", "localhost:2003", "graphite address")
	statsInterval   = flag.Int("stats-interval", 10, "interval in seconds to send statistics")
	statsBufferSize = flag.Int("stats-buffer-size", 20000, "how many messages (holding all measurements from one interval) to buffer up in case graphite endpoint is unavailable.")

	graphiteUrl      = flag.String("graphite-url", "http://localhost:8080", "graphite-api address")
	worldpingUrl     = flag.String("worldping-url", "", "worldping-api address")
	elasticsearchUrl = flag.String("elasticsearch-url", "http://localhost:9200", "elasticsearch server address")
	esIndex          = flag.String("es-index", "events", "elasticsearch index name")

	tracingEnabled = flag.Bool("tracing-enabled", false, "enable/disable distributed opentracing via jaeger")
	tracingAddr    = flag.String("tracing-addr", "localhost:6831", "address of the jaeger agent to send data to")
)

func main() {
	flag.Parse()

	// Only try and parse the conf file if it exists
	path := ""
	if _, err := os.Stat(*confFile); err == nil {
		path = *confFile
	}
	conf, err := globalconf.NewWithOptions(&globalconf.Options{
		Filename:  path,
		EnvPrefix: "GW_",
	})
	if err != nil {
		glog.Fatalf("error with configuration file: %s", err)
		os.Exit(1)
	}
	conf.ParseAll()

	if *showVersion {
		fmt.Printf("worldping-gw (built with %s, git hash %s)\n", runtime.Version(), GitHash)
		return
	}

	if *statsEnabled {
		stats.NewMemoryReporter()
		hostname, _ := os.Hostname()
		prefix := strings.Replace(*statsPrefix, "$hostname", strings.Replace(hostname, ".", "_", -1), -1)
		stats.NewGraphite(prefix, *statsAddr, *statsInterval, *statsBufferSize)
	} else {
		stats.NewDevnull()
	}

	_, traceCloser, err := util.GetTracer(*tracingEnabled, *tracingAddr)
	if err != nil {
		glog.Fatal("Could not initialize jaeger tracer: %s", err.Error())
	}
	defer traceCloser.Close()

	event_publish.Init(*broker)

	if err := graphite.Init(*graphiteUrl, *worldpingUrl); err != nil {
		glog.Fatal(err.Error())
	}
	if err := elasticsearch.Init(*elasticsearchUrl, *esIndex); err != nil {
		glog.Fatal(err.Error())
	}
	inputs := make([]Stoppable, 0)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	glog.Info("starting up")
	done := make(chan struct{})
	inputs = append(inputs, api.InitApi())
	go handleShutdown(done, interrupt, inputs)

	<-done
}

type Stoppable interface {
	Stop()
}

func handleShutdown(done chan struct{}, interrupt chan os.Signal, inputs []Stoppable) {
	<-interrupt
	glog.Info("shutdown started.")
	var wg sync.WaitGroup
	for _, input := range inputs {
		wg.Add(1)
		go func(plugin Stoppable) {
			plugin.Stop()
			wg.Done()
		}(input)
	}
	wg.Wait()
	close(done)
}
