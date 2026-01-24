package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Product struct {
	ID          uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name        string         `gorm:"not null" json:"name"`
	Price       float64        `gorm:"not null" json:"price"`
	CategoryID  uuid.UUID      `gorm:"type:uuid;not null" json:"category_id"`
	Category    Category       `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	Stock       int            `gorm:"default:0" json:"stock"`
	Description string         `json:"description"`
	PackSize    string         `json:"pack_size"`
	IsVegan     bool           `gorm:"default:false" json:"is_vegan"`
	IsGlutenFree bool          `gorm:"default:false" json:"is_gluten_free"`
	Brand       string         `json:"brand"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Images      []ProductImage `gorm:"foreignKey:ProductID" json:"images,omitempty"`
}

func (p *Product) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}
