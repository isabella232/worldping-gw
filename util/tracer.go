package util

import (
	"io"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/raintank/worldping-api/pkg/log"
	jaeger "github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	jaegerlog "github.com/uber/jaeger-client-go/log"
)

func GetTracer(enabled bool, addr string) (opentracing.Tracer, io.Closer, error) {
	// Sample configuration for testing. Use constant sampling to sample every trace
	// and enable LogSpan to log every span via configured Logger.
	cfg := jaegercfg.Configuration{
		Disabled: !enabled,
		Sampler: &jaegercfg.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		},
		Reporter: &jaegercfg.ReporterConfig{
			LogSpans:           true,
			LocalAgentHostPort: addr,
		},
	}

	if enabled {
		log.Info("Tracing enabled")
	} else {
		log.Info("Tracing disabled")
	}

	jLogger := jaegerlog.StdLogger

	tracer, closer, err := cfg.New(
		"worldping-gw",
		jaegercfg.Logger(jLogger),
	)
	if err != nil {
		return nil, nil, err
	}
	opentracing.InitGlobalTracer(tracer)
	return tracer, closer, nil
}
