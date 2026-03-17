package auth

import (
	"errors"

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
}

// CreateUser inserts a new user into the database.
func (r *AuthRepository) CreateUser(input CreateUserInput) error {
	user := models.User{
		ID:           input.ID,
		Email:        input.Email,
		PasswordHash: input.PasswordHash,
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
	err := r.db.Where("email = ?", input.Email).First(&user).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil // Return nil, nil if not found
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}
