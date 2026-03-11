package scheduler

import (
	"context"
	"log"
	"sync"
	"time"

	"Pulsemon/internal/services"
	"Pulsemon/pkg/models"

	"gorm.io/gorm"
)

// ProbeJob carries everything a worker needs to execute a single probe.
// Defined here because the Scheduler owns the contract between itself and
// the Worker Pool.
type ProbeJob struct {
	ServiceID      string
	UserID         string
	URL            string
	TimeoutSeconds int
	ExpectedStatus int
}

// Scheduler loads active services, spawns per-service tickers, and reacts
// to lifecycle events (create / delete) from the Service Manager.
type Scheduler struct {
	db        *gorm.DB
	jobs      chan ProbeJob
	events    chan services.ServiceEvent
	tickers   map[string]*time.Ticker
	stopChans map[string]chan struct{}
	mu        sync.Mutex
}

// NewScheduler creates a Scheduler. The jobs channel is created externally
// (in main.go) so the Worker Pool can share it.
func NewScheduler(db *gorm.DB, jobs chan ProbeJob) *Scheduler {
	return &Scheduler{
		db:        db,
		jobs:      jobs,
		events:    make(chan services.ServiceEvent, 100),
		tickers:   make(map[string]*time.Ticker),
		stopChans: make(map[string]chan struct{}),
	}
}

// Events returns the events channel. The Service Manager sends lifecycle
// events through this channel.
func (s *Scheduler) Events() chan services.ServiceEvent {
	return s.events
}

// Start loads all active services from the database, starts their tickers,
// and listens for lifecycle events until ctx is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	// Load all active services.
	var activeServices []models.Service
	if err := s.db.Where("is_active = true").Find(&activeServices).Error; err != nil {
		log.Printf("scheduler: failed to load active services: %v", err)
		return
	}

	log.Printf("scheduler: loaded %d active service(s)", len(activeServices))

	for i := range activeServices {
		s.addTicker(activeServices[i])
	}

	// Event loop — react to service lifecycle changes.
	for {
		select {
		case event := <-s.events:
			switch event.Type {
			case services.ServiceCreated:
				var svc models.Service
				if err := s.db.First(&svc, "id = ?", event.ServiceID).Error; err != nil {
					log.Printf("scheduler: failed to fetch service %s: %v", event.ServiceID, err)
					continue
				}
				s.addTicker(svc)
				log.Printf("scheduler: added ticker for service %s", event.ServiceID)

			case services.ServiceDeleted:
				s.removeTicker(event.ServiceID)
				log.Printf("scheduler: removed ticker for service %s", event.ServiceID)
			}

		case <-ctx.Done():
			s.mu.Lock()
			for id, stopCh := range s.stopChans {
				close(stopCh)
				delete(s.tickers, id)
				delete(s.stopChans, id)
			}
			s.mu.Unlock()
			log.Println("scheduler: stopped all tickers")
			return
		}
	}
}

// addTicker creates a per-service ticker goroutine that sends ProbeJobs
// at the configured interval.
func (s *Scheduler) addTicker(service models.Service) {
	duration := parseInterval(service.Interval)

	ticker := time.NewTicker(duration)
	stopCh := make(chan struct{})

	s.mu.Lock()
	s.tickers[service.ID.String()] = ticker
	s.stopChans[service.ID.String()] = stopCh
	s.mu.Unlock()

	go func() {
		for {
			select {
			case <-ticker.C:
				s.jobs <- ProbeJob{
					ServiceID:      service.ID.String(),
					UserID:         service.UserID.String(),
					URL:            service.URL,
					TimeoutSeconds: service.TimeoutSeconds,
					ExpectedStatus: service.ExpectedStatus,
				}
			case <-stopCh:
				ticker.Stop()
				return
			}
		}
	}()
}

// removeTicker signals the ticker goroutine to stop and cleans up map entries.
func (s *Scheduler) removeTicker(serviceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if stopCh, ok := s.stopChans[serviceID]; ok {
		close(stopCh)
		delete(s.tickers, serviceID)
		delete(s.stopChans, serviceID)
	}
}

// parseInterval converts a human-readable interval string to a time.Duration.
func parseInterval(interval string) time.Duration {
	switch interval {
	case "30s":
		return 30 * time.Second
	case "1m":
		return 1 * time.Minute
	case "5m":
		return 5 * time.Minute
	case "10m":
		return 10 * time.Minute
	case "30m":
		return 30 * time.Minute
	default:
		return 5 * time.Minute // safe fallback
	}
}
