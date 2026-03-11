package services

// ServiceEventType represents the kind of lifecycle event that occurred.
type ServiceEventType string

const (
	// ServiceCreated is emitted after a new service is persisted.
	ServiceCreated ServiceEventType = "created"
	// ServiceDeleted is emitted after a service is hard-deleted.
	ServiceDeleted ServiceEventType = "deleted"
)

// ServiceEvent carries lifecycle information from the Service Manager
// to the Scheduler so it can add or remove tickers dynamically.
type ServiceEvent struct {
	Type      ServiceEventType
	ServiceID string
	Interval  string // only relevant for ServiceCreated
}
