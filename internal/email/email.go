package email

import (
	"fmt"
	"time"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

// EmailService handles sending emails via SendGrid
type EmailService struct {
	apiKey       string
	supportEmail string
}

// NewEmailService creates a new email service instance
func NewEmailService(apiKey, supportEmail string) *EmailService {
	if supportEmail == "" {
		supportEmail = "support@israeldefensestore.com"
	}
	return &EmailService{
		apiKey:       apiKey,
		supportEmail: supportEmail,
	}
}

// SendSupportEscalationEmail sends an email to support with conversation summary
func (es *EmailService) SendSupportEscalationEmail(customerEmail, summary, fullConversation string) error {
	if es.apiKey == "" {
		return fmt.Errorf("SendGrid API key not configured")
	}

	from := mail.NewEmail("IDS Chat System", "noreply@israeldefensestore.com")
	to := mail.NewEmail("Support Team", es.supportEmail)
	cc := mail.NewEmail("Customer", customerEmail)

	subject := "Customer Support Request - Chat Escalation"

	body := fmt.Sprintf(`A customer has requested support escalation from the chat system.

Customer Email: %s
Timestamp: %s

Conversation Summary:
%s

Full Conversation:
%s`, customerEmail, time.Now().Format(time.RFC3339), summary, fullConversation)

	message := mail.NewSingleEmail(from, subject, to, body, body)

	// Add CC recipient using Personalizations
	if len(message.Personalizations) > 0 {
		message.Personalizations[0].AddCCs(cc)
	}

	client := sendgrid.NewSendClient(es.apiKey)
	response, err := client.Send(message)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	if response.StatusCode >= 400 {
		return fmt.Errorf("SendGrid API error: status %d, body: %s", response.StatusCode, response.Body)
	}

	return nil
}
