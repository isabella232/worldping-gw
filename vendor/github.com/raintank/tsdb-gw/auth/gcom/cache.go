package gcom

import (
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	defaultCacheTTL = time.Hour
	tokenCache      *TokenCache
	instanceCache   *InstanceCache
)

func InitTokenCache() {
	if tokenCache == nil {
		tokenCache = &TokenCache{
			items: make(map[string]*TokenResp),
			stop:  make(chan struct{}),

			cacheTTL: defaultCacheTTL,
		}
		go tokenCache.backgroundValidation()
	}
}

func StopTokenCache() {
	close(tokenCache.stop)
}

func InitInstanceCache() {
	if instanceCache == nil {
		instanceCache = &InstanceCache{
			items: make(map[string]*InstanceResp),
			stop:  make(chan struct{}),

			cacheTTL: defaultCacheTTL,
		}
		go instanceCache.backgroundValidation()
	}
}

func StopInstanceCache() {
	close(instanceCache.stop)
}

type TokenCache struct {
	sync.RWMutex
	items map[string]*TokenResp
	stop  chan struct{}

	cacheTTL time.Duration
}

type TokenResp struct {
	User      *SignedInUser
	retrieved time.Time
	lastRead  int64
}

func (c *TokenCache) Get(key string) (*SignedInUser, bool) {
	var user *SignedInUser
	c.RLock()
	i, ok := c.items[key]
	if ok {
		atomic.StoreInt64(&i.lastRead, time.Now().Unix())
		user = i.User
	}
	c.RUnlock()
	return user, ok
}

func (c *TokenCache) Set(key string, u *SignedInUser) {
	log.Debugf("Auth: Caching token validation response for %s", c.cacheTTL.String())
	now := time.Now()
	c.Lock()
	c.items[key] = &TokenResp{
		User:      u,
		retrieved: now,
		lastRead:  now.Unix(),
	}
	c.Unlock()
}

func (c *TokenCache) Clear() {
	c.Lock()
	c.items = make(map[string]*TokenResp)
	c.Unlock()
}

func (c *TokenCache) backgroundValidation() {
	ticker := time.NewTicker(c.cacheTTL / 2)
	for {
		select {
		case <-c.stop:
			return
		case t := <-ticker.C:
			c.validate(t)
		}
	}
}

func (c *TokenCache) validate(now time.Time) {
	oldestAllowed := now.Add(-1 * c.cacheTTL)
	// We want to hold the ReadLock for as short a time as possible,
	// so we lock, then get all of the keys in the cache in one go.
	c.RLock()
	var keys []string
	for k, v := range c.items {
		// if we are past our expiryTime we want this item.
		if v.retrieved.Before(oldestAllowed) {
			keys = append(keys, k)
		}
	}
	c.RUnlock()

	for _, k := range keys {
		user, err := ValidateToken(k)
		if err != nil && err != ErrInvalidApiKey {
			// we failed to validate the token.  Grafana.com might be down.
			// The current TokenResp is kept, and we will try to validate it
			// again in (cacheTTL/2)
			log.Warnf("Auth: Could not validate token. %s", err)
			continue
		}
		c.Lock()
		v, ok := c.items[k]
		if !ok {
			// this can only happen if a.Clear() was called after releasing the
			// ReadLock and before acquiring the writeLock
			c.Unlock()
			continue
		}

		// if lastRead is older than retrieved, then this key has not been used
		// since it was last validated, so we should drop it from the cache.
		// we dont need to use atomic.LoadInt64 for lastRead as we are already holding
		// a writeLock
		if v.lastRead < v.retrieved.Unix() {
			delete(c.items, k)
			c.Unlock()
			continue
		}

		v.retrieved = now
		v.User = user
		c.Unlock()
	}
}

type InstanceCache struct {
	sync.RWMutex
	items map[string]*InstanceResp
	stop  chan struct{}

	cacheTTL time.Duration
}

type InstanceResp struct {
	valid     bool
	retrieved time.Time
	lastRead  int64
}

func (c *InstanceCache) Get(key string) (bool, bool) {
	c.RLock()
	i, ok := c.items[key]
	if ok {
		atomic.StoreInt64(&i.lastRead, time.Now().Unix())
	}
	valid := false
	if ok {
		valid = i.valid
	}
	c.RUnlock()
	return valid, ok
}

func (c *InstanceCache) Set(key string, valid bool) {
	now := time.Now()
	c.Lock()
	c.items[key] = &InstanceResp{
		valid:     valid,
		retrieved: now,
		lastRead:  now.Unix(),
	}
	c.Unlock()
}

func (c *InstanceCache) Clear() {
	c.Lock()
	c.items = make(map[string]*InstanceResp)
	c.Unlock()
}

func (c *InstanceCache) backgroundValidation() {
	ticker := time.NewTicker(c.cacheTTL / 2)
	for {
		select {
		case <-c.stop:
			return
		case t := <-ticker.C:
			c.validate(t)
		}
	}
}

func (c *InstanceCache) validate(now time.Time) {
	oldestAllowed := now.Add(-1 * c.cacheTTL)
	// We want to hold the ReadLock for as short a time as possible,
	// so we lock, then get all of the keys in the cache.
	c.RLock()
	var keys []string
	for k, v := range c.items {
		// if we are past our expiryTime we want this item.
		if v.retrieved.Before(oldestAllowed) {
			keys = append(keys, k)
		}
	}
	c.RUnlock()

	for _, k := range keys {
		err := ValidateInstance(k)
		if err != nil && err != ErrInvalidInstanceID {
			// we failed to validate the token.  Grafana.com might be down.
			// The current TokenResp is kept, and we will try to validate it
			// again in (cacheTTL/2)
			log.Warnf("Auth: Could not validate instanceID. %s", err)
			continue
		}
		c.Lock()
		v, ok := c.items[k]
		if !ok {
			// this can only happen if a.Clear() was called after releasing the
			// ReadLock and before acquiring the writeLock
			c.Unlock()
			continue
		}

		// if lastRead is older than retrieved, then this key has not been used
		// since it was last validated, so we should drop it from the cache
		if v.lastRead < v.retrieved.Unix() {
			delete(c.items, k)
			c.Unlock()
			continue
		}

		v.retrieved = now
		v.valid = (err == nil)
		c.Unlock()
	}
}
