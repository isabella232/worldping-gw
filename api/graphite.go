package api

import (
	"github.com/grafana/worldping-gw/graphite"
)

func GraphiteProxy(c *Context) {
	graphite.Proxy(c.OrgId, c.Context)
}
