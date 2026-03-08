package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// --- Enums ---

type Interval string

const (
	Interval30s Interval = "30s"
	Interval1m  Interval = "1m"
	Interval5m  Interval = "5m"
	Interval10m Interval = "10m"
	Interval30m Interval = "30m"
)

type ServiceStatus string

const (
	StatusUp      ServiceStatus = "up"
	StatusDown    ServiceStatus = "down"
	StatusUnknown ServiceStatus = "unknown"
)

type AlertType string

const (
	AlertFailureStreak AlertType = "failure_streak"
	AlertSLABreach     AlertType = "sla_breach"
	AlertSSLExpiry     AlertType = "ssl_expiry"
	AlertRecovery      AlertType = "recovery"
)

// --- Models ---

type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	Email        string    `gorm:"type:varchar(255);uniqueIndex;not null"`
	PasswordHash string    `gorm:"type:varchar(255);not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Service struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID         uuid.UUID `gorm:"type:uuid;not null;index"`
	Name           string    `gorm:"type:varchar(255);not null"`
	URL            string    `gorm:"type:varchar(2048);not null"`
	Interval       Interval  `gorm:"type:varchar(10);not null"`
	TimeoutSeconds int       `gorm:"not null"`
	ExpectedStatus int       `gorm:"not null"`
	SLATarget      float64   `gorm:"not null"`
	IsActive       bool      `gorm:"default:true"`

	// Live state
	CurrentStatus ServiceStatus `gorm:"type:varchar(10);default:'unknown'"`
	LastCheckedAt *time.Time
	FailureStreak int `gorm:"default:0"`

	// Pre-calculated latency
	AvgLatencyMs float64 `gorm:"default:0"`
	P95LatencyMs float64 `gorm:"default:0"`
	P99LatencyMs float64 `gorm:"default:0"`

	// SLA tracking
	SLAPercentage    float64 `gorm:"default:100"`
	TotalChecks      int     `gorm:"default:0"`
	SuccessfulChecks int     `gorm:"default:0"`

	// SSL (HTTPS only — nullable)
	SSLCertExpiry    *time.Time
	SSLCertIssuer    *string
	SSLCertValid     *bool
	SSLDaysRemaining *int

	CreatedAt time.Time
	UpdatedAt time.Time
}

type ProbeResult struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	ServiceID    uuid.UUID `gorm:"type:uuid;not null;index"`
	UserID       uuid.UUID `gorm:"type:uuid;not null;index"`
	StatusCode   int       `gorm:"not null"`
	LatencyMs    float64   `gorm:"not null"`
	IsSuccess    bool      `gorm:"not null"`
	ErrorMessage *string
	CheckedAt    time.Time `gorm:"not null;index"`
	CertExpiry   *time.Time
	CertValid    *bool
}

type Alert struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	ServiceID uuid.UUID `gorm:"type:uuid;not null;index"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index"`
	AlertType AlertType `gorm:"type:varchar(20);not null"`
	Message   string    `gorm:"type:text;not null"`
	SentAt    time.Time `gorm:"not null;index"`
}

// --- Hooks ---

// BeforeCreate assigns a UUID to any model that embeds this hook pattern.
// We call uuid.New() explicitly per model to keep things transparent.

func (u *User) BeforeCreate(tx *gorm.DB) error {
	u.ID = uuid.New()
	return nil
}

func (s *Service) BeforeCreate(tx *gorm.DB) error {
	s.ID = uuid.New()
	return nil
}

func (p *ProbeResult) BeforeCreate(tx *gorm.DB) error {
	p.ID = uuid.New()
	return nil
}

func (a *Alert) BeforeCreate(tx *gorm.DB) error {
	a.ID = uuid.New()
	return nil
}
