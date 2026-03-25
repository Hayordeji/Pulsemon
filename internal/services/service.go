package services

import (
	"errors"
	"fmt"
	"strings"

	"Pulsemon/pkg/models"

	"github.com/google/uuid"
)

// Sentinel errors used by the service layer. Handlers inspect these to choose
// the correct HTTP status code.
var (
	ErrServiceLimitReached = errors.New("service limit reached: maximum 3 active services allowed")
	ErrInvalidInterval     = errors.New("invalid interval: must be one of 30s, 1m, 5m, 10m, 30m")
	ErrInvalidURL          = errors.New("invalid URL: must start with http:// or https://")
	ErrServiceNotFound     = errors.New("service not found")
)

// validIntervals is the set of accepted probe intervals.
var validIntervals = map[string]bool{
	"30s": true,
	"1m":  true,
	"5m":  true,
	"10m": true,
	"30m": true,
}

// CreateServiceInput carries the fields required to create a new service.
type CreateServiceInput struct {
	Name           string  `json:"name"`
	URL            string  `json:"url"`
	Interval       string  `json:"interval"`
	TimeoutSeconds int     `json:"timeout_seconds"`
	ExpectedStatus int     `json:"expected_status"`
	SLATarget      float64 `json:"sla_target"`
}

// UpdateServiceInput carries the fields that may be updated on an existing
// service. URL is intentionally omitted — it is immutable after creation.
type UpdateServiceInput struct {
	Name           string  `json:"name"`
	Interval       string  `json:"interval"`
	TimeoutSeconds int     `json:"timeout_seconds"`
	ExpectedStatus int     `json:"expected_status"`
	SLATarget      float64 `json:"sla_target"`
}

// ServiceService encapsulates business logic for service management.
type ServiceService struct {
	repo   *ServiceRepository
	events chan ServiceEvent
}

// NewServiceService creates a new ServiceService.
func NewServiceService(repo *ServiceRepository, events chan ServiceEvent) *ServiceService {
	return &ServiceService{repo: repo, events: events}
}

// CreateService validates the input, enforces the per-user service limit, and
// persists a new service.
func (s *ServiceService) CreateService(userID string, input CreateServiceInput) (*CreateServiceResponse, error) {
	// Enforce per-user active service limit.
	count, err := s.repo.CountActiveByUser(userID)
	if err != nil {
		return nil, err
	}
	if count >= 3 {
		return nil, ErrServiceLimitReached
	}

	// Validate interval.
	if !validIntervals[input.Interval] {
		return nil, ErrInvalidInterval
	}

	// Validate URL scheme.
	if !strings.HasPrefix(input.URL, "http://") && !strings.HasPrefix(input.URL, "https://") {
		return nil, ErrInvalidURL
	}

	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, errors.New("invalid user ID")
	}

	service := &models.Service{
		ID:             uuid.New(),
		UserID:         parsedUserID,
		Name:           input.Name,
		URL:            input.URL,
		Interval:       input.Interval,
		TimeoutSeconds: input.TimeoutSeconds,
		ExpectedStatus: input.ExpectedStatus,
		SLATarget:      input.SLATarget,
		CurrentStatus:  "unknown",
		IsActive:       true,
	}

	if err := s.repo.Create(service); err != nil {
		// Check for unique constraint violation
		if strings.Contains(err.Error(), "23505") ||
			strings.Contains(err.Error(), "uq_services_user_url") {
			return nil, errors.New("a service with this URL already exists")
		}
		return nil, fmt.Errorf("failed to create service: %w", err)
	}

	// Notify the scheduler that a new service was created.
	s.events <- ServiceEvent{
		Type:      ServiceCreated,
		ServiceID: service.ID.String(),
		Interval:  service.Interval,
	}

	//MAP NEWLY CREATED SERVICE
	mappedService := ToCreateServiceResponse(*service)

	return &mappedService, nil
}

// GetServices returns all services belonging to the given user.
func (s *ServiceService) GetServices(userID string) ([]ServiceSummaryResponse, error) {
	services, err := s.repo.FindAllByUser(userID)
	if err != nil {
		return nil, err
	}

	summaries := make([]ServiceSummaryResponse, len(services))
	for i, s := range services {
		summaries[i] = ToServiceSummaryResponse(s)
	}

	return summaries, nil
}

// GetServiceByID returns a single service by ID, scoped to the given user.
func (s *ServiceService) GetServiceByID(serviceID string, userID string) (*models.Service, error) {
	service, err := s.repo.FindByIDAndUser(serviceID, userID)
	if err != nil {
		return nil, err
	}
	if service == nil {
		return nil, ErrServiceNotFound
	}
	return service, nil
}

// UpdateService applies the allowed field changes to an existing service.
// URL is never updated.
func (s *ServiceService) UpdateService(serviceID string, userID string, input UpdateServiceInput) error {
	service, err := s.repo.FindByIDAndUser(serviceID, userID)
	if err != nil {
		return err
	}
	if service == nil {
		return ErrServiceNotFound
	}

	// Validate interval if a new value is provided.
	if input.Interval != "" && !validIntervals[input.Interval] {
		return ErrInvalidInterval
	}

	// Apply allowed updates.
	if input.Name != "" {
		service.Name = input.Name
	}
	if input.Interval != "" {
		service.Interval = input.Interval
	}
	if input.TimeoutSeconds != 0 {
		service.TimeoutSeconds = input.TimeoutSeconds
	}
	if input.ExpectedStatus != 0 {
		service.ExpectedStatus = input.ExpectedStatus
	}
	if input.SLATarget != 0 {
		service.SLATarget = input.SLATarget
	}

	return s.repo.Update(service)
}

// DeleteService removes a service by ID, scoped to the given user.
func (s *ServiceService) DeleteService(serviceID string, userID string) error {
	service, err := s.repo.FindByIDAndUser(serviceID, userID)
	if err != nil {
		return err
	}
	if service == nil {
		return ErrServiceNotFound
	}

	if err := s.repo.Delete(serviceID, userID); err != nil {
		return err
	}

	// Notify the scheduler that a service was deleted.
	s.events <- ServiceEvent{
		Type:      ServiceDeleted,
		ServiceID: serviceID,
	}

	return nil
}
