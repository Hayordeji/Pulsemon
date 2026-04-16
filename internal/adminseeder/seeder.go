package adminseeder

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"Pulsemon/pkg/config"
	"Pulsemon/pkg/models"
	"Pulsemon/pkg/roles"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AdminSeeder struct {
	db       *gorm.DB
	cfg      config.Config
	registry *roles.RoleRegistry
}

func NewAdminSeeder(db *gorm.DB, cfg config.Config, registry *roles.RoleRegistry) *AdminSeeder {
	return &AdminSeeder{
		db:       db,
		cfg:      cfg,
		registry: registry,
	}
}

type SeedInput struct {
	Email    string
	Password string
	Username string
	RoleID   uuid.UUID
}

func (s *AdminSeeder) Seed(ctx context.Context) error {
	if s.cfg.AdminEmail == "" && s.cfg.AdminPassword == "" && s.cfg.AdminUsername == "" {
		slog.Info("admin seeder skipped — no credentials configured")
		return nil
	}

	var user models.User
	err := s.db.WithContext(ctx).Table("users").Where("email = ?", s.cfg.AdminEmail).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	} else {
		slog.Info("admin user already exists, skipping", "email", s.cfg.AdminEmail)
		return nil
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(s.cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash admin password: %w", err)
	}

	input := SeedInput{
		Email:    s.cfg.AdminEmail,
		Password: string(hashedPassword),
		Username: s.cfg.AdminUsername,
		RoleID:   s.registry.AdminRoleID,
	}

	newUser := map[string]interface{}{
		"id":            uuid.New(),
		"email":         input.Email,
		"password_hash": input.Password,
		"username":      input.Username,
		"role_id":       input.RoleID,
		"created_at":    time.Now(),
		"updated_at":    time.Now(),
	}

	if err := s.db.WithContext(ctx).Table("users").Create(newUser).Error; err != nil {
		return err
	}

	slog.Info("admin user seeded successfully", "email", s.cfg.AdminEmail, "username", s.cfg.AdminUsername)
	return nil
}
