package metrictank

import (
	"context"
	"log"
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
	var mProxy httputil.ReverseProxy
	mProxy.Director = func(req *http.Request) {
		req.URL.Scheme = MetrictankUrl.Scheme
		req.URL.Host = MetrictankUrl.Host
		req.URL.Path = util.JoinUrlFragments(MetrictankUrl.Path, path)
		req.Header.Del("X-Org-Id")
		req.Header.Add("X-Org-Id", strconv.FormatInt(int64(orgId), 10))
	}
	mProxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		if mProxy.ErrorLog != nil {
			mProxy.ErrorLog.Printf("http: proxy error: %v", err)
		} else {
			log.Printf("http: proxy error: %v", err)
		}

		if req.Context().Err() == context.Canceled {
			// if the client disconnected before the query was fully processed
			rw.WriteHeader(499)
		} else {
			rw.WriteHeader(http.StatusBadGateway)
		}
	}
	return &mProxy
}

func MetrictankProxy(path string) func(c *models.Context) {
	return func(c *models.Context) {
		proxy := Proxy(c.ID, path)
		proxy.ServeHTTP(c.Resp, c.Req.Request)
	}
}
