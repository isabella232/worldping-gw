package carbon

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/grafana/metrictank/conf"
	"github.com/grafana/metrictank/schema"
)

var err3Fields = errors.New("need 3 fields")
var errBadTag = errors.New("can't parse tag")

// parseMetric parses a buffer into a MetricData message, using the schemas to deduce the interval of the data.
// The given orgId will be applied to the MetricData
func parseMetric(buf []byte, schemas *conf.Schemas, orgId int) (*schema.MetricData, error) {
	msg := strings.TrimSpace(string(buf))

	elements := strings.Fields(msg)
	if len(elements) != 3 {
		return nil, err3Fields
	}

	metric := strings.Split(elements[0], ";")
	name := metric[0]

	tags := metric[1:]
	for _, v := range tags {
		if v == "" || !strings.Contains(v, "=") || v[0] == '=' {
			return nil, errBadTag
		}
	}

	val, err := strconv.ParseFloat(elements[1], 64)
	if err != nil {
		return nil, fmt.Errorf("can't parse value: %s", err)
	}

	timestamp, err := strconv.ParseUint(elements[2], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("can't parse timestamp: %s", err)
	}

	md := metricPool.Get()
	*md = schema.MetricData{
		Name:     name,
		Interval: 0,
		Value:    val,
		Unit:     "unknown",
		Time:     int64(timestamp),
		Mtype:    "gauge",
		Tags:     tags,
		OrgId:    orgId,
	}
	md.SetId()

	return md, nil
}
