package ingest

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/codeskyblue/go-uuid"
	"github.com/golang/glog"
	"github.com/golang/snappy"
	"github.com/grafana/worldping-gw/events/msg"
	"github.com/grafana/worldping-gw/events/publish"
	"github.com/raintank/tsdb-gw/api/models"
	log "github.com/sirupsen/logrus"
)

func Events(ctx *models.Context) {
	contentType := ctx.Req.Header.Get("Content-Type")
	switch contentType {
	case "rt-metric-binary":
		eventsMsgp(ctx, false)
	case "rt-metric-binary-snappy":
		eventsMsgp(ctx, true)
	case "application/json":
		eventsJSON(ctx)
	default:
		ctx.JSON(400, fmt.Sprintf("unknown content-type: %s", contentType))
	}
}

func eventsJSON(ctx *models.Context) {
	defer ctx.Req.Request.Body.Close()
	if ctx.Req.Request.Body != nil {
		body, err := ioutil.ReadAll(ctx.Req.Request.Body)
		if err != nil {
			glog.Errorf("unable to read request body. %s", err)
		}
		event := new(msg.ProbeEvent)
		err = json.Unmarshal(body, event)
		if err != nil {
			ctx.JSON(400, fmt.Sprintf("unable to parse request body. %s", err))
			return
		}
		if !ctx.IsAdmin {
			event.OrgId = int64(ctx.ID)
		}

		u := uuid.NewUUID()
		event.Id = u.String()

		err = publish.Publish([]*msg.ProbeEvent{event})
		if err != nil {
			log.Errorf("failed to publish event. %s", err)
			ctx.JSON(500, err)
			return
		}
		ctx.JSON(200, "ok")
		return
	}
	ctx.JSON(400, "no data included in request.")
}

func eventsMsgp(ctx *models.Context, compressed bool) {
	var body io.ReadCloser
	if compressed {
		body = ioutil.NopCloser(snappy.NewReader(ctx.Req.Request.Body))
	} else {
		body = ctx.Req.Request.Body
	}
	defer body.Close()
	if ctx.Req.Request.Body != nil {
		body, err := ioutil.ReadAll(body)
		if err != nil {
			log.Errorf("unable to read request body. %s", err)
		}
		events, err := msg.ProbeEventsFromMsg(body)
		if err != nil {
			log.Errorf("event payload not Event. %s", err)
			ctx.JSON(500, err)
			return
		}
		for _, event := range events {
			if !ctx.IsAdmin {
				event.OrgId = int64(ctx.ID)
			}
			u := uuid.NewUUID()
			event.Id = u.String()
		}

		err = publish.Publish(events)
		if err != nil {
			log.Errorf("failed to publish Event. %s", err)
			ctx.JSON(500, err)
			return
		}
		ctx.JSON(200, "ok")
		return
	}
	ctx.JSON(400, "no data included in request.")
}
