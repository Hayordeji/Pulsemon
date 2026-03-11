package services

import (
	"Pulsemon/pkg/models"
	"errors"

	"gorm.io/gorm"
)

// ServiceRepository handles all database operations for the Service entity.
type ServiceRepository struct {
	db *gorm.DB
}

// NewServiceRepository creates a new ServiceRepository.
func NewServiceRepository(db *gorm.DB) *ServiceRepository {
	return &ServiceRepository{db: db}
}

// CountActiveByUser returns the number of active services for the given user.
func (r *ServiceRepository) CountActiveByUser(userID string) (int64, error) {
	var count int64
	err := r.db.Model(&models.Service{}).
		Where("user_id = ? AND is_active = true", userID).
		Count(&count).Error
	return count, err
}

// Create inserts a new service record into the database.
func (r *ServiceRepository) Create(service *models.Service) error {
	return r.db.Create(service).Error
}

// FindAllByUser returns all services belonging to the given user, ordered by
// creation date descending.
func (r *ServiceRepository) FindAllByUser(userID string) ([]models.Service, error) {
	var services []models.Service
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&services).Error
	return services, err
}

// FindByIDAndUser returns the service matching both the service ID and user ID.
// Returns (nil, nil) when no matching record is found.
func (r *ServiceRepository) FindByIDAndUser(serviceID string, userID string) (*models.Service, error) {
	var service models.Service
	err := r.db.Where("id = ? AND user_id = ?", serviceID, userID).
		First(&service).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &service, nil
}

// Update persists all changes on the given service record.
func (r *ServiceRepository) Update(service *models.Service) error {
	return r.db.Save(service).Error
}

// Delete hard-deletes the service matching both the service ID and user ID.
func (r *ServiceRepository) Delete(serviceID string, userID string) error {
	return r.db.Where("id = ? AND user_id = ?", serviceID, userID).
		Delete(&models.Service{}).Error
}
