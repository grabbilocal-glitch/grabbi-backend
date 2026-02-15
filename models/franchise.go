package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Franchise struct {
	ID              uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name            string         `gorm:"not null" json:"name"`
	Slug            string         `gorm:"uniqueIndex;not null" json:"slug"`
	OwnerID         uuid.UUID      `gorm:"type:uuid;not null" json:"owner_id"`
	Owner           User           `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
	Address         string         `json:"address"`
	City            string         `json:"city"`
	PostCode        string         `json:"post_code"`
	Latitude        float64        `gorm:"not null" json:"latitude"`
	Longitude       float64        `gorm:"not null" json:"longitude"`
	DeliveryRadius  float64        `gorm:"default:5" json:"delivery_radius"`
	DeliveryFee     float64        `gorm:"default:4.99" json:"delivery_fee"`
	FreeDeliveryMin float64        `gorm:"default:50" json:"free_delivery_min"`
	Phone           string         `json:"phone"`
	Email           string         `json:"email"`
	IsActive        bool           `gorm:"default:true" json:"is_active"`
	StoreHours      []StoreHours   `gorm:"foreignKey:FranchiseID" json:"store_hours,omitempty"`
	Staff           []FranchiseStaff `gorm:"foreignKey:FranchiseID" json:"staff,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

func (f *Franchise) BeforeCreate(tx *gorm.DB) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	return nil
}
