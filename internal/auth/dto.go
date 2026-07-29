package auth

import (
	"canepanion-server/models"
	"time"
)

type SignUpRequest struct {
	UserName        string      `json:"username" binding:"required,min=3,max=100"`
	Email           string      `json:"email" binding:"required,email,max=255"`
	Password        string      `json:"password" binding:"required"`
	ConfirmPassword string      `json:"confirmPassword" binding:"required"`
	Role            models.Role `json:"role"`
}

type SignInRequest struct {
	Email    string `json:"email" binding:"required,email,max=255"`
	Password string `json:"password" binding:"required"`
}

type JWTAuthResponse struct {
	ID   string `json:"id"`
	Role string `json:"role" binding:"omitempty,oneof=ADMIN CANE_USER GUARDIAN"`
}

type CurrentUserResponse struct {
	ID        string      `json:"id"`
	UserName  string      `json:"username"`
	Email     string      `json:"email"`
	Role      models.Role `json:"role"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}
