package util

import (
	"log"
	"net/http"
	"net/http/pprof"
	"runtime"
	rpprof "runtime/pprof"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type metricsServer struct {
	srv *http.Server
}

// NewMetricsServer starts an http server which acts as a
// Prometheus scrape and pprof debug endpoint.
func NewMetricsServer(addr string) *metricsServer {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	// our custom pprof handlers
	mux.HandleFunc("/debug/pprof/block", blockHandler)
	mux.HandleFunc("/debug/pprof/mutex", mutexHandler)

	// normal pprof handlers
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	mux.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	mux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	mux.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Failed to start metrics server: %v", err)
		}
	}()

	return &metricsServer{srv}
}

func (m *metricsServer) Stop() {
	m.srv.Close()
}

// blockhandler writes out a blocking profile
// similar to the standard library handler,
// except it allows to specify a rate.
// The profiler aims to sample an average of one blocking event
// per rate nanoseconds spent blocked.
//
// To include every blocking event in the profile, pass rate = 1.
// Defaults to 10k (10 microseconds)
func blockHandler(w http.ResponseWriter, r *http.Request) {
	debug, _ := strconv.Atoi(r.FormValue("debug"))
	sec, _ := strconv.ParseInt(r.FormValue("seconds"), 10, 64)
	if sec == 0 {
		sec = 30
	}
	rate, _ := strconv.Atoi(r.FormValue("rate"))
	if rate == 0 {
		rate = 10000
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	runtime.SetBlockProfileRate(rate)
	time.Sleep(time.Duration(sec) * time.Second)
	runtime.SetBlockProfileRate(0)
	p := rpprof.Lookup("block")
	p.WriteTo(w, debug)
}

// mutexHandler writes out a mutex profile similar to the
// standard library handler,
// except it allows to specify a mutex profiling rate.
// On average 1/rate events are reported.  The default is 1000
func mutexHandler(w http.ResponseWriter, r *http.Request) {
	debug, _ := strconv.Atoi(r.FormValue("debug"))
	sec, _ := strconv.ParseInt(r.FormValue("seconds"), 10, 64)
	if sec == 0 {
		sec = 30
	}
	rate, _ := strconv.Atoi(r.FormValue("rate"))
	if rate == 0 {
		rate = 1000
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	runtime.SetMutexProfileFraction(rate)
	time.Sleep(time.Duration(sec) * time.Second)
	runtime.SetMutexProfileFraction(0)
	p := rpprof.Lookup("mutex")
	p.WriteTo(w, debug)
}
