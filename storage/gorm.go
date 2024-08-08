package storage

import (
	"log"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var orm *gorm.DB

func GORM() *gorm.DB {
	if orm != nil {
		return orm
	}

	dataDir := filepath.Join(".", "data")
	if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
		log.Fatal(err)
	}

	db, err := gorm.Open(sqlite.Open(filepath.Join(dataDir, "data.db")), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	orm = db

	// TODO: Move migration to `models` package
	// if err := db.AutoMigrate(&models.Post{}); err != nil {
	// 	log.Fatal(err)
	// }

	return db
}
