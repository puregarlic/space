package db

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/puregarlic/space/models"

	"github.com/jellydator/ttlcache/v3"
	"go.hacdias.com/indielib/indieauth"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Storage struct {
	Db            *gorm.DB
	Authorization *ttlcache.Cache[string, *indieauth.AuthenticationRequest]
	Media         *s3.Client
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

	sdkConfig, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Printf("Couldn't load default configuration. Here's why: %v\n", err)
		panic(err)
	}

	svc := s3.NewFromConfig(sdkConfig, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("https://" + os.Getenv("AWS_S3_ENDPOINT"))
		o.Region = os.Getenv("AWS_REGION")
	})

	cache := ttlcache.New[string, *indieauth.AuthenticationRequest](
		ttlcache.WithTTL[string, *indieauth.AuthenticationRequest](10 * time.Minute),
	)

	go cache.Start()

	store := &Storage{
		Db:            db,
		Authorization: cache,
		Media:         svc,
	}

	return store
}

func (db *Storage) Cleanup() {
	db.Authorization.Stop()
}
