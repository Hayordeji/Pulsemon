package contact

import (
	"net/http"
	"strings"

	"Pulsemon/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/resendlabs/resend-go"
)

// ContactHandler serves the public contact form endpoint.
type ContactHandler struct {
	client    *resend.Client
	fromEmail string
	toEmail   string
}

// ContactRequest is the JSON body for a contact form submission.
type ContactRequest struct {
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Message  string `json:"message"`
}

// NewContactHandler creates a ContactHandler.
func NewContactHandler(apiKey, fromEmail, toEmail string) *ContactHandler {
	return &ContactHandler{
		client:    resend.NewClient(apiKey),
		fromEmail: fromEmail,
		toEmail:   toEmail,
	}
}

// RegisterRoutes wires up the contact form route.
func (h *ContactHandler) RegisterRoutes(router gin.IRouter) {
	router.POST("/contact", h.Submit)
}

// Submit handles POST /contact.
func (h *ContactHandler) Submit(c *gin.Context) {
	var input ContactRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	input.FullName = strings.TrimSpace(input.FullName)
	input.Email = strings.TrimSpace(input.Email)
	input.Message = strings.TrimSpace(input.Message)

	if input.FullName == "" || input.Email == "" || input.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "full_name, email, and message are required"})
		return
	}

	if !strings.Contains(input.Email, "@") || !strings.Contains(input.Email, ".") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid email format"})
		return
	}

	htmlBody := strings.NewReplacer(
		"{{FullName}}", input.FullName,
		"{{Email}}", input.Email,
		"{{Message}}", input.Message,
	).Replace(`<h2>New message from your portfolio</h2>
<p><strong>Name:</strong> {{FullName}}</p>
<p><strong>Email:</strong> {{Email}}</p>
<p><strong>Message:</strong></p>
<p>{{Message}}</p>`)

	params := &resend.SendEmailRequest{
		From:    h.fromEmail,
		To:      []string{h.toEmail},
		Subject: "New Portfolio Message from " + input.FullName,
		Html:    htmlBody,
	}

	_, err := h.client.Emails.Send(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send message"})
		return
	}

	c.JSON(http.StatusOK, models.ApiResponse{
		Success: true,
		Message: "Message sent successfully",
	})
}
