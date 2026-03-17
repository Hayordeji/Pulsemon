package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"Pulsemon/pkg/config"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailAlreadyExists = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
)

type AuthService struct {
	repo      *AuthRepository
	jwtSecret string
}

func NewAuthService(repo *AuthRepository, cfg config.Config) *AuthService {
	return &AuthService{
		repo:      repo,
		jwtSecret: cfg.JWTSecret,
	}
}

type RegisterInput struct {
	Email    string
	Password string
}

type LoginInput struct {
	Email    string
	Password string
}

func (s *AuthService) Register(ctx context.Context, input RegisterInput) error {
	if !strings.Contains(input.Email, "@") || !strings.Contains(strings.Split(input.Email, "@")[1], ".") {
		return errors.New("invalid email format")
	}

	user, err := s.repo.FindUserByEmail(FindUserByEmailInput{Email: input.Email})
	if err != nil {
		return fmt.Errorf("failed to check existing user: %w", err)
	}
	if user != nil {
		return ErrEmailAlreadyExists
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	err = s.repo.CreateUser(CreateUserInput{
		ID:           uuid.New(),
		Email:        input.Email,
		PasswordHash: string(hashedPassword),
	})
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	slog.Info("user registered successfully", "email", input.Email)

	return nil
}

func (s *AuthService) Login(ctx context.Context, input LoginInput) (string, error) {
	user, err := s.repo.FindUserByEmail(FindUserByEmailInput{Email: input.Email})
	if err != nil {
		return "", fmt.Errorf("failed to fetch user: %w", err)
	}
	if user == nil {
		return "", ErrInvalidCredentials
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password))
	if err != nil {
		return "", ErrInvalidCredentials
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID.String(),
		"email":   user.Email,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	})

	tokenString, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	slog.Info("user logged in successfully", "email", input.Email)

	return tokenString, nil
}
