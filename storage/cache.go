package storage

import (
	"time"

	"github.com/jellydator/ttlcache/v3"
	"go.hacdias.com/indielib/indieauth"
)

var authCache *ttlcache.Cache[string, *indieauth.AuthenticationRequest]

func CleanupAuthCache() {
	AuthCache().Stop()
}

func AuthCache() *ttlcache.Cache[string, *indieauth.AuthenticationRequest] {
	if authCache != nil {
		return authCache
	}

	cache := ttlcache.New(
		ttlcache.WithTTL[string, *indieauth.AuthenticationRequest](10 * time.Minute),
	)

	go cache.Start()

	authCache = cache

	return cache
}
