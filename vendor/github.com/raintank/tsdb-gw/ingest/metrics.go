package ingest

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"

	"github.com/golang/snappy"
	"github.com/grafana/metrictank/stats"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/raintank/schema"
	"github.com/raintank/schema/msg"
	"github.com/raintank/tsdb-gw/api/models"
	"github.com/raintank/tsdb-gw/publish"
	log "github.com/sirupsen/logrus"
)

var (
	metricsValid    = stats.NewCounterRate32("metrics.http.valid")    // valid metrics received (not necessarily published)
	metricsRejected = stats.NewCounterRate32("metrics.http.rejected") // invalid metrics received

	discardedSamples = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gateway",
			Name:      "invalid_samples_total",
			Help:      "The total number of samples that were discarded because they are invalid.",
		},
		[]string{"reason", "org"},
	)
)

func Metrics(ctx *models.Context) {
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

type discardsByReason map[string]int
type discardsByOrg map[int]discardsByReason

func (dbo discardsByOrg) Add(org int, reason string) {
	dbr, ok := dbo[org]
	if !ok {
		dbr = make(discardsByReason)
	}
	dbr[reason]++
	dbo[org] = dbr
}

func prepareIngest(ctx *models.Context, in []*schema.MetricData, toPublish []*schema.MetricData) ([]*schema.MetricData, MetricsResponse) {
	resp := NewMetricsResponse()
	promDiscards := make(discardsByOrg)

	for i, m := range in {
		if !ctx.IsAdmin {
			m.OrgId = ctx.ID
		}
		if m.Mtype == "" {
			m.Mtype = "gauge"
		}
		// some customers still use old raintank-probes that include tags in the wrong format.
		// we need to filter those out.
		m.Tags = nil
		if err := m.Validate(); err != nil {
			log.Debugf("received invalid metric: %v %v %v", m.Name, m.OrgId, m.Tags)
			resp.AddInvalid(err, i)
			promDiscards.Add(m.OrgId, err.Error())
			continue
		}
		if !ctx.IsAdmin {
			m.SetId()
		}
		toPublish = append(toPublish, m)
	}

	// track invalid/discards in graphite and prometheus
	metricsRejected.Add(resp.Invalid)
	metricsValid.Add(len(toPublish))
	for org, promDiscardsByOrg := range promDiscards {
		for reason, cnt := range promDiscardsByOrg {
			discardedSamples.WithLabelValues(reason, strconv.Itoa(org)).Add(float64(cnt))
		}
	}
	return toPublish, resp
}

func metricsJson(ctx *models.Context) {
	if ctx.Req.Request.Body == nil {
		ctx.JSON(400, "no data included in request.")
		return
	}
	defer ctx.Req.Request.Body.Close()
	body, err := ioutil.ReadAll(ctx.Req.Request.Body)
	if err != nil {
		select {
		case <-ctx.Req.Context().Done():
			ctx.Error(499, "request canceled")
		default:
			log.Errorf("unable to read request body. %s", err)
			ctx.JSON(500, err)
		}
		return
	}
	metrics := make([]*schema.MetricData, 0)
	err = json.Unmarshal(body, &metrics)
	if err != nil {
		ctx.JSON(400, fmt.Sprintf("unable to parse request body. %s", err))
		return
	}

	toPublish := make([]*schema.MetricData, 0, len(metrics))
	toPublish, resp := prepareIngest(ctx, metrics, toPublish)

	select {
	case <-ctx.Req.Context().Done():
		ctx.Error(499, "request canceled")
		return
	default:
	}

	err = publish.Publish(toPublish)
	if err != nil {
		log.Errorf("failed to publish metrics. %s", err)
		ctx.JSON(500, err)
		return
	}

	// track published in the response (it already has discards)
	resp.Published = len(toPublish)
	ctx.JSON(200, resp)
}

func metricsBinary(ctx *models.Context, compressed bool) {
	if ctx.Req.Request.Body == nil {
		ctx.JSON(400, "no data included in request.")
		return
	}
	var bodyReadCloser io.ReadCloser
	if compressed {
		bodyReadCloser = ioutil.NopCloser(snappy.NewReader(ctx.Req.Request.Body))
	} else {
		bodyReadCloser = ctx.Req.Request.Body
	}
	defer bodyReadCloser.Close()

	body, err := ioutil.ReadAll(bodyReadCloser)
	if err != nil {
		select {
		case <-ctx.Req.Context().Done():
			ctx.Error(499, "request canceled")
		default:
			log.Errorf("unable to read request body. %s", err)
			ctx.JSON(500, err)
		}
		return
	}
	metricData := new(msg.MetricData)
	err = metricData.InitFromMsg(body)
	if err != nil {
		log.Errorf("payload not metricData. %s", err)
		ctx.JSON(400, err)
		return
	}

	err = metricData.DecodeMetricData()
	if err != nil {
		log.Errorf("failed to unmarshal metricData. %s", err)
		ctx.JSON(400, err)
		return
	}

	toPublish := make([]*schema.MetricData, 0, len(metricData.Metrics))
	toPublish, resp := prepareIngest(ctx, metricData.Metrics, toPublish)

	select {
	case <-ctx.Req.Context().Done():
		ctx.Error(499, "request canceled")
		return
	default:
	}

	err = publish.Publish(toPublish)
	if err != nil {
		log.Errorf("failed to publish metrics. %s", err)
		ctx.JSON(500, err)
		return
	}

	// track published in the response (it already has discards)
	resp.Published = len(toPublish)
	ctx.JSON(200, resp)
}
