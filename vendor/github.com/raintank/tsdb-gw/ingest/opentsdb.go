package ingest

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"io/ioutil"

	schema "github.com/grafana/metrictank/schema"
	"github.com/raintank/tsdb-gw/api/models"
	"github.com/raintank/tsdb-gw/publish"
	log "github.com/sirupsen/logrus"
)

func OpenTSDBWrite(ctx *models.Context) {
	if ctx.Req.Request.Body != nil {
		defer ctx.Req.Request.Body.Close()
		var reader io.Reader
		var err error
		if ctx.Req.Header.Get("Content-Encoding") == "gzip" {
			reader, err = gzip.NewReader(ctx.Req.Request.Body)
			if err != nil {
				ctx.JSON(400, err.Error())
				log.Errorf("Read Error, %v", err)
				return
			}
		} else {
			reader = ctx.Req.Request.Body
		}

		body, err := ioutil.ReadAll(reader)
		if err != nil {
			ctx.JSON(400, err.Error())
			log.Errorf("Read Error, %v", err)
			return
		}

		var req OpenTSDBPutRequest
		err = json.Unmarshal(body, &req)
		if err != nil {
			ctx.JSON(400, err.Error())
			log.Errorf("Read Error, %v", err)
			return
		}

		var buf []*schema.MetricData
		for _, ts := range req {
			md := MetricPool.Get()
			*md = schema.MetricData{
				Name:     ts.Metric,
				Interval: 0,
				Value:    ts.Value,
				Unit:     "unknown",
				Time:     ts.Timestamp,
				Mtype:    "gauge",
				Tags:     ts.FormatTags(md.Tags),
				OrgId:    ctx.ID,
			}
			md.SetId()
			buf = append(buf, md)
		}

		err = publish.Publish(buf)
		for _, m := range buf {
			m.Tags = m.Tags[:0]
			MetricPool.Put(m)
		}
		if err != nil {
			log.Errorf("failed to publish opentsdb write metrics. %s", err)
			ctx.JSON(500, err)
			return
		}
		ctx.JSON(200, "ok")
		return
	}

	ctx.JSON(400, "no data included in request.")
}

type OpenTSDBMetric struct {
	Metric    string            `json:"metric"`
	Timestamp int64             `json:"timestamp"`
	Value     float64           `json:"value"`
	Tags      map[string]string `json:"tags"`
}

type OpenTSDBPutRequest []OpenTSDBMetric

func (m OpenTSDBMetric) FormatTags(tagArray []string) []string {
	for t, v := range m.Tags {
		tagArray = append(tagArray, t+"="+v)
	}
	return tagArray
}
