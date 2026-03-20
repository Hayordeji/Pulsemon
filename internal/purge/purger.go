package purge

import (
	"context"
	"log/slog"
	"time"

	"Pulsemon/pkg/models"

	"gorm.io/gorm"
)

// Purger deletes probe results older than the configured retention period.
type Purger struct {
	db            *gorm.DB
	retentionDays int
}

// NewPurger creates a Purger with a 30-day retention window.
func NewPurger(db *gorm.DB) *Purger {
	return &Purger{
		db:            db,
		retentionDays: 30,
	}
}

// Start runs the purge immediately once, then repeats every 24 hours.
// It exits when ctx is cancelled.
func (p *Purger) Start(ctx context.Context) {
	p.purge(ctx)

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.purge(ctx)
		case <-ctx.Done():
			slog.Info("purger stopped")
			return
		}
	}
}

// purge deletes probe results older than the retention window.
func (p *Purger) purge(ctx context.Context) {
	cutoff := time.Now().AddDate(0, 0, -p.retentionDays)

	result := p.db.WithContext(ctx).
		Where("checked_at < ?", cutoff).
		Delete(&models.ProbeResult{})

	if result.Error != nil {
		slog.Error("purge job failed",
			"error", result.Error)
		return
	}

	slog.Info("purge job completed",
		"deleted_rows", result.RowsAffected,
		"cutoff_date", cutoff.Format(time.DateOnly))
}
