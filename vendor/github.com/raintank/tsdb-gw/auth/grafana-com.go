package auth

import (
	"github.com/raintank/tsdb-gw/auth/gcom"
)

type GrafanaComAuth struct{}

func NewGrafanaComAuth() *GrafanaComAuth {
	gcom.InitTokenCache()
	return &GrafanaComAuth{}
}

func (a *GrafanaComAuth) Auth(username, password string) (*User, error) {
	if username != "api_key" {
		return nil, ErrInvalidCredentials
	}
	u, err := gcom.Auth(AdminKey, password)
	if err != nil {
		if err == gcom.ErrInvalidApiKey {
			return nil, ErrInvalidCredentials
		}
		if err == gcom.ErrInvalidOrgId {
			return nil, ErrInvalidOrgId
		}
		return nil, err
	}

	return &User{
		ID:      int(u.OrgId),
		IsAdmin: u.IsAdmin,
		Role:    u.Role,
	}, nil
}

func (a *GrafanaComAuth) Stop() {
	gcom.StopTokenCache()
	gcom.StopInstanceCache()
}
