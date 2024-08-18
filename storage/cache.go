package storage

import (
	"time"

	"github.com/jellydator/ttlcache/v3"
	"go.hacdias.com/indielib/indieauth"
)

var authCache *ttlcache.Cache[string, *indieauth.AuthenticationRequest]
var nonceCache *ttlcache.Cache[string, string]

func CleanupCaches() {
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

func NonceCache() *ttlcache.Cache[string, string] {
	if nonceCache != nil {
		return nonceCache
	}

	cache := ttlcache.New(
		ttlcache.WithTTL[string, string](5 * time.Minute),
	)

	go cache.Start()

	nonceCache = cache

	return cache
}
