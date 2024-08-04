package db

import (
	"log"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/ostafen/clover/v2"
	"go.hacdias.com/indielib/indieauth"
)

type Storage struct {
	Docs          *clover.DB
	Authorization *ttlcache.Cache[string, *indieauth.AuthenticationRequest]
}

type CollectionName string

var (
	PostCollection CollectionName = "posts"
)

func NewStorage() *Storage {
	c, err := clover.Open("data/docs")
	if err != nil {
		log.Fatal(err)
	}

	cache := ttlcache.New[string, *indieauth.AuthenticationRequest](
		ttlcache.WithTTL[string, *indieauth.AuthenticationRequest](10 * time.Minute),
	)

	go cache.Start()

	store := &Storage{
		Docs:          c,
		Authorization: cache,
	}

	store.SetupClover()

	return store
}

func (db *Storage) SetupClover() {
	if ok, err := db.Docs.HasCollection(string(PostCollection)); err != nil {
		panic(err)
	} else if !ok {
		err := db.Docs.CreateCollection(string(PostCollection))
		if err != nil {
			panic(err)
		}
	}

	if ok, err := db.Docs.HasIndex(string(PostCollection), "createdAt"); err != nil {
		panic(err)
	} else if !ok {
		err := db.Docs.CreateIndex(string(PostCollection), "createdAt")
		if err != nil {
			panic(err)
		}
	}
}

func (db *Storage) Cleanup() {
	if err := db.Docs.Close(); err != nil {
		panic(err)
	}

	db.Authorization.Stop()
}

func (db *Storage) SetupTokenCache() {

}
