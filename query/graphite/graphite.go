package graphite

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/raintank/tsdb-gw/api/models"
	"github.com/raintank/tsdb-gw/query/graphite"
	"github.com/raintank/tsdb-gw/util"
	log "github.com/sirupsen/logrus"
	"gopkg.in/macaron.v1"
)

var (
	GraphiteUrl  *url.URL
	WorldpingUrl *url.URL

	wpProxy       httputil.ReverseProxy
	worldpingHack bool
)

func Init(graphiteUrl, worldpingUrl string) error {
	// init tsdb-gw's graphite querier.
	err := graphite.Init(graphiteUrl, 0)
	if err != nil {
		return err
	}
	if worldpingUrl != "" {
		worldpingHack = true
		WorldpingUrl, err = url.Parse(worldpingUrl)
		if err != nil {
			return err
		}

		wpProxy.Director = func(req *http.Request) {
			req.URL.Scheme = WorldpingUrl.Scheme
			req.URL.Host = WorldpingUrl.Host
		}
	}

	return nil
}

func Proxy(orgId int, c *macaron.Context) {
	proxyPath := c.Params("*")

	// check if this is a special raintank_db c.Req.Requests then proxy to the worldping-api service.
	if worldpingHack && proxyPath == "metrics/find" && c.Req.Method == "GET" {
		log.Debug("proxying metrics/find request to worldping-api")
		query := c.Req.Request.FormValue("query")
		if strings.HasPrefix(query, "raintank_db") {
			c.Req.Request.URL.Path = util.JoinUrlFragments(WorldpingUrl.Path, "/api/graphite/"+proxyPath)
			wpProxy.ServeHTTP(c.Resp, c.Req.Request)
			return
		}
	}
	// call tsdb-gw's Proxy function()
	graphite.Proxy(orgId, c)
}

func GraphiteProxy(c *models.Context) {
	if c.Body != nil {
		c.Req.Request.Body = c.Body
	}
	Proxy(c.ID, c.Context)
}
