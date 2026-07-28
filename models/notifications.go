package models

import (
	"time"

	"github.com/google/uuid"
)

type Notifications struct {
	ID      uuid.UUID `gorm:"primaryKey;type:uuid" json:"id"`
	UserID  uuid.UUID `gorm:"type:uuid;not null;index" json:"userId"`
	AlertID uuid.UUID `gorm:"type:uuid;not null;index" json:"alertId"`
	Message string    `gorm:"type:text;not null" json:"message"`
	IsRead  bool      `gorm:"not null;default:false" json:"isRead"`
	SentAt  time.Time `gorm:"not null;index" json:"sentAt"`

	User  Users  `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	Alert Alerts `gorm:"foreignKey:AlertID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
}
