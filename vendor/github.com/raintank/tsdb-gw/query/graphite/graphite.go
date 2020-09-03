package graphite

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/raintank/tsdb-gw/api/models"
	"github.com/raintank/tsdb-gw/util"
	log "github.com/sirupsen/logrus"
	"gopkg.in/macaron.v1"
)

var (
	GraphiteUrl    *url.URL
	WorldpingUrl   *url.URL
	gProxy         httputil.ReverseProxy
	timerangeLimit uint32
)

type proxyRetryTransport struct {
	transport http.RoundTripper
}

func NewProxyRetrytransport() *proxyRetryTransport {
	return &proxyRetryTransport{
		transport: http.DefaultTransport,
	}
}

func (t *proxyRetryTransport) RoundTrip(outreq *http.Request) (*http.Response, error) {
	span := opentracing.SpanFromContext(outreq.Context())
	span = span.Tracer().StartSpan("graphiteProxy.RoundTrip", opentracing.ChildOf(span.Context()))
	defer span.Finish()

	attempts := 0
	var res *http.Response
	var err error
	hasBody := false
	var body []byte
	if outreq.Body != nil {
		body, err = ioutil.ReadAll(outreq.Body)
		if err != nil {
			return res, err
		}
		hasBody = true
	}

	for {
		attempts++
		if hasBody {
			outreq.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		}
		attempt_span := span.Tracer().StartSpan(fmt.Sprintf("attempt %d", attempts), opentracing.ChildOf(span.Context()))
		carrier := opentracing.HTTPHeadersCarrier(outreq.Header)
		err = opentracing.GlobalTracer().Inject(attempt_span.Context(), opentracing.HTTPHeaders, carrier)
		if err != nil {
			log.Errorf("CLU failed to inject span into headers: %s", err)
		}

		res, err = t.transport.RoundTrip(outreq)
		attempt_span.Finish()
		if err == nil {
			break
		}

		if attempts <= 3 {
			log.Infof("graphiteProxy: request to %v failed, will retry: %s", outreq.URL.Host, err)
		} else {
			log.Errorf("graphiteProxy: request to %v failed 3 times. Giving up: %s", outreq.URL.Host, err)
			break
		}
	}

	span.SetTag("attempts", attempts)
	return res, err
}

func Init(graphiteUrl string, limit uint32) error {
	timerangeLimit = limit
	var err error
	GraphiteUrl, err = url.Parse(graphiteUrl)
	if err != nil {
		return err
	}

	gProxy.Director = func(req *http.Request) {
		req.URL.Scheme = GraphiteUrl.Scheme
		req.URL.Host = GraphiteUrl.Host
	}
	gProxy.Transport = NewProxyRetrytransport()
	gProxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		if gProxy.ErrorLog != nil {
			gProxy.ErrorLog.Printf("http: proxy error: %v", err)
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

	return nil
}

func Proxy(orgId int, c *macaron.Context) {
	proxyPath := c.Params("*")

	c.Req.Request.Header.Del("X-Org-Id")
	c.Req.Request.Header.Add("X-Org-Id", strconv.FormatInt(int64(orgId), 10))
	c.Req.Request.URL.Path = util.JoinUrlFragments(GraphiteUrl.Path, proxyPath)
	gProxy.ServeHTTP(c.Resp, c.Req.Request)
}

func GraphiteProxy(c *models.Context) {
	if c.Body != nil {
		c.Req.Request.Body = c.Body
	}

	Proxy(c.ID, c.Context)
}
