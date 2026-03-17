package dashboard

import (
	"context"
	"errors"
)

var (
	ErrServiceNotFound = errors.New("service not found")
)

type DashboardService struct {
	repo *DashboardRepository
}

func NewDashboardService(repo *DashboardRepository) *DashboardService {
	return &DashboardService{repo: repo}
}

type GetDashboardInput struct {
	ServiceID string
	UserID    string
	Params    PaginationParams
}

func (s *DashboardService) GetDashboard(ctx context.Context, input GetDashboardInput) (*DashboardResponse, error) {
	service, err := s.repo.FindServiceByIDAndUser(FindServiceInput{
		ServiceID: input.ServiceID,
		UserID:    input.UserID,
	})
	if err != nil {
		return nil, err
	}
	if service == nil {
		return nil, ErrServiceNotFound
	}

	results, err := s.repo.FindRecentProbeResults(FindProbeResultsInput{
		ServiceID: input.ServiceID,
		Params:    input.Params,
	})
	if err != nil {
		return nil, err
	}

	response := ToDashboardResponse(*service, results, input.Params.Limit)
	return &response, nil
}

type GetServiceAlertsInput struct {
	ServiceID string
	UserID    string
	Params    PaginationParams
}

func (s *DashboardService) GetServiceAlerts(ctx context.Context, input GetServiceAlertsInput) (*ServiceAlertsResponse, error) {
	service, err := s.repo.FindServiceByIDAndUser(FindServiceInput{
		ServiceID: input.ServiceID,
		UserID:    input.UserID,
	})
	if err != nil {
		return nil, err
	}
	if service == nil {
		return nil, ErrServiceNotFound
	}

	alertsList, err := s.repo.FindAlertsByService(FindAlertsInput{
		ServiceID: input.ServiceID,
		UserID:    input.UserID,
		Params:    input.Params,
	})
	if err != nil {
		return nil, err
	}

	response := ToServiceAlertsResponse(input.ServiceID, alertsList, input.Params.Limit)
	return &response, nil
}
