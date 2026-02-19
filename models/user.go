package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID                    uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Email                 string    `gorm:"uniqueIndex;not null" json:"email"`
	Password              string    `gorm:"not null" json:"-"`
	Name                  string    `json:"name"`
	Role                  string    `gorm:"default:customer" json:"role"` // customer, franchise_owner, franchise_staff, admin
	FranchiseID           *uuid.UUID `gorm:"type:uuid;index" json:"franchise_id,omitempty"`
	LoyaltyPoints         int        `gorm:"default:0" json:"loyalty_points"`
	Phone                 string `json:"phone"`
	IsBlocked             bool   `gorm:"default:false" json:"is_blocked"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}
