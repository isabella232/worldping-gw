package ingest

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"

	"github.com/raintank/tsdb-gw/api/models"
)

var (
	mtBulkImporterUrl *url.URL
)

func InitMtBulkImporter(importerUrlStr string) error {
	var err error
	mtBulkImporterUrl, err = url.Parse(importerUrlStr)
	return err
}

func Proxy(orgId int) *httputil.ReverseProxy {
	director := func(req *http.Request) {
		req.URL.Scheme = mtBulkImporterUrl.Scheme
		req.URL.Host = mtBulkImporterUrl.Host
		req.URL.Path = mtBulkImporterUrl.Path
		req.Header.Del("X-Org-Id")
		req.Header.Add("X-Org-Id", strconv.FormatInt(int64(orgId), 10))
	}
	return &httputil.ReverseProxy{Director: director}
}

func MtBulkImporter() func(c *models.Context) {
	return func(c *models.Context) {
		proxy := Proxy(c.ID)
		proxy.ServeHTTP(c.Resp, c.Req.Request)
	}
}
