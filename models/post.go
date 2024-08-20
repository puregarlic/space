package models

import (
	"time"

	"go.hacdias.com/indielib/microformats"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Post struct {
	ID ULID `gorm:"primaryKey;unique"`

	Type            string
	MicroformatType microformats.Type
	Properties      datatypes.JSON

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (p *Post) Timestamp() string {
	return p.CreatedAt.Format("01/02/2006 at 3:04 PM")
}
