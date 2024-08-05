package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Post struct {
	ID ULID `gorm:"primaryKey;unique"`

	Type       string
	Properties datatypes.JSON

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
