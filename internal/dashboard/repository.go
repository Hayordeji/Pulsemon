package dashboard

import (
	"errors"

	"Pulsemon/pkg/models"

	"gorm.io/gorm"
)

// DashboardRepository provides read-only database queries for the dashboard.
type DashboardRepository struct {
	db *gorm.DB
}

// NewDashboardRepository creates a new DashboardRepository.
func NewDashboardRepository(db *gorm.DB) *DashboardRepository {
	return &DashboardRepository{db: db}
}

// ---------------------------------------------------------------------------
// Input structs (one per method — no multi-parameter methods)
// ---------------------------------------------------------------------------

// FindServiceInput groups parameters for FindServiceByIDAndUser.
type FindServiceInput struct {
	ServiceID string
	UserID    string
}

// FindProbeResultsInput groups parameters for FindRecentProbeResults.
type FindProbeResultsInput struct {
	ServiceID string
	Params    PaginationParams
}

// FindAlertsInput groups parameters for FindAlertsByService.
type FindAlertsInput struct {
	ServiceID string
	UserID    string
	Params    PaginationParams
}

// ---------------------------------------------------------------------------
// Query methods
// ---------------------------------------------------------------------------

// FindServiceByIDAndUser returns the service matching the given ID and user.
// Returns (nil, nil) when the service does not exist or belongs to another user.
func (r *DashboardRepository) FindServiceByIDAndUser(input FindServiceInput) (*models.Service, error) {
	var service models.Service
	err := r.db.
		Where("id = ? AND user_id = ?", input.ServiceID, input.UserID).
		First(&service).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &service, nil
}

// FindRecentProbeResults returns probe results for a service, paginated by
// checked_at descending. It fetches Limit+1 rows so the caller can determine
// whether more pages exist.
func (r *DashboardRepository) FindRecentProbeResults(input FindProbeResultsInput) ([]models.ProbeResult, error) {
	var results []models.ProbeResult
	q := r.db.
		Where("service_id = ?", input.ServiceID).
		Order("checked_at DESC").
		Limit(input.Params.Limit + 1)

	if input.Params.Cursor != nil {
		q = q.Where("checked_at < ?", *input.Params.Cursor)
	}

	if err := q.Find(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

// FindAlertsByService returns alerts for a service and user, paginated by
// sent_at descending. It fetches Limit+1 rows so the caller can determine
// whether more pages exist.
func (r *DashboardRepository) FindAlertsByService(input FindAlertsInput) ([]models.Alert, error) {
	var alertsList []models.Alert
	q := r.db.
		Where("service_id = ? AND user_id = ?", input.ServiceID, input.UserID).
		Order("sent_at DESC").
		Limit(input.Params.Limit + 1)

	if input.Params.Cursor != nil {
		q = q.Where("sent_at < ?", *input.Params.Cursor)
	}

	if err := q.Find(&alertsList).Error; err != nil {
		return nil, err
	}
	return alertsList, nil
}
