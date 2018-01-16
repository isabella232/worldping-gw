package api

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/golang/glog"
	"github.com/golang/snappy"
	"github.com/grafana/metrictank/stats"
	"github.com/grafana/worldping-gw/metric_publish"
	"gopkg.in/raintank/schema.v1"
	"gopkg.in/raintank/schema.v1/msg"
)

var (
	metricsValid    = stats.NewCounter32("metrics.http.valid")
	metricsRejected = stats.NewCounter32("metrics.http.rejected")
)

func Metrics(ctx *Context) {
	contentType := ctx.Req.Header.Get("Content-Type")
	switch contentType {
	case "rt-metric-binary":
		metricsBinary(ctx, false)
	case "rt-metric-binary-snappy":
		metricsBinary(ctx, true)
	case "application/json":
		metricsJson(ctx)
	default:
		ctx.JSON(400, fmt.Sprintf("unknown content-type: %s", contentType))
	}
}

func metricsJson(ctx *Context) {
	defer ctx.Req.Request.Body.Close()
	if ctx.Req.Request.Body != nil {
		body, err := ioutil.ReadAll(ctx.Req.Request.Body)
		if err != nil {
			glog.Errorf("unable to read request body. %s", err)
		}
		metrics := make([]*schema.MetricData, 0)
		err = json.Unmarshal(body, &metrics)
		if err != nil {
			ctx.JSON(400, fmt.Sprintf("unable to parse request body. %s", err))
			return
		}

		if ctx.IsAdmin {
			for _, m := range metrics {
				m.Metric = m.Name

				if m.Mtype == "" {
					m.Mtype = "gauge"
				}
				m.Tags = nil
				if err := m.Validate(); err != nil {
					metricsRejected.Add(len(metrics))
					ctx.JSON(400, err.Error())
					return
				}
			}
		} else {
			for _, m := range metrics {
				m.OrgId = ctx.OrgId
				m.Metric = m.Name

				if m.Mtype == "" {
					m.Mtype = "gauge"
				}
				m.Tags = nil
				if err := m.Validate(); err != nil {
					metricsRejected.Add(len(metrics))
					ctx.JSON(400, err.Error())
					return
				}
				m.SetId()
			}
		}
		metricsValid.Add(len(metrics))
		err = metric_publish.Publish(metrics)
		if err != nil {
			glog.Errorf("failed to publish metrics. %s", err)
			ctx.JSON(500, err)
			return
		}
		ctx.JSON(200, "ok")
		return
	}
	ctx.JSON(400, "no data included in request.")
}

func metricsBinary(ctx *Context, compressed bool) {
	var body io.ReadCloser
	if compressed {
		body = ioutil.NopCloser(snappy.NewReader(ctx.Req.Request.Body))
	} else {
		body = ctx.Req.Request.Body
	}
	defer body.Close()

	if ctx.Req.Request.Body != nil {
		body, err := ioutil.ReadAll(body)
		if err != nil {
			glog.Errorf("unable to read request body. %s", err)
			ctx.JSON(500, err)
			return
		}
		metricData := new(msg.MetricData)
		err = metricData.InitFromMsg(body)
		if err != nil {
			glog.Errorf("payload not metricData. %s", err)
			ctx.JSON(500, err)
			return
		}

		err = metricData.DecodeMetricData()
		if err != nil {
			glog.Errorf("failed to unmarshal metricData. %s", err)
			ctx.JSON(500, err)
			return
		}

		if ctx.IsAdmin {
			for _, m := range metricData.Metrics {
				m.Metric = m.Name

				if m.Mtype == "" {
					m.Mtype = "gauge"
				}
				m.Tags = nil

				if err := m.Validate(); err != nil {
					metricsRejected.Add(len(metricData.Metrics))
					ctx.JSON(400, err.Error())
					return
				}
			}
		} else {
			for _, m := range metricData.Metrics {
				m.OrgId = ctx.OrgId
				m.Metric = m.Name

				if m.Mtype == "" {
					m.Mtype = "gauge"
				}
				m.Tags = nil
				if err := m.Validate(); err != nil {
					metricsRejected.Add(len(metricData.Metrics))
					ctx.JSON(400, err.Error())
					return
				}
				m.SetId()
			}
		}
		metricsValid.Add(len(metricData.Metrics))
		err = metric_publish.Publish(metricData.Metrics)
		if err != nil {
			glog.Errorf("failed to publish metrics. %s", err)
			ctx.JSON(500, err)
			return
		}
		ctx.JSON(200, "ok")
		return
	}
	ctx.JSON(400, "no data included in request.")
}
