package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LoyaltyHistory struct {
	ID          uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID      uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	User        User       `gorm:"foreignKey:UserID" json:"-"`
	Points      int        `gorm:"not null" json:"points"`
	Type        string     `gorm:"not null" json:"type"` // "earned" or "redeemed"
	Description string     `json:"description"`
	OrderID     *uuid.UUID `gorm:"type:uuid" json:"order_id,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

func (h *LoyaltyHistory) BeforeCreate(tx *gorm.DB) error {
	if h.ID == uuid.Nil {
		h.ID = uuid.New()
	}
	return nil
}
