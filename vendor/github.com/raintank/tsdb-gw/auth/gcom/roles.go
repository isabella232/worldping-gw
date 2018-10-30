package gcom

import (
	"errors"
	"time"
)

// Typed errors
var (
	ErrInvalidRoleType   = errors.New("Invalid role type")
	ErrInvalidApiKey     = errors.New("Invalid API Key")
	ErrInvalidOrgId      = errors.New("Invalid Org Id")
	ErrInvalidInstanceID = errors.New("Invalid Instance ID")
)

type RoleType string

const (
	ROLE_VIEWER            RoleType = "Viewer"
	ROLE_EDITOR            RoleType = "Editor"
	ROLE_METRICS_PUBLISHER RoleType = "MetricsPublisher"
	ROLE_ADMIN             RoleType = "Admin"
)

func (r RoleType) IsValid() bool {
	return r == ROLE_VIEWER || r == ROLE_ADMIN || r == ROLE_EDITOR || r == ROLE_METRICS_PUBLISHER
}

func (r RoleType) IsPublisher() bool {
	return r == ROLE_METRICS_PUBLISHER || r == ROLE_ADMIN || r == ROLE_EDITOR
}

func (r RoleType) IsViewer() bool {
	return r == ROLE_VIEWER || r == ROLE_ADMIN || r == ROLE_EDITOR
}

type SignedInUser struct {
	Id        int64     `json:"id"`
	OrgName   string    `json:"orgName"`
	OrgId     int64     `json:"orgId"`
	OrgSlug   string    `json:"orgSlug"`
	Name      string    `json:"name"`
	Role      RoleType  `json:"role"`
	CreatedAt time.Time `json:"createAt"`
	IsAdmin   bool      `json:"-"`
	key       string
}

type Instance struct {
	ID          int64  `json:"id"`
	OrgID       int    `json:"orgId"`
	ClusterName string `json:"clusterName"`
	ClusterID   int    `json:"clusterId"`
}
