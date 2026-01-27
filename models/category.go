package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Category struct {
	ID            uuid.UUID       `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name          string          `gorm:"not null;index" json:"name"`
	Icon          string          `json:"icon"`
	Description   string          `gorm:"type:text" json:"description"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	DeletedAt     gorm.DeletedAt  `gorm:"index" json:"-"`
	Subcategories []Subcategory   `gorm:"foreignKey:CategoryID" json:"subcategories,omitempty"`
	Products      []Product       `gorm:"foreignKey:CategoryID" json:"products,omitempty"`
}

func (c *Category) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}
