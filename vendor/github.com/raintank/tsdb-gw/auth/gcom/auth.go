package gcom

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/singleflight"
)

var singlef = &singleflight.Group{}

func init() {
	flag.StringVar(&authEndpoint, "auth-endpoint", authEndpoint, "Endpoint to authenticate users on")
	flag.DurationVar(&defaultCacheTTL, "auth-cache-ttl", defaultCacheTTL, "how long auth responses should be cached")
	flag.Var(&validOrgIds, "auth-valid-org-id", "restrict authentication to the listed orgId (comma separated list)")
	flag.StringVar(&validInstanceType, "auth-valid-instance-type", "", "if set, instance validation while fail if the type attribute of an instance does not match. (graphite|graphite-shared|prometheus|logs)")
}

type int64SliceFlag []int64

func (i *int64SliceFlag) Set(value string) error {
	for _, split := range strings.Split(value, ",") {
		split = strings.TrimSpace(split)
		if split == "" {
			continue
		}
		parsed, err := strconv.Atoi(split)
		if err != nil {
			return err
		}
		*i = append(*i, int64(parsed))
	}
	return nil
}

func (i *int64SliceFlag) String() string {
	// This is just a 1-liner to convert print a slice as a command separated list.
	return strings.Trim(strings.Replace(fmt.Sprint(*i), " ", ", ", -1), "[]")
}

var (
	authEndpoint      = "https://grafana.com"
	validOrgIds       = int64SliceFlag{}
	validInstanceType string

	// global HTTP client.  By sharing the client we can take
	// advantage of keepalives and re-use connections instead
	// of establishing a new tcp connection for every request.
	client = &http.Client{
		Timeout: time.Second * 2,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          10,
			IdleConnTimeout:       300 * time.Second,
			TLSHandshakeTimeout:   5 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
)

func ValidateToken(keyString string) (*SignedInUser, error) {
	user, err, _ := singlef.Do(keyString, func() (interface{}, error) {
		return validateToken(keyString)
	})
	if err != nil {
		return nil, err
	}
	return user.(*SignedInUser), nil
}

func validateToken(keyString string) (*SignedInUser, error) {
	payload := url.Values{}
	payload.Add("token", keyString)

	res, err := client.PostForm(fmt.Sprintf("%s/api/api-keys/check", authEndpoint), payload)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()

	if res.StatusCode >= 500 {
		return nil, fmt.Errorf("Auth token could not be validated: %s", res.Status)
	}

	if res.StatusCode != 200 {
		return nil, ErrInvalidApiKey
	}

	user := &SignedInUser{key: keyString}
	err = json.Unmarshal(body, user)
	if err != nil {
		return nil, err
	}

	valid := false

	if len(validOrgIds) == 0 {
		valid = true
	} else {
		for _, id := range validOrgIds {
			if user.OrgId == id {
				valid = true
				break
			}
		}
	}

	if !valid {
		log.Debugln("Auth: orgID is not listed in auth-valid-org-id setting.")
		return nil, ErrInvalidOrgId
	}

	return user, nil
}

func Auth(adminKey, keyString string) (*SignedInUser, error) {
	if keyString == adminKey {
		return &SignedInUser{
			Role:    ROLE_ADMIN,
			OrgId:   1,
			OrgName: "Admin",
			OrgSlug: "admin",
			IsAdmin: true,
			key:     keyString,
		}, nil
	}

	user, cached := tokenCache.Get(keyString)
	if cached {
		if user != nil {
			log.Debugln("Auth: valid key cached")
			return user, nil
		}
		log.Debugln("Auth: invalid key cached")
		return nil, ErrInvalidApiKey
	}

	var err error
	user, err = ValidateToken(keyString)

	// ErrInvalidApiKey and ErrInvalidOrgId are successful responses so we
	// dont return them here.  Instead we cache the response so that
	// if the token is used again we can reject it straight away.
	if err != nil && err != ErrInvalidApiKey && err != ErrInvalidOrgId {
		return nil, err
	}

	// add the user to the cache.
	tokenCache.Set(keyString, user)
	return user, err
}

func ValidateInstance(cacheKey string) error {
	_, err, _ := singlef.Do(cacheKey, func() (interface{}, error) {
		idKey := strings.SplitN(cacheKey, ":", 2)
		err := validateInstance(idKey[0], idKey[1])
		return nil, err
	})

	return err
}

func validateInstance(instanceID, token string) error {
	var instanceUrl string

	if validInstanceType == "logs" {
		instanceUrl = fmt.Sprintf("%s/api/hosted-logs/%s", authEndpoint, instanceID)
	} else {
		instanceUrl = fmt.Sprintf("%s/api/hosted-metrics/%s", authEndpoint, instanceID)
	}

	req, err := http.NewRequest("GET", instanceUrl, nil)
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	res, err := client.Do(req)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()

	log.Debugf("Auth: %s response was: %s", instanceUrl, body)

	if res.StatusCode >= 500 {
		return err
	}

	if res.StatusCode != 200 {
		return ErrInvalidInstanceID
	}

	instance := &Instance{}
	err = json.Unmarshal(body, instance)
	if err != nil {
		return err
	}

	if strconv.Itoa(int(instance.ID)) != instanceID {
		log.Errorf("Auth: instanceID returned from grafana.com doesnt match requested instanceID. %d != %s", instance.ID, instanceID)
		return ErrInvalidInstanceID
	}

	if validInstanceType != "" && validInstanceType != instance.InstanceType {
		log.Infof("Auth: instanceType returned from grafana.com doesnt match required instanceType. %s != %s", instance.InstanceType, validInstanceType)
		return ErrInvalidInstanceType
	}

	return nil
}

// validate that the signedInUser has a hosted-metrics instance with the
// passed instanceID.  It is assumed that the instanceID has already been
// confirmed to be an integer.
func (u *SignedInUser) CheckInstance(instanceID string) error {
	cachekey := fmt.Sprintf("%s:%s", instanceID, u.key)
	// check the cache
	log.Debugln("Auth: Checking cache for instance")
	valid, cached := instanceCache.Get(cachekey)
	if cached {
		if valid {
			log.Debugln("Auth: valid instance key cached")
			return nil
		}

		log.Debugln("Auth: invalid instance key cached")
		return ErrInvalidInstanceID
	}

	err := ValidateInstance(cachekey)
	// ErrInvalidInstanceID responses are successful responses so we
	// dont return them here.  Instead we cache the response so that
	// if the token is used again we can reject it straight away.
	if err != nil && err != ErrInvalidInstanceID {
		return err
	}

	instanceCache.Set(cachekey, (err == nil))
	return err
}
