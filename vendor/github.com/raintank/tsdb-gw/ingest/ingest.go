package ingest

import (
	"github.com/raintank/tsdb-gw/util"
)

// MetricPool is a shared buffer for metrics ingested over http
var MetricPool = util.NewMetricDataPool()
