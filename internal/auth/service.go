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
	ErrInvalidRefreshToken        = errors.New("invalid or expired refresh token")
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

type TokenPair struct {
	JWT          string
	RefreshToken string
}

func (s *AuthService) Login(ctx context.Context, input LoginInput) (*TokenPair, error) {
	user, err := s.repo.FindUserByEmail(FindUserByEmailInput{Email: input.Email})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}
	if user == nil {
		return nil, ErrInvalidCredentials
	}

	if !user.IsVerified {
		return nil, ErrUserIsNotVerified
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password))
	if err != nil {
		return nil, ErrInvalidCredentials
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
		return nil, fmt.Errorf("failed to sign token: %w", err)
	}

	refreshToken := uuid.New().String()
	refreshExpiry := time.Now().Add(7 * 24 * time.Hour)

	err = s.repo.SetRefreshToken(SetRefreshTokenInput{
		UserID: user.ID.String(),
		Token:  refreshToken,
		Expiry: refreshExpiry,
	})
	if err != nil {
		slog.Error("failed to store refresh token",
			"user_id", user.ID.String(),
			"error", err)
		refreshToken = ""
	}

	slog.Info("user logged in successfully", "email", input.Email)

	return &TokenPair{
		JWT:          tokenString,
		RefreshToken: refreshToken,
	}, nil
}

type RefreshInput struct {
	RefreshToken string
}

func (s *AuthService) Refresh(ctx context.Context, input RefreshInput) (*TokenPair, error) {
	user, err := s.repo.FindUserByRefreshToken(FindByRefreshTokenInput{
		Token: input.RefreshToken,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find user by refresh token: %w", err)
	}
	if user == nil {
		return nil, ErrInvalidRefreshToken
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userID": user.ID.String(),
		"email":  user.Email,
		"roleID": user.RoleID.String(),
		"exp":    time.Now().Add(24 * time.Hour).Unix(),
		"iat":    time.Now().Unix(),
	})

	newJWTToken, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign token: %w", err)
	}

	newRefreshToken := uuid.New().String()
	newExpiry := time.Now().Add(7 * 24 * time.Hour)

	err = s.repo.SetRefreshToken(SetRefreshTokenInput{
		UserID: user.ID.String(),
		Token:  newRefreshToken,
		Expiry: newExpiry,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to rotate refresh token: %w", err)
	}

	slog.Info("token refreshed successfully", "user_id", user.ID.String())

	return &TokenPair{
		JWT:          newJWTToken,
		RefreshToken: newRefreshToken,
	}, nil
}

// sendVerificationEmail sends a verification email using Resend
func (s *AuthService) sendVerificationEmail(email string, userID string, token string) {
	url := fmt.Sprintf("%s/api/v1/auth/verify?token=%s&user_id=%s", s.cfg.AppBaseURL, token, userID)

	// body := fmt.Sprintf("Welcome to Pulsemon!\n\n"+
	// 	"Please verify your email by clicking the link below:\n"+
	// 	"%s\n\n"+
	// 	"This link expires in 24 hours.\n\n"+
	// 	"If you did not create an account, ignore this email.", url)

	htmlBody := returnEmailVerificationHtmlBody(&email, &url)
	params := &resend.SendEmailRequest{
		From:    s.cfg.ResendFromEmail,
		To:      []string{email},
		Subject: "Verify your Pulsemon account",
		Html:    htmlBody,
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
		s.sendPasswordResetEmail(user.Username, user.Email, user.ID.String(), token)
	}()

	slog.Info("password reset email sent", "user_id", user.ID.String())

	return nil
}

