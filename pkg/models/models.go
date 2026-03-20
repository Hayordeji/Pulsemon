package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User represents a registered user of the system.
type User struct {
	ID                 uuid.UUID  `gorm:"type:uuid;primaryKey;column:id"`
	Email              string     `gorm:"type:varchar(255);uniqueIndex;not null;column:email"`
	PasswordHash       string     `gorm:"type:varchar(255);not null;column:password_hash"`
	Username           string     `gorm:"type:varchar(255);not null;column:username"`
	RoleID             uuid.UUID  `gorm:"type:uuid;not null" json:"role_id"`
	CreatedAt          time.Time  `gorm:"not null;column:created_at"`
	UpdatedAt          time.Time  `gorm:"not null;column:updated_at"`
	IsVerified         bool       `gorm:"column:is_verified;not null;default:false" json:"is_verified"`
	VerificationToken  *string    `gorm:"column:verification_token" json:"-"`
	TokenExpiresAt     *time.Time `gorm:"column:token_expires_at" json:"-"`
	RefreshToken       *string    `gorm:"column:refresh_token" json:"-"`
	RefreshTokenExpiry *time.Time `gorm:"column:refresh_token_expiry" json:"-"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

// Service represents a monitored HTTP/HTTPS endpoint belonging to a user.
type Service struct {
	ID               uuid.UUID  `gorm:"type:uuid;primaryKey;column:id"`
	UserID           uuid.UUID  `gorm:"type:uuid;not null;column:user_id"`
	Name             string     `gorm:"type:varchar(255);not null;column:name"`
	URL              string     `gorm:"type:varchar(2048);not null;column:url"`
	Interval         string     `gorm:"type:varchar(10);not null;column:interval"`
	TimeoutSeconds   int        `gorm:"not null;column:timeout_seconds"`
	ExpectedStatus   int        `gorm:"not null;column:expected_status"`
	SLATarget        float64    `gorm:"not null;column:sla_target"`
	IsActive         bool       `gorm:"not null;default:true;column:is_active"`
	CurrentStatus    string     `gorm:"type:varchar(10);not null;default:'unknown';column:current_status"`
	LastCheckedAt    *time.Time `gorm:"column:last_checked_at"`
	FailureStreak    int        `gorm:"not null;default:0;column:failure_streak"`
	AvgLatencyMs     float64    `gorm:"not null;default:0;column:avg_latency_ms"`
	P95LatencyMs     float64    `gorm:"not null;default:0;column:p95_latency_ms"`
	P99LatencyMs     float64    `gorm:"not null;default:0;column:p99_latency_ms"`
	SLAPercentage    float64    `gorm:"not null;default:100.0;column:sla_percentage"`
	TotalChecks      int        `gorm:"not null;default:0;column:total_checks"`
	SuccessfulChecks int        `gorm:"not null;default:0;column:successful_checks"`
	SSLCertExpiry    *time.Time `gorm:"column:ssl_cert_expiry"`
	SSLCertIssuer    *string    `gorm:"type:varchar(255);column:ssl_cert_issuer"`
	SSLCertValid     *bool      `gorm:"column:ssl_cert_valid"`
	SSLDaysRemaining *int       `gorm:"column:ssl_days_remaining"`
	CreatedAt        time.Time  `gorm:"not null;column:created_at"`
	UpdatedAt        time.Time  `gorm:"not null;column:updated_at"`

	// Associations
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
}

func (s *Service) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

// ProbeResult stores the outcome of a single health check probe.
type ProbeResult struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey;column:id"`
	ServiceID    uuid.UUID  `gorm:"type:uuid;not null;column:service_id"`
	UserID       uuid.UUID  `gorm:"type:uuid;not null;column:user_id"`
	StatusCode   int        `gorm:"not null;column:status_code"`
	LatencyMs    float64    `gorm:"not null;column:latency_ms"`
	IsSuccess    bool       `gorm:"not null;column:is_success"`
	ErrorMessage *string    `gorm:"type:text;column:error_message"`
	CheckedAt    time.Time  `gorm:"not null;column:checked_at"`
	CertExpiry   *time.Time `gorm:"column:cert_expiry"`
	CertValid    *bool      `gorm:"column:cert_valid"`

	// Associations
	Service Service `gorm:"foreignKey:ServiceID;constraint:OnDelete:CASCADE"`
	User    User    `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
}

func (p *ProbeResult) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// Alert records a notification sent to a user about a service event.
type Alert struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;column:id"`
	ServiceID uuid.UUID `gorm:"type:uuid;not null;column:service_id"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;column:user_id"`
	AlertType string    `gorm:"type:varchar(20);not null;column:alert_type"`
	Message   string    `gorm:"type:text;not null;column:message"`
	SentAt    time.Time `gorm:"not null;column:sent_at"`

	// Associations
	Service Service `gorm:"foreignKey:ServiceID;constraint:OnDelete:CASCADE"`
	User    User    `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
}

func (a *Alert) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

// Role represents a user role in the system.
type Role struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name      string    `gorm:"uniqueIndex;not null"`
	CreatedAt time.Time
}
