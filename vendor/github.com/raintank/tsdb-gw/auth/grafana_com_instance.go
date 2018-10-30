package auth

import (
	"strconv"

	"github.com/raintank/tsdb-gw/auth/gcom"
	log "github.com/sirupsen/logrus"
)

type GrafanaComInstanceAuth struct{}

func NewGrafanaComInstanceAuth() *GrafanaComInstanceAuth {
	gcom.InitTokenCache()
	gcom.InitInstanceCache()
	return &GrafanaComInstanceAuth{}
}

func (a *GrafanaComInstanceAuth) Auth(username, password string) (*User, error) {
	// ensure that the username is an integer.
	instanceID, err := strconv.ParseInt(username, 10, 64)
	if err != nil {
		log.Debugf("unable to parse username: %v", username)
		return nil, ErrInvalidCredentials
	}
	u, err := gcom.Auth(AdminKey, password)
	if err != nil {
		if err == gcom.ErrInvalidApiKey {
			log.Debugf("failed to authenticate request: %v", err)
			return nil, ErrInvalidCredentials
		}
		if err == gcom.ErrInvalidOrgId {
			log.Debugf("failed to authenticate request: %v", err)
			return nil, ErrInvalidOrgId
		}
		return nil, err
	}

	if password != AdminKey {
		err = u.CheckInstance(username)
		if err != nil {
			log.Debugf("invalid credentials, %v", err)
			return nil, ErrInvalidCredentials
		}
	}

	return &User{
		ID:      int(instanceID),
		IsAdmin: u.IsAdmin,
		Role:    u.Role,
	}, nil
}

func (a *GrafanaComInstanceAuth) Stop() {
	gcom.StopTokenCache()
	gcom.StopInstanceCache()
}
