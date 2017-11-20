package auth

import (
	gAuth "github.com/raintank/raintank-apps/pkg/auth"
)

type GrafanaComAuth struct {
}

func NewGrafanaComAuth() *GrafanaComAuth {
	return &GrafanaComAuth{}
}

func (a *GrafanaComAuth) Auth(userKey string) (*User, error) {
	u, err := gAuth.Auth(AdminKey, userKey)
	if err != nil {
		if err == gAuth.ErrInvalidApiKey {
			return nil, ErrInvalidKey
		}
		if err == gAuth.ErrInvalidOrgId {
			return nil, ErrInvalidOrgId
		}
		return nil, err
	}
	return &User{
		OrgId:   int(u.OrgId),
		IsAdmin: u.IsAdmin,
	}, nil
}
