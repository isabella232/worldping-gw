package auth

import (
	"flag"
	"path"

	ini "github.com/glacjay/goini"
	"github.com/raintank/worldping-api/pkg/log"
)

/*
Reads an ini file containing a section for each auth Key.  Each section contains
the user details associated with that key.

example:
------------------
[ahLaibi7Ee]
orgId = 1
isAdmin = true

[ubi5ZahD6l]
orgId = 23
isAdmin = false
-------------------
*/
type FileAuth struct {
	keys     map[string]*User //map auth key to orgId
	filePath string
}

var filePath string

func init() {
	flag.StringVar(&filePath, "auth-file-path", "/etc/raintank/tsdb-auth.ini", "path to ini file containing user details")
}

func NewFileAuth() *FileAuth {
	log.Info("loading carbon auth file from %s", filePath)
	a := &FileAuth{
		keys:     make(map[string]*User),
		filePath: path.Clean(filePath),
	}

	conf := ini.MustLoad(filePath)

	keys := conf.GetSections()

	for _, key := range keys {
		if key == "" {
			continue
		}
		orgId, ok := conf.GetInt(key, "orgid")
		if !ok {
			log.Error(3, "auth.file: no orgId defined for key %s", key)
			continue
		}
		isAdmin, _ := conf.GetBool(key, "isadmin")
		a.keys[key] = &User{
			OrgId:   orgId,
			IsAdmin: isAdmin,
		}
	}
	if len(a.keys) == 0 {
		log.Fatal(4, "no auth credentials found in auth-file.")
	}

	return a
}

func (a *FileAuth) Auth(userKey string) (*User, error) {
	if userKey == AdminKey {
		return AdminUser, nil
	}
	user, ok := a.keys[userKey]
	if !ok {
		return nil, ErrInvalidKey
	}

	return user, nil
}
