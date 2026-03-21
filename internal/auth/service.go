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
	ErrEmailAlreadyExists         = errors.New("email already registered")
	ErrUsernameAlreadyExists      = errors.New("username already registered")
	ErrInvalidCredentials         = errors.New("invalid email or password")
	ErrInvalidOrExpiredToken      = errors.New("invalid or expired verification token")
	ErrAlreadyVerified            = errors.New("email already verified")
	ErrUserIsNotVerified          = errors.New("user is not verified")
	ErrInvalidOrExpiredResetToken = errors.New("invalid or expired reset token")
	ErrInvalidNewPassword         = errors.New("password must be at least 8 characters")
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

	token := fmt.Sprintf("verify_%s", uuid.New().String())
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
	if !strings.HasPrefix(input.Token, "verify_") {
		return ErrInvalidOrExpiredToken
	}

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
	Email string `json:"email"`
}

func (s *AuthService) ResendVerification(ctx context.Context, input ResendVerificationInput) error {
	user, err := s.repo.FindUserByEmail(FindUserByEmailInput{Email: input.Email})
	if err != nil {
		return fmt.Errorf("failed to find user: %w", err)
	}
	if user == nil {
		return errors.New("user not found")
	}

	if user.IsVerified {
		return ErrAlreadyVerified
	}

	token := fmt.Sprintf("verify_%s", uuid.New().String())
	expiresAt := time.Now().Add(24 * time.Hour)

	err = s.repo.UpdateVerificationToken(UpdateVerificationTokenInput{
		UserID:    user.ID.String(),
		Token:     token,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return fmt.Errorf("failed to update verification token: %w", err)
	}

	go s.sendVerificationEmail(user.Email, user.ID.String(), token)

	return nil
}

type ForgotPasswordInput struct {
	Email string
}

func (s *AuthService) ForgotPassword(ctx context.Context, input ForgotPasswordInput) error {
	user, err := s.repo.FindUserByEmail(FindUserByEmailInput{Email: input.Email})
	if err != nil {
		return fmt.Errorf("failed to check existing user: %w", err)
	}
	if user == nil {
		slog.Info("forgot password requested for unknown email", "email", input.Email)
		return nil
	}

	token := fmt.Sprintf("reset_%s", uuid.New().String())
	expiresAt := time.Now().Add(30 * time.Minute)

	err = s.repo.SetResetToken(SetResetTokenInput{
		UserID:    user.ID.String(),
		Token:     token,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return fmt.Errorf("failed to set reset token: %w", err)
	}

	go func() {
		s.sendPasswordResetEmail(user.Email, user.ID.String(), token)
	}()

	slog.Info("password reset email sent", "user_id", user.ID.String())

	return nil
}

func (s *AuthService) sendPasswordResetEmail(email string, userID string, token string) {
	url := fmt.Sprintf("%s/api/v1/auth/reset-password?token=%s&user_id=%s", s.cfg.AppBaseURL, token, userID)

	body := fmt.Sprintf("You requested a password reset for your Pulsemon account.\n\n"+
		"Click the link below to reset your password:\n"+
		"%s\n\n"+
		"This link expires in 30 minutes.\n\n"+
		"If you did not request a password reset, ignore this email.\n"+
		"Your password will not be changed.", url)

	params := &resend.SendEmailRequest{
		From:    s.cfg.ResendFromEmail,
		To:      []string{email},
		Subject: "Reset your Pulsemon password",
		Text:    body,
	}

	_, err := s.resend.Emails.Send(params)
	if err != nil {
		slog.Error("failed to send password reset email",
			"email", email,
			"error", err)
	}
}

type ResetPasswordInput struct {
	UserID      string
	Token       string
	NewPassword string
}

func (s *AuthService) ResetPassword(ctx context.Context, input ResetPasswordInput) error {
	if !strings.HasPrefix(input.Token, "reset_") {
		return ErrInvalidOrExpiredResetToken
	}

	if len(input.NewPassword) < 8 {
		return ErrInvalidNewPassword
	}

	user, err := s.repo.FindUserByResetToken(FindUserByResetTokenInput{
		UserID: input.UserID,
		Token:  input.Token,
	})
	if err != nil {
		return fmt.Errorf("failed to find user by reset token: %w", err)
	}
	if user == nil {
		return ErrInvalidOrExpiredResetToken
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	err = s.repo.UpdatePassword(UpdatePasswordInput{
		UserID:       input.UserID,
		PasswordHash: string(hashedPassword),
	})
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	slog.Info("password reset successfully", "user_id", input.UserID)

	return nil
}
