package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type StoreHours struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	FranchiseID uuid.UUID `gorm:"type:uuid;not null;index" json:"franchise_id"`
	DayOfWeek   int       `gorm:"not null" json:"day_of_week"` // 0=Sunday, 6=Saturday
	OpenTime    string    `gorm:"not null;default:'09:00'" json:"open_time"`
	CloseTime   string    `gorm:"not null;default:'21:00'" json:"close_time"`
	IsClosed    bool      `gorm:"default:false" json:"is_closed"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (s *StoreHours) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}
