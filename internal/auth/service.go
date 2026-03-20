package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"Pulsemon/pkg/config"
	"Pulsemon/pkg/roles"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/resendlabs/resend-go"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailAlreadyExists    = errors.New("email already registered")
	ErrUsernameAlreadyExists = errors.New("username already registered")
	ErrInvalidCredentials    = errors.New("invalid email or password")
	ErrInvalidOrExpiredToken = errors.New("invalid or expired verification token")
	ErrAlreadyVerified       = errors.New("email already verified")
	ErrUserIsNotVerified     = errors.New("user is not verified")
)

type AuthService struct {
	repo      *AuthRepository
	jwtSecret string
	roles     *roles.RoleRegistry
	cfg       config.Config
	resend    *resend.Client
}

func NewAuthService(repo *AuthRepository, cfg config.Config, registry *roles.RoleRegistry) *AuthService {
	return &AuthService{
		repo:      repo,
		jwtSecret: cfg.JWTSecret,
		roles:     registry,
		cfg:       cfg,
		resend:    resend.NewClient(cfg.ResendAPIKey),
	}
}

type RegisterInput struct {
	Email    string
	Username string
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

	user, err = s.repo.FindUserByName(input.Username)
	if err != nil {
		return fmt.Errorf("failed to check existing user: %w", err)
	}
	if user != nil {
		return ErrUsernameAlreadyExists
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	userID := uuid.New()
	err = s.repo.CreateUser(CreateUserInput{
		ID:           userID,
		Email:        strings.ToLower(input.Email),
		PasswordHash: string(hashedPassword),
		RoleID:       s.roles.UserRoleID,
		Username:     input.Username,
	})
	if err != nil {
		slog.Error("failed to create user", "error", err)
		return fmt.Errorf("failed to create user: %w", err)
	}

	token := uuid.New().String()
	expiresAt := time.Now().Add(24 * time.Hour)

	err = s.repo.UpdateVerificationToken(UpdateVerificationTokenInput{
		UserID:    userID.String(),
		Token:     token,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		slog.Error("failed to set verification token during registration",
			"user_id", userID.String(),
			"error", err)
	} else {
		go func() {
			s.sendVerificationEmail(input.Email, userID.String(), token)
		}()
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

	if !user.IsVerified {
		return "", ErrUserIsNotVerified
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password))
	if err != nil {
		return "", ErrInvalidCredentials
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userID": user.ID.String(),
		"email":  user.Email,
		"roleID": user.RoleID.String(),
		"exp":    time.Now().Add(24 * time.Hour).Unix(),
		"iat":    time.Now().Unix(),
	})

	tokenString, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	slog.Info("user logged in successfully", "email", input.Email)

	return tokenString, nil
}

// sendVerificationEmail sends a verification email using Resend
func (s *AuthService) sendVerificationEmail(email string, userID string, token string) {
	url := fmt.Sprintf("%s/api/v1/auth/verify?token=%s&user_id=%s", s.cfg.AppBaseURL, token, userID)

	body := fmt.Sprintf("Welcome to Pulsemon!\n\n"+
		"Please verify your email by clicking the link below:\n"+
		"%s\n\n"+
		"This link expires in 24 hours.\n\n"+
		"If you did not create an account, ignore this email.", url)

	params := &resend.SendEmailRequest{
		From:    s.cfg.ResendFromEmail,
		To:      []string{email},
		Subject: "Verify your Pulsemon account",
		Text:    body,
	}

	_, err := s.resend.Emails.Send(params)
	if err != nil {
		slog.Error("failed to send verification email",
			"email", email,
			"error", err)
	}
}

type VerifyEmailInput struct {
	Token  string
	UserID string
}

func (s *AuthService) VerifyEmail(ctx context.Context, input VerifyEmailInput) error {
	user, err := s.repo.VerifyUser(VerifyUserInput{Token: input.Token, UserID: input.UserID})
	if err != nil {
		return fmt.Errorf("failed to verify user token: %w", err)
	}
	if user == nil {
		return ErrInvalidOrExpiredToken
	}

	if user.IsVerified {
		return ErrAlreadyVerified
	}

	err = s.repo.SetVerified(SetVerifiedInput{UserID: user.ID.String()})
	if err != nil {
		return fmt.Errorf("failed to set user as verified: %w", err)
	}

	slog.Info("email verified successfully",
		"user_id", user.ID.String())

	return nil
}

type ResendVerificationInput struct {
	UserID string
}

func (s *AuthService) ResendVerification(ctx context.Context, input ResendVerificationInput) error {
	user, err := s.repo.FindUserByID(FindUserByIDInput{UserID: input.UserID})
	if err != nil {
		return fmt.Errorf("failed to find user: %w", err)
	}
	if user == nil {
		return errors.New("user not found")
	}

	if user.IsVerified {
		return ErrAlreadyVerified
	}

	token := uuid.New().String()
	expiresAt := time.Now().Add(24 * time.Hour)

	err = s.repo.UpdateVerificationToken(UpdateVerificationTokenInput{
		UserID:    input.UserID,
		Token:     token,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return fmt.Errorf("failed to update verification token: %w", err)
	}

	go s.sendVerificationEmail(user.Email, input.UserID, token)

	return nil
}
