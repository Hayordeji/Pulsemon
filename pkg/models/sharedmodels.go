package models

type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type ServiceStructResponse struct {
	Message string
	Data    interface{}
	Error   string
	Success bool
}

type AlertType string

const (
	AlertTypeUnknown       AlertType = ""
	AlertTypeRecovery      AlertType = "recovery"
	AlertTypeFailureStreak AlertType = "failure_streak"
	AlertTypeSLA_Breach    AlertType = "sla_breach"
	AlertTypeSSL_Expiry    AlertType = "ssl_expiry"
)
