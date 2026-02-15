package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type FranchiseStaff struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	FranchiseID uuid.UUID `gorm:"type:uuid;not null;index" json:"franchise_id"`
	Franchise   Franchise `gorm:"foreignKey:FranchiseID" json:"-"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"user_id"`
	User        User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Role        string    `gorm:"not null;default:'staff'" json:"role"` // manager, staff
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (fs *FranchiseStaff) BeforeCreate(tx *gorm.DB) error {
	if fs.ID == uuid.Nil {
		fs.ID = uuid.New()
	}
	return nil
}
