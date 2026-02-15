package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type FranchisePromotion struct {
	ID          uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	FranchiseID uuid.UUID      `gorm:"type:uuid;not null;index" json:"franchise_id"`
	Franchise   Franchise      `gorm:"foreignKey:FranchiseID" json:"-"`
	Title       string         `gorm:"not null" json:"title"`
	Description string         `json:"description"`
	Image       string         `json:"image"`
	ProductURL  string         `gorm:"column:product_url" json:"product_url"`
	IsActive    bool           `gorm:"default:true" json:"is_active"`
	StartDate   *time.Time     `json:"start_date"`
	EndDate     *time.Time     `json:"end_date"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (fp *FranchisePromotion) BeforeCreate(tx *gorm.DB) error {
	if fp.ID == uuid.Nil {
		fp.ID = uuid.New()
	}
	return nil
}