func (s *AuthService) sendPasswordResetEmail(username string, email string, userID string, token string) {
	url := fmt.Sprintf("%s/api/v1/auth/reset-password?token=%s&user_id=%s", s.cfg.AppBaseURL, token, userID)

	// body := fmt.Sprintf("You requested a password reset for your Pulsemon account.\n\n"+
	// 	"Click the link below to reset your password:\n"+
	// 	"%s\n\n"+
	// 	"This link expires in 30 minutes.\n\n"+
	// 	"If you did not request a password reset, ignore this email.\n"+
	// 	"Your password will not be changed.", url)

	htmlBody := returnForgotPasswordHtmlBody(&username, &email, &url)

	params := &resend.SendEmailRequest{
		From:    s.cfg.ResendFromEmail,
		To:      []string{email},
		Subject: "Reset your Pulsemon password",
		Html:    htmlBody,
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

func returnEmailVerificationHtmlBody(email *string, url *string) string {
	body := `
					<!DOCTYPE html>
		<html lang="en" xmlns="http://www.w3.org/1999/xhtml">
		<head>
		<meta charset="UTF-8" />
		<meta name="viewport" content="width=device-width, initial-scale=1.0" />
		<meta http-equiv="X-UA-Compatible" content="IE=edge" />
		<title>Verify Your Pulsemon Account</title>
		<style>
			@media only screen and (max-width: 600px) {
			.email-wrapper  { padding: 16px !important; }
			.email-card     { border-radius: 12px !important; }
			.card-body      { padding: 24px 20px !important; }
			.banner-cell    { padding: 14px 20px !important; }
			.cta-btn        { width: 100% !important; text-align: center !important; display: block !important; }
			.footer-cell    { padding: 24px 0 0 0 !important; }
			}
		</style>
		</head>
		<body style="margin:0;padding:0;background-color:#0a0c12;-webkit-text-size-adjust:100%;moz-text-size-adjust:100%;">

		<!-- Outer wrapper -->
		<table role="presentation" width="100%" cellpadding="0" cellspacing="0"
				style="background-color:#0a0c12;min-height:100vh;">
			<tr>
			<td class="email-wrapper" align="center" style="padding:40px 16px;">

				<!-- Max-width container -->
				<table role="presentation" width="100%" cellpadding="0" cellspacing="0"
					style="max-width:580px;width:100%;">

				<!-- ─── HEADER ─── -->
				<tr>
					<td style="padding:0 0 28px 0;">
					<table role="presentation" width="100%" cellpadding="0" cellspacing="0">
						<tr>
						<td style="vertical-align:middle;">
							<table role="presentation" cellpadding="0" cellspacing="0">
							<tr>
								<td style="vertical-align:middle;padding-right:10px;">
								<svg width="20" height="20" viewBox="0 0 24 24" fill="none"
									xmlns="http://www.w3.org/2000/svg">
									<path d="M13 2L4.5 13.5H11L10 22L19.5 10H13L13 2Z"
										fill="#3b82f6" stroke="#3b82f6"
										stroke-width="1.5" stroke-linejoin="round"/>
								</svg>
								</td>
								<td style="vertical-align:middle;">
								<span style="color:#ffffff;font-size:17px;font-weight:700;
											font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',
											Helvetica,Arial,sans-serif;letter-spacing:-0.4px;">
									Pulsemon
								</span>
								</td>
							</tr>
							</table>
						</td>
						<td align="right" style="vertical-align:middle;">
							<span style="color:#4a4e6a;font-size:12px;
										font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',
										Helvetica,Arial,sans-serif;">
							Email Verification
							</span>
						</td>
						</tr>
					</table>
					</td>
				</tr>

				<!-- ─── MAIN CARD ─── -->
				<tr>
					<td class="email-card"
						style="background-color:#12151f;border-radius:16px;
							border:1px solid #1e2235;overflow:hidden;">

					<!-- Top accent banner -->
					<table role="presentation" width="100%" cellpadding="0" cellspacing="0">
						<tr>
						<td class="banner-cell"
							style="background-color:#1d2a42;border-bottom:1px solid #1e2235;
									padding:14px 28px;">
							<table role="presentation" cellpadding="0" cellspacing="0">
							<tr>
								<td style="vertical-align:middle;padding-right:10px;">
								<div style="width:8px;height:8px;border-radius:50%;
											background-color:#3b82f6;display:inline-block;">
								</div>
								</td>
								<td style="vertical-align:middle;">
								<span style="color:#93c5fd;font-size:12px;font-weight:600;
											letter-spacing:1px;text-transform:uppercase;
											font-family:-apple-system,BlinkMacSystemFont,
											'Segoe UI',Helvetica,Arial,sans-serif;">
									Confirm Your Email Address
								</span>
								</td>
							</tr>
							</table>
						</td>
						</tr>
					</table>

					<!-- Card content -->
					<table role="presentation" width="100%" cellpadding="0" cellspacing="0">
						<tr>
						<td class="card-body" style="padding:32px 28px 28px 28px;">

							<!-- Greeting -->
							<p style="margin:0 0 8px 0;color:#6b7280;font-size:11px;
									font-weight:600;letter-spacing:1.2px;
									text-transform:uppercase;
									font-family:-apple-system,BlinkMacSystemFont,
									'Segoe UI',Helvetica,Arial,sans-serif;">
							Welcome to Pulsemon
							</p>
							<p style="margin:0 0 20px 0;color:#ffffff;font-size:22px;
									font-weight:700;letter-spacing:-0.4px;line-height:1.3;
									font-family:-apple-system,BlinkMacSystemFont,
									'Segoe UI',Helvetica,Arial,sans-serif;">
							Verify your email address
							</p>

							<!-- Divider -->
							<table role="presentation" width="100%" cellpadding="0" cellspacing="0"
								style="margin-bottom:24px;">
							<tr>
								<td style="border-top:1px solid #1e2235;font-size:0;line-height:0;">
								&nbsp;
								</td>
							</tr>
							</table>

							<!-- Body text -->
							<p style="margin:0 0 8px 0;color:#d1d5db;font-size:14px;
									line-height:1.7;
									font-family:-apple-system,BlinkMacSystemFont,
									'Segoe UI',Helvetica,Arial,sans-serif;">
							Hi <strong style="color:#ffffff;">{{EMAIL}}</strong>,
							</p>
							<p style="margin:0 0 28px 0;color:#9ca3af;font-size:14px;
									line-height:1.7;
									font-family:-apple-system,BlinkMacSystemFont,
									'Segoe UI',Helvetica,Arial,sans-serif;">
							Thank you for creating a Pulsemon account. Click the button
							below to verify your email address and activate your account.
							This link expires in <strong style="color:#d1d5db;">24 hours.</strong>
							</p>

							<!-- CTA button -->
							<table role="presentation" cellpadding="0" cellspacing="0"
								style="margin-bottom:32px;">
							<tr>
								<td style="border-radius:8px;background-color:#3b82f6;">
								<a href="{{URL}}" class="cta-btn"
									style="display:inline-block;padding:13px 32px;
											color:#ffffff;font-size:14px;font-weight:600;
											text-decoration:none;letter-spacing:0.1px;
											font-family:-apple-system,BlinkMacSystemFont,
											'Segoe UI',Helvetica,Arial,sans-serif;">
									Verify Email Address &rarr;
								</a>
								</td>
							</tr>
							</table>

							<!-- Divider -->
							<table role="presentation" width="100%" cellpadding="0" cellspacing="0"
								style="margin-bottom:20px;">
							<tr>
								<td style="border-top:1px solid #1e2235;font-size:0;line-height:0;">
								&nbsp;
								</td>
							</tr>
							</table>

							<!-- Fallback link box -->
							<p style="margin:0 0 8px 0;color:#6b7280;font-size:11px;
									font-weight:600;letter-spacing:1.2px;
									text-transform:uppercase;
									font-family:-apple-system,BlinkMacSystemFont,
									'Segoe UI',Helvetica,Arial,sans-serif;">
							Button not working?
							</p>
							<table role="presentation" width="100%" cellpadding="0" cellspacing="0">
							<tr>
								<td style="background-color:#0a0c12;border-radius:10px;
										border:1px solid #1e2235;
										border-left:3px solid #3b82f6;
										padding:14px 18px;">
								<p style="margin:0 0 6px 0;color:#6b7280;font-size:11px;
											line-height:1.5;
											font-family:-apple-system,BlinkMacSystemFont,
											'Segoe UI',Helvetica,Arial,sans-serif;">
									Copy and paste this link into your browser:
								</p>
								<p style="margin:0;color:#93c5fd;font-size:12px;
											word-break:break-all;line-height:1.6;
											font-family:'Courier New',Courier,monospace;">
									{{URL}}
								</p>
								</td>
							</tr>
							</table>

						</td>
						</tr>
					</table>

					</td>
				</tr>

				<!-- ─── FOOTER ─── -->
				<tr>
					<td class="footer-cell" style="padding:28px 4px 0 4px;">

					<table role="presentation" width="100%" cellpadding="0" cellspacing="0"
							style="margin-bottom:20px;">
						<tr>
						<td style="border-top:1px solid #1a1d2e;font-size:0;line-height:0;">
							&nbsp;
						</td>
						</tr>
					</table>

					<p style="margin:0 0 8px 0;color:#4a4e6a;font-size:12px;line-height:1.6;
								font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',
								Helvetica,Arial,sans-serif;">
						If you did not create a Pulsemon account, you can safely ignore this email.
						Your email address will not be used without verification.
					</p>
					<p style="margin:0;color:#323650;font-size:11px;
								font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',
								Helvetica,Arial,sans-serif;">
						&copy; 2026 Pulsemon &nbsp;&middot;&nbsp; Service Reliability Monitor
					</p>

					</td>
				</tr>

				</table>
			</td>
			</tr>
		</table>

		</body>
		</html>
	`

	body = strings.ReplaceAll(body, "{{URL}}", *url)
	body = strings.ReplaceAll(body, "{{EMAIL}}", *email)

	return body
}

func returnForgotPasswordHtmlBody(username *string, email *string, url *string) string {
	body := `
				<!DOCTYPE html>
<html lang="en" xmlns="http://www.w3.org/1999/xhtml">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <meta http-equiv="X-UA-Compatible" content="IE=edge" />
  <title>Reset Your Pulsemon Password</title>
  <style>
    @media only screen and (max-width: 600px) {
      .email-wrapper  { padding: 16px !important; }
      .email-card     { border-radius: 12px !important; }
      .card-body      { padding: 24px 20px !important; }
      .banner-cell    { padding: 14px 20px !important; }
      .cta-btn        { width: 100% !important; text-align: center !important; display: block !important; }
      .footer-cell    { padding: 24px 0 0 0 !important; }
    }
  </style>
</head>
<body style="margin:0;padding:0;background-color:#0a0c12;-webkit-text-size-adjust:100%;moz-text-size-adjust:100%;">

  <!-- Outer wrapper -->
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0"
         style="background-color:#0a0c12;min-height:100vh;">
    <tr>
      <td class="email-wrapper" align="center" style="padding:40px 16px;">

        <!-- Max-width container -->
        <table role="presentation" width="100%" cellpadding="0" cellspacing="0"
               style="max-width:580px;width:100%;">

          <!-- ─── HEADER ─── -->
          <tr>
            <td style="padding:0 0 28px 0;">
              <table role="presentation" width="100%" cellpadding="0" cellspacing="0">
                <tr>
                  <td style="vertical-align:middle;">
                    <table role="presentation" cellpadding="0" cellspacing="0">
                      <tr>
                        <td style="vertical-align:middle;padding-right:10px;">
                          <svg width="20" height="20" viewBox="0 0 24 24" fill="none"
                               xmlns="http://www.w3.org/2000/svg">
                            <path d="M13 2L4.5 13.5H11L10 22L19.5 10H13L13 2Z"
                                  fill="#3b82f6" stroke="#3b82f6"
                                  stroke-width="1.5" stroke-linejoin="round"/>
                          </svg>
                        </td>
                        <td style="vertical-align:middle;">
                          <span style="color:#ffffff;font-size:17px;font-weight:700;
                                       font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',
                                       Helvetica,Arial,sans-serif;letter-spacing:-0.4px;">
                            Pulsemon
                          </span>
                        </td>
                      </tr>
                    </table>
                  </td>
                  <td align="right" style="vertical-align:middle;">
                    <span style="color:#4a4e6a;font-size:12px;
                                 font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',
                                 Helvetica,Arial,sans-serif;">
                      Password Reset
                    </span>
                  </td>
                </tr>
              </table>
            </td>
          </tr>

          <!-- ─── MAIN CARD ─── -->
          <tr>
            <td class="email-card"
                style="background-color:#12151f;border-radius:16px;
                       border:1px solid #1e2235;overflow:hidden;">

              <!-- Top accent banner -->
              <table role="presentation" width="100%" cellpadding="0" cellspacing="0">
                <tr>
                  <td class="banner-cell"
                      style="background-color:#1d2a42;border-bottom:1px solid #1e2235;
                             padding:14px 28px;">
                    <table role="presentation" cellpadding="0" cellspacing="0">
                      <tr>
                        <td style="vertical-align:middle;padding-right:10px;">
                          <div style="width:8px;height:8px;border-radius:50%;
                                      background-color:#3b82f6;display:inline-block;">
                          </div>
                        </td>
                        <td style="vertical-align:middle;">
                          <span style="color:#93c5fd;font-size:12px;font-weight:600;
                                       letter-spacing:1px;text-transform:uppercase;
                                       font-family:-apple-system,BlinkMacSystemFont,
                                       'Segoe UI',Helvetica,Arial,sans-serif;">
                            Password Reset Request
                          </span>
                        </td>
                      </tr>
                    </table>
                  </td>
                </tr>
              </table>

              <!-- Card content -->
              <table role="presentation" width="100%" cellpadding="0" cellspacing="0">
                <tr>
                  <td class="card-body" style="padding:32px 28px 28px 28px;">

                    <!-- Greeting -->
                    <p style="margin:0 0 8px 0;color:#6b7280;font-size:11px;
                               font-weight:600;letter-spacing:1.2px;
                               text-transform:uppercase;
                               font-family:-apple-system,BlinkMacSystemFont,
                               'Segoe UI',Helvetica,Arial,sans-serif;">
                      Account Security
                    </p>
                    <p style="margin:0 0 20px 0;color:#ffffff;font-size:22px;
                               font-weight:700;letter-spacing:-0.4px;line-height:1.3;
                               font-family:-apple-system,BlinkMacSystemFont,
                               'Segoe UI',Helvetica,Arial,sans-serif;">
                      Reset your password
                    </p>

                    <!-- Divider -->
                    <table role="presentation" width="100%" cellpadding="0" cellspacing="0"
                           style="margin-bottom:24px;">
                      <tr>
                        <td style="border-top:1px solid #1e2235;font-size:0;line-height:0;">
                          &nbsp;
                        </td>
                      </tr>
                    </table>

                    <!-- Body text -->
                    <p style="margin:0 0 8px 0;color:#d1d5db;font-size:14px;
                               line-height:1.7;
                               font-family:-apple-system,BlinkMacSystemFont,
                               'Segoe UI',Helvetica,Arial,sans-serif;">
                      Hi <strong style="color:#ffffff;">{{EMAIL}}</strong>,
                    </p>
                    <p style="margin:0 0 24px 0;color:#9ca3af;font-size:14px;
                               line-height:1.7;
                               font-family:-apple-system,BlinkMacSystemFont,
                               'Segoe UI',Helvetica,Arial,sans-serif;">
                      We received a request to reset the password for your Pulsemon account.
                      Click the button below to choose a new password.
                      This link expires in <strong style="color:#d1d5db;">30 minutes.</strong>
                    </p>

                    <!-- Expiry warning box -->
                    <table role="presentation" width="100%" cellpadding="0" cellspacing="0"
                           style="margin-bottom:28px;">
                      <tr>
                        <td style="background-color:#0a0c12;border-radius:10px;
                                   border:1px solid #1e2235;
                                   border-left:3px solid #3b82f6;
                                   padding:14px 18px;">
                          <p style="margin:0;color:#9ca3af;font-size:13px;
                                     line-height:1.6;
                                     font-family:-apple-system,BlinkMacSystemFont,
                                     'Segoe UI',Helvetica,Arial,sans-serif;">
                            This reset link will expire in
                            <strong style="color:#93c5fd;">30 minutes</strong>
                            from when this email was sent. If it expires, you can
                            request a new one from the login page.
                          </p>
                        </td>
                      </tr>
                    </table>

                    <!-- CTA button -->
                    <table role="presentation" cellpadding="0" cellspacing="0"
                           style="margin-bottom:32px;">
                      <tr>
                        <td style="border-radius:8px;background-color:#3b82f6;">
                          <a href="{{URL}}" class="cta-btn"
                             style="display:inline-block;padding:13px 32px;
                                    color:#ffffff;font-size:14px;font-weight:600;
                                    text-decoration:none;letter-spacing:0.1px;
                                    font-family:-apple-system,BlinkMacSystemFont,
                                    'Segoe UI',Helvetica,Arial,sans-serif;">
                            Reset Password &rarr;
                          </a>
                        </td>
                      </tr>
                    </table>

                    <!-- Divider -->
                    <table role="presentation" width="100%" cellpadding="0" cellspacing="0"
                           style="margin-bottom:20px;">
                      <tr>
                        <td style="border-top:1px solid #1e2235;font-size:0;line-height:0;">
                          &nbsp;
                        </td>
                      </tr>
                    </table>

                    <!-- Fallback link box -->
                    <p style="margin:0 0 8px 0;color:#6b7280;font-size:11px;
                               font-weight:600;letter-spacing:1.2px;
                               text-transform:uppercase;
                               font-family:-apple-system,BlinkMacSystemFont,
                               'Segoe UI',Helvetica,Arial,sans-serif;">
                      Button not working?
                    </p>
                    <table role="presentation" width="100%" cellpadding="0" cellspacing="0">
                      <tr>
                        <td style="background-color:#0a0c12;border-radius:10px;
                                   border:1px solid #1e2235;
                                   border-left:3px solid #3b82f6;
                                   padding:14px 18px;">
                          <p style="margin:0 0 6px 0;color:#6b7280;font-size:11px;
                                     line-height:1.5;
                                     font-family:-apple-system,BlinkMacSystemFont,
                                     'Segoe UI',Helvetica,Arial,sans-serif;">
                            Copy and paste this link into your browser:
                          </p>
                          <p style="margin:0;color:#93c5fd;font-size:12px;
                                     word-break:break-all;line-height:1.6;
                                     font-family:'Courier New',Courier,monospace;">
                            {{URL}}
                          </p>
                        </td>
                      </tr>
                    </table>

                  </td>
                </tr>
              </table>

            </td>
          </tr>

          <!-- ─── FOOTER ─── -->
          <tr>
            <td class="footer-cell" style="padding:28px 4px 0 4px;">

              <table role="presentation" width="100%" cellpadding="0" cellspacing="0"
                     style="margin-bottom:20px;">
                <tr>
                  <td style="border-top:1px solid #1a1d2e;font-size:0;line-height:0;">
                    &nbsp;
                  </td>
                </tr>
              </table>

              <p style="margin:0 0 8px 0;color:#4a4e6a;font-size:12px;line-height:1.6;
                         font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',
                         Helvetica,Arial,sans-serif;">
                If you did not request a password reset, you can safely ignore this email.
                Your password will not be changed unless you click the link above.
              </p>
              <p style="margin:0;color:#323650;font-size:11px;
                         font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',
                         Helvetica,Arial,sans-serif;">
                &copy; 2026 Pulsemon &nbsp;&middot;&nbsp; Service Reliability Monitor
              </p>

            </td>
          </tr>

        </table>
      </td>
    </tr>
  </table>

</body>
</html>
	`

	body = strings.ReplaceAll(body, "{{URL}}", *url)
	body = strings.ReplaceAll(body, "{{EMAIL}}", strings.ToUpper(*username))

	return body
}
