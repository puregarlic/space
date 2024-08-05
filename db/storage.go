package db

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/puregarlic/space/models"

	"github.com/jellydator/ttlcache/v3"
	"go.hacdias.com/indielib/indieauth"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type Storage struct {
	Db            *gorm.DB
	Authorization *ttlcache.Cache[string, *indieauth.AuthenticationRequest]
}

type CollectionName string

var (
	PostCollection CollectionName = "posts"
)

func NewStorage() *Storage {
	dataDir := filepath.Join(".", "data")
	if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
		log.Fatal(err)
	}

	db, err := gorm.Open(sqlite.Open(filepath.Join(dataDir, "data.db")), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	if err := db.AutoMigrate(&models.Post{}); err != nil {
		panic(err)
	}

	cache := ttlcache.New[string, *indieauth.AuthenticationRequest](
		ttlcache.WithTTL[string, *indieauth.AuthenticationRequest](10 * time.Minute),
	)

	go cache.Start()

	store := &Storage{
		// Docs:          c,
		Db:            db,
		Authorization: cache,
	}

	return store
}

func (db *Storage) Cleanup() {
	db.Authorization.Stop()
}
