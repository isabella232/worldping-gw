package util

import (
	"sync"

	schema "github.com/raintank/schema"
)

type MetricDataPool struct {
	pool sync.Pool
}

func NewMetricDataPool() *MetricDataPool {
	return &MetricDataPool{pool: sync.Pool{
		New: func() interface{} { return new(schema.MetricData) },
	}}
}

func (b *MetricDataPool) Get() *schema.MetricData {
	return b.pool.Get().(*schema.MetricData)
}

func (b *MetricDataPool) Put(m *schema.MetricData) {
	b.pool.Put(m)
}
