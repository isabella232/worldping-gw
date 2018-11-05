package graphite

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-macaron/binding"
	"github.com/raintank/dur"
	"gopkg.in/macaron.v1"
)

type FromTo struct {
	From  string `json:"from" form:"from"`
	Until string `json:"until" form:"until"`
	To    string `json:"to" form:"to"` // graphite uses 'until' but we allow to alternatively cause it's shorter
	Tz    string `json:"tz" form:"tz"`
}

func (ft FromTo) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	// we only care about queries to /graphite/render which specify a timerange
	if !strings.HasPrefix(ctx.Req.URL.Path, "/graphite/render") {
		return errs
	}

	now := time.Now()
	defaultFrom := uint32(now.Add(-time.Duration(24) * time.Hour).Unix())
	defaultTo := uint32(now.Unix())
	fromUnix, toUnix, err := getFromTo(ft, now, defaultFrom, defaultTo)
	if err != nil {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"from", "to"},
			Classification: "ValueError",
			Message:        fmt.Sprintf("Unable to parse from/to: %s", err),
		})
		return errs
	}

	if toUnix-fromUnix > timerangeLimit {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"from", "to"},
			Classification: "ValueError",
			Message:        "Time range is too large",
		})
	}

	return errs
}

func getFromTo(ft FromTo, now time.Time, defaultFrom, defaultTo uint32) (uint32, uint32, error) {
	from := ft.From
	to := ft.To
	if to == "" {
		to = ft.Until
	}

	fromUnix, err := dur.ParseDateTime(from, time.Local, now, defaultFrom)
	if err != nil {
		return 0, 0, err
	}

	toUnix, err := dur.ParseDateTime(to, time.Local, now, defaultTo)
	if err != nil {
		return 0, 0, err
	}

	return fromUnix, toUnix, nil
}
