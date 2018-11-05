package util

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type metricsServer struct {
	srv *http.Server
}

func NewMetricsServer(addr string) *metricsServer {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

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
