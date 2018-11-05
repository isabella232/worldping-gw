package models

import (
	"io"

	"github.com/raintank/tsdb-gw/auth"
	macaron "gopkg.in/macaron.v1"
)

type Context struct {
	*macaron.Context
	*auth.User
	Body io.ReadCloser
}
