package metrictank

import "github.com/raintank/tsdb-gw/api/models"

func PrometheusProxy(c *models.Context) {
	proxyPath := c.Params("*")
	proxy := Proxy(c.ID, "/prometheus/"+proxyPath)
	proxy.ServeHTTP(c.Resp, c.Req.Request)
}
