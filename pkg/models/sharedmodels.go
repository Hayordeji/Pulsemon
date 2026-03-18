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
