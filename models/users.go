package models

import (
	"time"

	"github.com/google/uuid"
)

type Users struct {
	ID           uuid.UUID `gorm:"primaryKey;type:uuid" json:"id"`
	UserName     string    `gorm:"column:username;type:varchar(100);not null" json:"username"`
	Email        string    `gorm:"type:varchar(255);uniqueIndex;not null" json:"email"`
	PasswordHash string    `gorm:"column:password_hash;type:text;not null" json:"-"`

	Role Role `gorm:"type:varchar(20);not null;default:VIEWER" json:"role"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updatedAt"`
}
