package auth

import (
	"errors"
	"strings"
	"time"

	"Pulsemon/pkg/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuthRepository handles database operations for user authentication.
type AuthRepository struct {
	db *gorm.DB
}

// NewAuthRepository creates a new AuthRepository.
func NewAuthRepository(db *gorm.DB) *AuthRepository {
	return &AuthRepository{db: db}
}

// CreateUserInput holds data for creating a new user.
type CreateUserInput struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	RoleID       uuid.UUID
	Username     string
}

// CreateUser inserts a new user into the database.
func (r *AuthRepository) CreateUser(input CreateUserInput) error {
	user := models.User{
		ID:           input.ID,
		Email:        input.Email,
		PasswordHash: input.PasswordHash,
		RoleID:       input.RoleID,
		Username:     input.Username,
	}

	return r.db.Create(&user).Error
}

// FindUserByEmailInput holds data for finding a user.
type FindUserByEmailInput struct {
	Email string
}

// FindUserByEmail retrieves a user by their email address.
func (r *AuthRepository) FindUserByEmail(input FindUserByEmailInput) (*models.User, error) {
	var user models.User
	err := r.db.Where("email = ?", strings.ToLower(input.Email)).First(&user).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil // Return nil, nil if not found
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// FindUserByName retrieves a user by their email address.
func (r *AuthRepository) FindUserByName(userName string) (*models.User, error) {
	var user models.User
	err := r.db.Where("username = ?", strings.ToLower(userName)).First(&user).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil // Return nil, nil if not found
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// FindUserByIDInput holds data for finding a user by ID.
type FindUserByIDInput struct {
	UserID string
}

// FindUserByID retrieves a user by their UUID.
func (r *AuthRepository) FindUserByID(input FindUserByIDInput) (*models.User, error) {
	var user models.User
	err := r.db.Where("id = ?", input.UserID).First(&user).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// UpdateVerificationTokenInput holds data to set the verification token.
type UpdateVerificationTokenInput struct {
	UserID    string
	Token     string
	ExpiresAt time.Time
}

// UpdateVerificationToken sets the verification token and expiry for a user.
func (r *AuthRepository) UpdateVerificationToken(input UpdateVerificationTokenInput) error {
	return r.db.Model(&models.User{}).
		Where("id = ?", input.UserID).
		Updates(map[string]interface{}{
			"verification_token": input.Token,
			"token_expires_at":   input.ExpiresAt,
		}).Error
}

// VerifyUserInput holds data for verifying a user by token.
type VerifyUserInput struct {
	Token  string
	UserID string
}

// VerifyUser retrieves a user by a verification token that hasn't expired.
func (r *AuthRepository) VerifyUser(input VerifyUserInput) (*models.User, error) {
	var user models.User
	err := r.db.Where("verification_token = ? AND token_expires_at > ?", input.Token, time.Now()).First(&user).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// SetVerifiedInput holds data to mark a user as verified.
type SetVerifiedInput struct {
	UserID string
}

// SetVerified marks a user as verified and clears the token details.
func (r *AuthRepository) SetVerified(input SetVerifiedInput) error {
	return r.db.Model(&models.User{}).
		Where("id = ?", input.UserID).
		Updates(map[string]interface{}{
			"is_verified":        true,
			"verification_token": gorm.Expr("NULL"),
			"token_expires_at":   gorm.Expr("NULL"),
		}).Error
}

// SetResetTokenInput holds data to set the reset token.
type SetResetTokenInput struct {
	UserID    string
	Token     string
	ExpiresAt time.Time
}

// SetResetToken sets the reset token and expiry for a user.
func (r *AuthRepository) SetResetToken(input SetResetTokenInput) error {
	return r.db.Model(&models.User{}).
		Where("id = ?", input.UserID).
		Updates(map[string]interface{}{
			"verification_token": input.Token,
			"token_expires_at":   input.ExpiresAt,
		}).Error
}

// FindUserByResetTokenInput holds data for finding a user by reset token.
type FindUserByResetTokenInput struct {
	UserID string
	Token  string
}

// FindUserByResetToken retrieves a user by a reset token that hasn't expired.
func (r *AuthRepository) FindUserByResetToken(input FindUserByResetTokenInput) (*models.User, error) {
	var user models.User
	err := r.db.Where("id = ? AND verification_token = ? AND token_expires_at > ?", input.UserID, input.Token, time.Now()).First(&user).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// UpdatePasswordInput holds data to update a user's password.
type UpdatePasswordInput struct {
	UserID       string
	PasswordHash string
}

// UpdatePassword updates a user's password and clears the verification token.
func (r *AuthRepository) UpdatePassword(input UpdatePasswordInput) error {
	return r.db.Model(&models.User{}).
		Where("id = ?", input.UserID).
		Updates(map[string]interface{}{
			"password_hash":      input.PasswordHash,
			"verification_token": gorm.Expr("NULL"),
			"token_expires_at":   gorm.Expr("NULL"),
		}).Error
}

// SetRefreshTokenInput holds data to set the refresh token.
type SetRefreshTokenInput struct {
	UserID string
	Token  string
	Expiry time.Time
}

// SetRefreshToken sets the refresh token and expiry for a user.
func (r *AuthRepository) SetRefreshToken(input SetRefreshTokenInput) error {
	return r.db.Model(&models.User{}).
		Where("id = ?", input.UserID).
		Updates(map[string]interface{}{
			"refresh_token":        input.Token,
			"refresh_token_expiry": input.Expiry,
		}).Error
}

// FindByRefreshTokenInput holds data for finding a user by refresh token.
type FindByRefreshTokenInput struct {
	Token string
}

// FindUserByRefreshToken retrieves a user by a refresh token that hasn't expired.
func (r *AuthRepository) FindUserByRefreshToken(input FindByRefreshTokenInput) (*models.User, error) {
	var user models.User
	err := r.db.Where("refresh_token = ? AND refresh_token_expiry > ?", input.Token, time.Now()).First(&user).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil // Return nil, nil if not found or expired
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// ClearRefreshTokenInput holds data to clear a user's refresh token.
type ClearRefreshTokenInput struct {
	UserID string
}

// ClearRefreshToken clears a user's refresh token and expiry.
func (r *AuthRepository) ClearRefreshToken(input ClearRefreshTokenInput) error {
	return r.db.Model(&models.User{}).
		Where("id = ?", input.UserID).
		Updates(map[string]interface{}{
			"refresh_token":        gorm.Expr("NULL"),
			"refresh_token_expiry": gorm.Expr("NULL"),
		}).Error
}
