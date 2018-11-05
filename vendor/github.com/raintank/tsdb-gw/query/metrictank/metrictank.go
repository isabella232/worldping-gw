package metrictank

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"

	"github.com/raintank/tsdb-gw/api/models"
	"github.com/raintank/tsdb-gw/util"
)

var (
	MetrictankUrl *url.URL
)

func Init(metrictankUrl string) error {
	var err error
	MetrictankUrl, err = url.Parse(metrictankUrl)
	if err != nil {
		return err
	}
	return err
}

func Proxy(orgId int, path string) *httputil.ReverseProxy {
	director := func(req *http.Request) {
		req.URL.Scheme = MetrictankUrl.Scheme
		req.URL.Host = MetrictankUrl.Host
		req.URL.Path = util.JoinUrlFragments(MetrictankUrl.Path, path)
		req.Header.Del("X-Org-Id")
		req.Header.Add("X-Org-Id", strconv.FormatInt(int64(orgId), 10))
	}
	return &httputil.ReverseProxy{Director: director}
}

func MetrictankProxy(path string) func(c *models.Context) {
	return func(c *models.Context) {
		proxy := Proxy(c.ID, path)
		proxy.ServeHTTP(c.Resp, c.Req.Request)
	}
}
