package roleseeder

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"Pulsemon/pkg/roles"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Seeder struct {
	db *gorm.DB
}

func NewSeeder(db *gorm.DB) *Seeder {
	return &Seeder{
		db: db,
	}
}

type SeedInput struct {
	Name string
}

type roleRow struct {
	ID        uuid.UUID
	Name      string
	CreatedAt time.Time
}

func (roleRow) TableName() string {
	return "roles"
}

func (s *Seeder) Seed(ctx context.Context) (*roles.RoleRegistry, error) {
	seeds := []SeedInput{
		{Name: "user"},
		{Name: "admin"},
	}

	registry := &roles.RoleRegistry{}

	for _, seed := range seeds {
		var r roleRow
		err := s.db.WithContext(ctx).Where("name = ?", seed.Name).First(&r).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				r = roleRow{
					ID:        uuid.New(),
					Name:      seed.Name,
					CreatedAt: time.Now(),
				}
				if createErr := s.db.WithContext(ctx).Create(&r).Error; createErr != nil {
					return nil, createErr
				}
				slog.Info("role seeded successfully", "role", seed.Name)
			} else {
				return nil, err
			}
		} else {
			slog.Info("role already exists, skipping", "role", seed.Name)
		}

		if seed.Name == "user" {
			registry.UserRoleID = r.ID
		}
		if seed.Name == "admin" {
			registry.AdminRoleID = r.ID
		}
	}

	// Backfill existing users with no role_id
	result := s.db.WithContext(ctx).Exec("UPDATE users SET role_id = ? WHERE role_id IS NULL", registry.UserRoleID)
	if result.Error != nil {
		return nil, result.Error
	}
	slog.Info("backfilled users with default role", "role_id", registry.UserRoleID.String(), "count", result.RowsAffected)

	return registry, nil
}
