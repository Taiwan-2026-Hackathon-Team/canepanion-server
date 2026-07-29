package auth

import (
	"canepanion-server/models"
	appErr "canepanion-server/pkg/errors"

	"canepanion-server/pkg/utils"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) SignUp(req SignUpRequest) (*JWTAuthResponse, string, error) {
	// Missing Required Field Validation
	if req.Email == "" || req.UserName == "" || req.Role == "" {
		return nil, "", appErr.NewBadRequest("Missing required fields", nil)
	}

	// Existing User Validation
	existingUser, err := s.repo.FindUserByEmail(req.Email)
	if err != nil {
		return nil, "", appErr.NewBadRequest("Failed to verify if the email exists", err)
	}

	if existingUser != nil {
		return nil, "", appErr.NewBadRequest("User with that email already exists", nil)
	}

	// Role Validation
	if req.Role != "ADMIN" && req.Role != "CANE_USER" && req.Role != "GUARDIAN" {
		return nil, "", appErr.NewBadRequest("Invalid Role", nil)
	}

	// Password Validation
	if req.Password != req.ConfirmPassword {
		return nil, "", appErr.NewBadRequest("Password does not match", nil)
	}

	// Password Hashing
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, "", appErr.NewInternal("Failed to hash the password", nil)
	}

	// Register User Data Preparation
	userId := utils.GenerateUUID()

	newUser := &models.Users{
		ID:           userId,
		UserName:     req.UserName,
		Email:        req.Email,
		PasswordHash: hashedPassword,
		Role:         req.Role,
	}

	createdUser, err := s.repo.CreateUser(newUser)
	if err != nil {
		return nil, "", appErr.NewInternal("Failed to create the user", err)
	}

	registeredUserRes := JWTAuthResponse{
		ID:   createdUser.ID.String(),
		Role: string(createdUser.Role),
	}

	token, err := utils.GenerateJWT(createdUser)
	if err != nil {
		return nil, "", appErr.NewInternal("Failed to generate token", err)
	}

	return &registeredUserRes, token, nil
}

func (s *Service) LogIn(req SignInRequest) (*JWTAuthResponse, string, error) {
	// Missing Required Field Validation
	if req.Email == "" || req.Password == "" {
		return nil, "", appErr.NewBadRequest("Missing required fields", nil)
	}

	// Validate if user exists
	user, err := s.repo.FindUserByEmail(req.Email)
	if err != nil {
		return nil, "", appErr.NewNotFound("Failed to verify if user exists", err)
	}
	if user == nil {
		return nil, "", appErr.NewBadRequest("Invalid email or password", nil)
	}

	if err := utils.ValidatePassword(user.PasswordHash, req.Password); err != nil {
		return nil, "", appErr.NewBadRequest("Invalid email or password", nil)
	}

	// JWT Token Generation
	token, err := utils.GenerateJWT(user)
	if err != nil {
		return nil, "", appErr.NewInternal("Failed to generate token", err)
	}

	// Return Data
	userResp := JWTAuthResponse{
		ID:   user.ID.String(),
		Role: string(user.Role),
	}

	return &userResp, token, nil
}

func (s *Service) GetCurrentUser(userID string) (*CurrentUserResponse, error) {
	if userID == "" {
		return nil, appErr.NewUnauthorized("Missing authenticated user", nil)
	}

	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		return nil, appErr.NewInternal("Failed to get current user", err)
	}
	if user == nil {
		return nil, appErr.NewNotFound("User not found", nil)
	}

	return &CurrentUserResponse{
		ID:        user.ID.String(),
		UserName:  user.UserName,
		Email:     user.Email,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}, nil
}
