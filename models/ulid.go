package models

import (
	"database/sql/driver"

	"codeberg.org/gruf/go-ulid"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type ULID ulid.ULID

func NewULID() ULID {
	return ULID(ulid.MustNew())
}

func (ULID) GormDataType() string {
	return "string"
}

func (ULID) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	switch db.Dialector.Name() {
	case "mysql":
		return "LONGTEXT"
	case "postgres":
		return "UUID"
	case "sqlserver":
		return "NVARCHAR"
	case "sqlite":
		return "TEXT"
	default:
		return ""
	}
}

func (u *ULID) Scan(value interface{}) error {
	var result ulid.ULID
	if err := result.Scan(value); err != nil {
		return err
	}
	*u = ULID(result)
	return nil
}

func (u ULID) Value() (driver.Value, error) {
	return ulid.ULID(u).String(), nil
}

func (u ULID) String() string {
	return ulid.ULID(u).String()
}

func (u ULID) Equals(other ULID) bool {
	return u.String() == other.String()
}

func (u ULID) Length() int {
	return len(u.String())
}

func (u ULID) IsNil() bool {
	zero, err := ulid.ParseString("0000000000000000")
	if err != nil {
		panic(err)
	}

	return ulid.ULID(u) == zero
}

func (u ULID) IsEmpty() bool {
	return u.IsNil() || u.Length() == 0
}

func (u *ULID) IsNilPtr() bool {
	return u == nil
}

func (u *ULID) IsEmptyPtr() bool {
	return u.IsNilPtr() || u.IsEmpty()
}
