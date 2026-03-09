package email

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ids/internal/models"
)

const (
	roleUser      = "User"
	roleAssistant = "Assistant"
	acsAPIVersion = "2023-03-31"
)

// EmailService handles sending emails via Azure Communication Services
type EmailService struct {
	endpoint     string
	accessKey    string
	supportEmail string
}

// NewEmailService creates a new email service instance from an ACS connection string
func NewEmailService(connectionString, supportEmail string) *EmailService {
	if supportEmail == "" {
		supportEmail = "support@israeldefensestore.com"
	}

	endpoint, accessKey := parseACSConnectionString(connectionString)

	return &EmailService{
		endpoint:     endpoint,
		accessKey:    accessKey,
		supportEmail: supportEmail,
	}
}

// parseACSConnectionString extracts endpoint and accesskey from a connection string
// Format: "endpoint=https://xxx.communication.azure.com/;accesskey=base64key"
func parseACSConnectionString(connStr string) (string, string) {
	var endpoint, accessKey string
	for _, part := range strings.Split(connStr, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(strings.ToLower(part), "endpoint=") {
			endpoint = strings.TrimRight(strings.TrimPrefix(part, part[:len("endpoint=")]), "/")
		} else if strings.HasPrefix(strings.ToLower(part), "accesskey=") {
			accessKey = part[len("accesskey="):]
		}
	}
	return endpoint, accessKey
}

// acsEmailRequest represents the ACS Email send request body
type acsEmailRequest struct {
	SenderAddress string          `json:"senderAddress"`
	Recipients    acsRecipients   `json:"recipients"`
	Content       acsEmailContent `json:"content"`
}

type acsRecipients struct {
	To []acsEmailAddress `json:"to"`
	CC []acsEmailAddress `json:"cc,omitempty"`
}

type acsEmailAddress struct {
	Address     string `json:"address"`
	DisplayName string `json:"displayName,omitempty"`
}

type acsEmailContent struct {
	Subject   string `json:"subject"`
	PlainText string `json:"plainText,omitempty"`
	HTML      string `json:"html,omitempty"`
}

// sendEmail sends an email via Azure Communication Services REST API
func (es *EmailService) sendEmail(req acsEmailRequest) error {
	if es.endpoint == "" || es.accessKey == "" {
		return fmt.Errorf("ACS connection string not configured")
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal email request: %w", err)
	}

	pathAndQuery := "/emails:send?api-version=" + acsAPIVersion
	requestURL := es.endpoint + pathAndQuery

	httpReq, err := http.NewRequest("POST", requestURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Parse host from endpoint
	parsedURL, err := url.Parse(es.endpoint)
	if err != nil {
		return fmt.Errorf("failed to parse endpoint URL: %w", err)
	}

	// HMAC-SHA256 authentication
	dateStr := time.Now().UTC().Format(http.TimeFormat)
	contentHash := computeContentHash(body)
	signature := es.computeSignature("POST", pathAndQuery, dateStr, parsedURL.Host, contentHash)

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-ms-date", dateStr)
	httpReq.Header.Set("x-ms-content-sha256", contentHash)
	httpReq.Header.Set("Authorization", fmt.Sprintf(
		"HMAC-SHA256 SignedHeaders=x-ms-date;host;x-ms-content-sha256&Signature=%s", signature,
	))

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send email request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ACS Email API error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func computeContentHash(content []byte) string {
	hash := sha256.Sum256(content)
	return base64.StdEncoding.EncodeToString(hash[:])
}

func (es *EmailService) computeSignature(method, pathAndQuery, date, host, contentHash string) string {
	stringToSign := fmt.Sprintf("%s\n%s\n%s;%s;%s", method, pathAndQuery, date, host, contentHash)
	decodedKey, _ := base64.StdEncoding.DecodeString(es.accessKey)
	mac := hmac.New(sha256.New, decodedKey)
	mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// SendSupportEscalationEmail sends an email to support with conversation summary
func (es *EmailService) SendSupportEscalationEmail(customerEmail, summary, fullConversation string) (string, error) {
	subject := "🎫 Support Escalation - Chat Request"
	timestamp := time.Now().Format("January 2, 2006 at 3:04 PM MST")

	plainText := generatePlainTextEmail(customerEmail, timestamp, summary, fullConversation)
	htmlContent := generateHTMLEmail(customerEmail, timestamp, summary, fullConversation)

	req := acsEmailRequest{
		SenderAddress: es.supportEmail,
		Recipients: acsRecipients{
			To: []acsEmailAddress{{Address: es.supportEmail, DisplayName: "Support Team"}},
			CC: []acsEmailAddress{{Address: customerEmail, DisplayName: "Customer"}},
		},
		Content: acsEmailContent{
			Subject:   subject,
			PlainText: plainText,
			HTML:      htmlContent,
		},
	}

	if err := es.sendEmail(req); err != nil {
		return htmlContent, err
	}

	return htmlContent, nil
}

// generatePlainTextEmail creates the plain text version of the email
func generatePlainTextEmail(customerEmail, timestamp, summary, fullConversation string) string {
	return fmt.Sprintf(`SUPPORT ESCALATION REQUEST
============================

A customer has requested support escalation from the chat system.

CUSTOMER INFORMATION
--------------------
Email: %s
Submitted: %s

AI-GENERATED SUMMARY
--------------------
%s

FULL CONVERSATION
-----------------
%s

---
This email was automatically generated by the IDS Chat System.
Please respond to the customer at their email address above.
`, customerEmail, timestamp, summary, fullConversation)
}

// generateHTMLEmail creates a styled HTML email for better readability
func generateHTMLEmail(customerEmail, timestamp, summary, fullConversation string) string {
	// Escape HTML in user content to prevent XSS
	escapedEmail := html.EscapeString(customerEmail)
	escapedSummary := html.EscapeString(summary)

	// Format the conversation with proper styling
	formattedConversation := formatConversationHTML(fullConversation)

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Support Escalation Request</title>
</head>
<body style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background-color: #f5f5f5; line-height: 1.6;">
    <table role="presentation" style="width: 100%%; max-width: 700px; margin: 0 auto; background-color: #ffffff; border-collapse: collapse;">
        <!-- Header -->
        <tr>
            <td style="background: linear-gradient(135deg, #1a365d 0%%, #2c5282 100%%); padding: 30px 40px; text-align: center;">
                <h1 style="margin: 0; color: #ffffff; font-size: 24px; font-weight: 600;">
                    🎫 Support Escalation Request
                </h1>
                <p style="margin: 10px 0 0 0; color: #a0aec0; font-size: 14px;">
                    A customer needs assistance from the support team
                </p>
            </td>
        </tr>

        <!-- Priority Banner -->
        <tr>
            <td style="background-color: #fef3c7; padding: 15px 40px; border-left: 4px solid #f59e0b;">
                <table role="presentation" style="width: 100%%;">
                    <tr>
                        <td style="width: 30px; vertical-align: top;">
                            <span style="font-size: 20px;">⚡</span>
                        </td>
                        <td>
                            <strong style="color: #92400e;">Action Required:</strong>
                            <span style="color: #78350f;"> Please respond to the customer inquiry below.</span>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>

        <!-- Customer Information Card -->
        <tr>
            <td style="padding: 30px 40px 20px 40px;">
                <table role="presentation" style="width: 100%%; background-color: #f8fafc; border-radius: 8px; border: 1px solid #e2e8f0;">
                    <tr>
                        <td style="padding: 20px;">
                            <h2 style="margin: 0 0 15px 0; color: #1e293b; font-size: 16px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.5px;">
                                📋 Customer Information
                            </h2>
                            <table role="presentation" style="width: 100%%;">
                                <tr>
                                    <td style="padding: 8px 0; color: #64748b; width: 120px;">Email:</td>
                                    <td style="padding: 8px 0;">
                                        <a href="mailto:%s" style="color: #2563eb; text-decoration: none; font-weight: 500;">%s</a>
                                    </td>
                                </tr>
                                <tr>
                                    <td style="padding: 8px 0; color: #64748b;">Submitted:</td>
                                    <td style="padding: 8px 0; color: #334155;">%s</td>
                                </tr>
                            </table>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>

        <!-- AI Summary Section -->
        <tr>
            <td style="padding: 0 40px 20px 40px;">
                <table role="presentation" style="width: 100%%; background-color: #eff6ff; border-radius: 8px; border: 1px solid #bfdbfe;">
                    <tr>
                        <td style="padding: 20px;">
                            <h2 style="margin: 0 0 15px 0; color: #1e40af; font-size: 16px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.5px;">
                                🤖 AI-Generated Summary
                            </h2>
                            <div style="color: #1e3a5f; font-size: 15px; white-space: pre-wrap; line-height: 1.7;">%s</div>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>

        <!-- Conversation Section -->
        <tr>
            <td style="padding: 0 40px 30px 40px;">
                <h2 style="margin: 0 0 15px 0; color: #1e293b; font-size: 16px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.5px;">
                    💬 Full Conversation
                </h2>
                <div style="background-color: #fafafa; border-radius: 8px; border: 1px solid #e5e7eb; padding: 20px; max-height: 500px; overflow-y: auto;">
                    %s
                </div>
            </td>
        </tr>

        <!-- Footer -->
        <tr>
            <td style="background-color: #f8fafc; padding: 25px 40px; border-top: 1px solid #e2e8f0; text-align: center;">
                <p style="margin: 0 0 10px 0; color: #64748b; font-size: 13px;">
                    This email was automatically generated by the <strong>IDS Chat System</strong>
                </p>
                <p style="margin: 0; color: #94a3b8; font-size: 12px;">
                    Israel Defense Store • Customer Support Portal
                </p>
            </td>
        </tr>
    </table>
</body>
</html>`, escapedEmail, escapedEmail, timestamp, escapedSummary, formattedConversation)
}

const (
	embeddingsNotRun     = "Not run"
	embeddingsNotRunHTML = `<span style="color: #6b7280;">❌ Not run</span>`
)

// SendWeeklyAnalyticsEmail sends the weekly analytics report to the given recipients
func (es *EmailService) SendWeeklyAnalyticsEmail(summary *models.AnalyticsSummary, recipients []string) error {
	subject := fmt.Sprintf("📊 IDS Weekly Analytics Report — %s to %s",
		summary.StartDate.Format("Jan 2"),
		summary.EndDate.Format("Jan 2, 2006"),
	)

	plainText := generateWeeklyReportPlainText(summary)
	htmlContent := generateWeeklyReportHTML(summary)

	toAddresses := make([]acsEmailAddress, len(recipients))
	for i, r := range recipients {
		toAddresses[i] = acsEmailAddress{Address: r}
	}

	req := acsEmailRequest{
		SenderAddress: es.supportEmail,
		Recipients: acsRecipients{
			To: toAddresses,
		},
		Content: acsEmailContent{
			Subject:   subject,
			PlainText: plainText,
			HTML:      htmlContent,
		},
	}

	return es.sendEmail(req)
}

// generateWeeklyReportPlainText creates the plain text fallback for the weekly report email
func generateWeeklyReportPlainText(s *models.AnalyticsSummary) string {
	chatCost := float64(s.OpenAITokensUsed) * 0.0000003
	summarizationCost := float64(s.SupportSummaryTokens) * 0.0000003
	queryEmbeddingCost := float64(s.QueryEmbeddings) * 500 * 0.00000002
	totalCost := chatCost + summarizationCost + queryEmbeddingCost

	productEmbeddings := embeddingsNotRun
	if s.ProductEmbeddingsRan {
		productEmbeddings = fmt.Sprintf("Ran (%d products)", s.ProductEmbeddingsCount)
	}
	emailEmbeddings := embeddingsNotRun
	if s.EmailEmbeddingsRan {
		emailEmbeddings = fmt.Sprintf("Ran (%d emails, %d threads)", s.EmailEmbeddingsCount, s.ThreadEmbeddingsCount)
	}

	return fmt.Sprintf(`IDS WEEKLY ANALYTICS REPORT
============================

Period: %s - %s

USAGE METRICS
-------------
Conversations:      %d
Product Suggestions: %d
Emails in Database: %d
Email Threads:      %d
Support Escalations: %d

BILLING BREAKDOWN (Estimated)
------------------------------
OpenAI Chat Completions:
  Calls:  %d
  Tokens: %s
  Cost:   ~$%.4f

Support Summarizations:
  Calls:  %d
  Tokens: %s
  Cost:   ~$%.4f

Query Embeddings:   %d queries  (~$%.4f)
Emails Sent:        %d sent

Total Est. OpenAI Cost: $%.4f

EMBEDDINGS STATUS
-----------------
Product: %s
Email:   %s
Total in DB: %d products, %d emails

---
This report was automatically generated by the IDS Analytics System.
`,
		s.StartDate.Format("January 2, 2006"),
		s.EndDate.Format("January 2, 2006"),
		s.TotalConversations,
		s.ProductSuggestions,
		s.TotalEmails,
		s.EmailThreads,
		s.SupportEscalations,
		s.OpenAICalls,
		formatTokenCountEmail(s.OpenAITokensUsed),
		chatCost,
		s.SupportSummarizations,
		formatTokenCountEmail(s.SupportSummaryTokens),
		summarizationCost,
		s.QueryEmbeddings,
		queryEmbeddingCost,
		s.SendGridEmailsSent,
		totalCost,
		productEmbeddings,
		emailEmbeddings,
		s.TotalProductEmbeddings,
		s.TotalEmailEmbeddings,
	)
}

// generateWeeklyReportHTML creates the styled HTML email for the weekly analytics report
func generateWeeklyReportHTML(s *models.AnalyticsSummary) string {
	chatCost := float64(s.OpenAITokensUsed) * 0.0000003
	summarizationCost := float64(s.SupportSummaryTokens) * 0.0000003
	queryEmbeddingCost := float64(s.QueryEmbeddings) * 500 * 0.00000002
	totalCost := chatCost + summarizationCost + queryEmbeddingCost

	statusEmoji := "✅"
	statusText := "Active week"
	statusColor := "#d1fae5"
	statusBorder := "#6ee7b7"
	statusTextColor := "#064e3b"
	if s.TotalConversations == 0 {
		statusEmoji = "⚠️"
		statusText = "No activity this week"
		statusColor = "#fef3c7"
		statusBorder = "#fcd34d"
		statusTextColor = "#78350f"
	}

	productEmbeddingsHTML := embeddingsNotRunHTML
	if s.ProductEmbeddingsRan {
		productEmbeddingsHTML = fmt.Sprintf(`<span style="color: #059669;">✅ Ran (%d products)</span>`, s.ProductEmbeddingsCount)
	}
	emailEmbeddingsHTML := embeddingsNotRunHTML
	if s.EmailEmbeddingsRan {
		emailEmbeddingsHTML = fmt.Sprintf(`<span style="color: #059669;">✅ Ran (%d emails, %d threads)</span>`, s.EmailEmbeddingsCount, s.ThreadEmbeddingsCount)
	}

	avgProducts := 0.0
	if s.TotalConversations > 0 {
		avgProducts = float64(s.ProductSuggestions) / float64(s.TotalConversations)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>IDS Weekly Analytics Report</title>
</head>
<body style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background-color: #f5f5f5; line-height: 1.6;">
    <table role="presentation" style="width: 100%%; max-width: 700px; margin: 0 auto; background-color: #ffffff; border-collapse: collapse;">

        <!-- Header -->
        <tr>
            <td style="background: linear-gradient(135deg, #1a365d 0%%, #2c5282 100%%); padding: 30px 40px; text-align: center;">
                <h1 style="margin: 0; color: #ffffff; font-size: 24px; font-weight: 600;">
                    📊 IDS Weekly Analytics Report
                </h1>
                <p style="margin: 10px 0 0 0; color: #a0aec0; font-size: 14px;">
                    %s &ndash; %s
                </p>
            </td>
        </tr>

        <!-- Status Banner -->
        <tr>
            <td style="background-color: %s; padding: 15px 40px; border-left: 4px solid %s;">
                <span style="color: %s; font-weight: 600;">%s %s</span>
            </td>
        </tr>

        <!-- Quick Stats Row -->
        <tr>
            <td style="padding: 30px 40px 0 40px;">
                <table role="presentation" style="width: 100%%; border-collapse: collapse;">
                    <tr>
                        <td style="width: 25%%; padding: 0 8px 0 0; text-align: center;">
                            <table role="presentation" style="width: 100%%; background-color: #eff6ff; border-radius: 8px; border: 1px solid #bfdbfe;">
                                <tr><td style="padding: 16px 12px; text-align: center;">
                                    <div style="font-size: 28px; font-weight: 700; color: #1e40af;">%d</div>
                                    <div style="font-size: 11px; color: #3b82f6; text-transform: uppercase; letter-spacing: 0.5px; margin-top: 4px;">Conversations</div>
                                </td></tr>
                            </table>
                        </td>
                        <td style="width: 25%%; padding: 0 8px; text-align: center;">
                            <table role="presentation" style="width: 100%%; background-color: #f0fdf4; border-radius: 8px; border: 1px solid #bbf7d0;">
                                <tr><td style="padding: 16px 12px; text-align: center;">
                                    <div style="font-size: 28px; font-weight: 700; color: #15803d;">%d</div>
                                    <div style="font-size: 11px; color: #16a34a; text-transform: uppercase; letter-spacing: 0.5px; margin-top: 4px;">Products Suggested</div>
                                </td></tr>
                            </table>
                        </td>
                        <td style="width: 25%%; padding: 0 8px; text-align: center;">
                            <table role="presentation" style="width: 100%%; background-color: #fff7ed; border-radius: 8px; border: 1px solid #fed7aa;">
                                <tr><td style="padding: 16px 12px; text-align: center;">
                                    <div style="font-size: 28px; font-weight: 700; color: #c2410c;">%d</div>
                                    <div style="font-size: 11px; color: #ea580c; text-transform: uppercase; letter-spacing: 0.5px; margin-top: 4px;">Escalations</div>
                                </td></tr>
                            </table>
                        </td>
                        <td style="width: 25%%; padding: 0 0 0 8px; text-align: center;">
                            <table role="presentation" style="width: 100%%; background-color: #fdf4ff; border-radius: 8px; border: 1px solid #e9d5ff;">
                                <tr><td style="padding: 16px 12px; text-align: center;">
                                    <div style="font-size: 28px; font-weight: 700; color: #7e22ce;">$%.3f</div>
                                    <div style="font-size: 11px; color: #9333ea; text-transform: uppercase; letter-spacing: 0.5px; margin-top: 4px;">AI Cost Est.</div>
                                </td></tr>
                            </table>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>

        <!-- Usage Metrics Section -->
        <tr>
            <td style="padding: 25px 40px 0 40px;">
                <table role="presentation" style="width: 100%%; background-color: #eff6ff; border-radius: 8px; border: 1px solid #bfdbfe;">
                    <tr>
                        <td style="padding: 20px;">
                            <h2 style="margin: 0 0 15px 0; color: #1e40af; font-size: 16px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.5px;">
                                📊 Usage Metrics
                            </h2>
                            <table role="presentation" style="width: 100%%;">
                                <tr>
                                    <td style="padding: 7px 0; color: #374151; width: 55%%;">💬 Conversations</td>
                                    <td style="padding: 7px 0; font-weight: 600; color: #111827; text-align: right;">%d</td>
                                </tr>
                                <tr style="background-color: #dbeafe; border-radius: 4px;">
                                    <td style="padding: 7px 8px; color: #374151;">🛍️ Product Suggestions</td>
                                    <td style="padding: 7px 8px; font-weight: 600; color: #111827; text-align: right;">%d</td>
                                </tr>
                                <tr>
                                    <td style="padding: 7px 0; color: #374151;">📈 Avg Products / Conversation</td>
                                    <td style="padding: 7px 0; font-weight: 600; color: #111827; text-align: right;">%.1f</td>
                                </tr>
                                <tr style="background-color: #dbeafe; border-radius: 4px;">
                                    <td style="padding: 7px 8px; color: #374151;">📧 Emails in Database</td>
                                    <td style="padding: 7px 8px; font-weight: 600; color: #111827; text-align: right;">%d</td>
                                </tr>
                                <tr>
                                    <td style="padding: 7px 0; color: #374151;">🗂️ Email Threads</td>
                                    <td style="padding: 7px 0; font-weight: 600; color: #111827; text-align: right;">%d</td>
                                </tr>
                                <tr style="background-color: #dbeafe; border-radius: 4px;">
                                    <td style="padding: 7px 8px; color: #374151;">🆘 Support Escalations</td>
                                    <td style="padding: 7px 8px; font-weight: 600; color: #111827; text-align: right;">%d</td>
                                </tr>
                            </table>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>

        <!-- Billing Breakdown Section -->
        <tr>
            <td style="padding: 20px 40px 0 40px;">
                <table role="presentation" style="width: 100%%; background-color: #f8fafc; border-radius: 8px; border: 1px solid #e2e8f0;">
                    <tr>
                        <td style="padding: 20px;">
                            <h2 style="margin: 0 0 15px 0; color: #1e293b; font-size: 16px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.5px;">
                                💰 Billing Breakdown (Estimated)
                            </h2>

                            <!-- OpenAI Chat -->
                            <p style="margin: 0 0 6px 0; font-weight: 600; color: #334155; font-size: 13px;">🤖 OpenAI Chat Completions</p>
                            <table role="presentation" style="width: 100%%; background-color: #ffffff; border-radius: 6px; border: 1px solid #e2e8f0; margin-bottom: 14px;">
                                <tr>
                                    <td style="padding: 8px 12px; color: #64748b; font-size: 13px; width: 50%%;">Calls</td>
                                    <td style="padding: 8px 12px; font-weight: 600; color: #1e293b; text-align: right; font-size: 13px;">%d</td>
                                </tr>
                                <tr style="border-top: 1px solid #f1f5f9;">
                                    <td style="padding: 8px 12px; color: #64748b; font-size: 13px;">Tokens</td>
                                    <td style="padding: 8px 12px; font-weight: 600; color: #1e293b; text-align: right; font-size: 13px;">%s</td>
                                </tr>
                                <tr style="border-top: 1px solid #f1f5f9;">
                                    <td style="padding: 8px 12px; color: #64748b; font-size: 13px;">Est. Cost</td>
                                    <td style="padding: 8px 12px; font-weight: 600; color: #1e293b; text-align: right; font-size: 13px;">~$%.4f</td>
                                </tr>
                            </table>

                            <!-- Support Summarizations -->
                            <p style="margin: 0 0 6px 0; font-weight: 600; color: #334155; font-size: 13px;">🆘 Support Summarizations</p>
                            <table role="presentation" style="width: 100%%; background-color: #ffffff; border-radius: 6px; border: 1px solid #e2e8f0; margin-bottom: 14px;">
                                <tr>
                                    <td style="padding: 8px 12px; color: #64748b; font-size: 13px; width: 50%%;">Calls</td>
                                    <td style="padding: 8px 12px; font-weight: 600; color: #1e293b; text-align: right; font-size: 13px;">%d</td>
                                </tr>
                                <tr style="border-top: 1px solid #f1f5f9;">
                                    <td style="padding: 8px 12px; color: #64748b; font-size: 13px;">Tokens</td>
                                    <td style="padding: 8px 12px; font-weight: 600; color: #1e293b; text-align: right; font-size: 13px;">%s</td>
                                </tr>
                                <tr style="border-top: 1px solid #f1f5f9;">
                                    <td style="padding: 8px 12px; color: #64748b; font-size: 13px;">Est. Cost</td>
                                    <td style="padding: 8px 12px; font-weight: 600; color: #1e293b; text-align: right; font-size: 13px;">~$%.4f</td>
                                </tr>
                            </table>

                            <!-- Other costs -->
                            <table role="presentation" style="width: 100%%; background-color: #ffffff; border-radius: 6px; border: 1px solid #e2e8f0; margin-bottom: 14px;">
                                <tr>
                                    <td style="padding: 8px 12px; color: #64748b; font-size: 13px; width: 50%%;">🔍 Query Embeddings</td>
                                    <td style="padding: 8px 12px; font-weight: 600; color: #1e293b; text-align: right; font-size: 13px;">%d (~$%.4f)</td>
                                </tr>
                                <tr style="border-top: 1px solid #f1f5f9;">
                                    <td style="padding: 8px 12px; color: #64748b; font-size: 13px;">📤 Emails Sent</td>
                                    <td style="padding: 8px 12px; font-weight: 600; color: #1e293b; text-align: right; font-size: 13px;">%d sent</td>
                                </tr>
                            </table>

                            <!-- Total -->
                            <table role="presentation" style="width: 100%%; background-color: #1e293b; border-radius: 6px;">
                                <tr>
                                    <td style="padding: 12px 16px; color: #94a3b8; font-size: 13px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.5px;">Total Est. OpenAI Cost</td>
                                    <td style="padding: 12px 16px; color: #ffffff; font-size: 18px; font-weight: 700; text-align: right;">$%.4f</td>
                                </tr>
                            </table>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>

        <!-- Embeddings Status Section -->
        <tr>
            <td style="padding: 20px 40px 30px 40px;">
                <table role="presentation" style="width: 100%%; background-color: #f8fafc; border-radius: 8px; border: 1px solid #e2e8f0;">
                    <tr>
                        <td style="padding: 20px;">
                            <h2 style="margin: 0 0 15px 0; color: #1e293b; font-size: 16px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.5px;">
                                🧠 Embeddings Status
                            </h2>
                            <table role="presentation" style="width: 100%%;">
                                <tr>
                                    <td style="padding: 8px 0; color: #64748b; width: 140px;">Product Embeddings</td>
                                    <td style="padding: 8px 0;">%s</td>
                                </tr>
                                <tr>
                                    <td style="padding: 8px 0; color: #64748b;">Email Embeddings</td>
                                    <td style="padding: 8px 0;">%s</td>
                                </tr>
                                <tr>
                                    <td style="padding: 8px 0; color: #64748b;">Total in DB</td>
                                    <td style="padding: 8px 0; color: #334155;">%d products &bull; %d emails</td>
                                </tr>
                            </table>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>

        <!-- Footer -->
        <tr>
            <td style="background-color: #f8fafc; padding: 25px 40px; border-top: 1px solid #e2e8f0; text-align: center;">
                <p style="margin: 0 0 10px 0; color: #64748b; font-size: 13px;">
                    This report was automatically generated by the <strong>IDS Analytics System</strong>
                </p>
                <p style="margin: 0; color: #94a3b8; font-size: 12px;">
                    Israel Defense Store &bull; Weekly Analytics Digest
                </p>
            </td>
        </tr>

    </table>
</body>
</html>`,
		// Header dates
		s.StartDate.Format("Jan 2"),
		s.EndDate.Format("Jan 2, 2006"),
		// Status banner
		statusColor, statusBorder, statusTextColor, statusEmoji, statusText,
		// Quick stat cards
		s.TotalConversations,
		s.ProductSuggestions,
		s.SupportEscalations,
		totalCost,
		// Usage metrics table
		s.TotalConversations,
		s.ProductSuggestions,
		avgProducts,
		s.TotalEmails,
		s.EmailThreads,
		s.SupportEscalations,
		// Billing: OpenAI chat
		s.OpenAICalls,
		formatTokenCountEmail(s.OpenAITokensUsed),
		chatCost,
		// Billing: summarizations
		s.SupportSummarizations,
		formatTokenCountEmail(s.SupportSummaryTokens),
		summarizationCost,
		// Billing: other
		s.QueryEmbeddings, queryEmbeddingCost,
		s.SendGridEmailsSent,
		// Total
		totalCost,
		// Embeddings
		productEmbeddingsHTML,
		emailEmbeddingsHTML,
		s.TotalProductEmbeddings,
		s.TotalEmailEmbeddings,
	)
}

func formatTokenCountEmail(tokens int) string {
	if tokens >= 1000000 {
		return fmt.Sprintf("%.2fM", float64(tokens)/1000000)
	} else if tokens >= 1000 {
		return fmt.Sprintf("%.1fK", float64(tokens)/1000)
	}
	return fmt.Sprintf("%d", tokens)
}

// formatConversationHTML formats the conversation with styled message bubbles
func formatConversationHTML(conversation string) string {
	var result strings.Builder
	lines := strings.Split(conversation, "\n")

	var currentRole string
	var currentMessage strings.Builder
	messageCount := 0

	flushMessage := func() {
		if currentMessage.Len() == 0 {
			return
		}
		messageCount++
		msg := strings.TrimSpace(currentMessage.String())
		if msg == "" {
			currentMessage.Reset()
			return
		}

		escapedMsg := html.EscapeString(msg)
		// Convert newlines to <br> for HTML
		escapedMsg = strings.ReplaceAll(escapedMsg, "\n", "<br>")

		if currentRole == roleUser {
			fmt.Fprintf(&result, `
                <div style="margin-bottom: 15px;">
                    <div style="display: flex; align-items: center; margin-bottom: 5px;">
                        <span style="background-color: #3b82f6; color: white; width: 28px; height: 28px; border-radius: 50%%; display: inline-flex; align-items: center; justify-content: center; font-size: 12px; font-weight: 600; margin-right: 8px;">C</span>
                        <span style="font-weight: 600; color: #1e40af; font-size: 13px;">Customer</span>
                        <span style="color: #94a3b8; font-size: 11px; margin-left: 10px;">#%d</span>
                    </div>
                    <div style="background-color: #dbeafe; border-radius: 12px; padding: 12px 16px; margin-left: 36px; color: #1e3a8a; font-size: 14px; line-height: 1.5;">
                        %s
                    </div>
                </div>`, messageCount, escapedMsg)
		} else {
			fmt.Fprintf(&result, `
                <div style="margin-bottom: 15px;">
                    <div style="display: flex; align-items: center; margin-bottom: 5px;">
                        <span style="background-color: #10b981; color: white; width: 28px; height: 28px; border-radius: 50%%; display: inline-flex; align-items: center; justify-content: center; font-size: 12px; font-weight: 600; margin-right: 8px;">A</span>
                        <span style="font-weight: 600; color: #047857; font-size: 13px;">AI Assistant</span>
                        <span style="color: #94a3b8; font-size: 11px; margin-left: 10px;">#%d</span>
                    </div>
                    <div style="background-color: #d1fae5; border-radius: 12px; padding: 12px 16px; margin-left: 36px; color: #064e3b; font-size: 14px; line-height: 1.5;">
                        %s
                    </div>
                </div>`, messageCount, escapedMsg)
		}
		currentMessage.Reset()
	}

	for _, line := range lines {
		// Skip header lines
		if strings.HasPrefix(line, "Full Conversation:") || strings.HasPrefix(line, "===") {
			continue
		}

		// Check for message headers like "[Message 1] User:"
		if strings.HasPrefix(line, "[Message") {
			flushMessage()
			if strings.Contains(line, roleUser+":") {
				currentRole = roleUser
			} else if strings.Contains(line, roleAssistant+":") {
				currentRole = roleAssistant
			}
			continue
		}

		// Accumulate message content
		if currentRole != "" {
			if currentMessage.Len() > 0 {
				currentMessage.WriteString("\n")
			}
			currentMessage.WriteString(line)
		}
	}

	// Flush the last message
	flushMessage()

	if result.Len() == 0 {
		return `<p style="color: #64748b; font-style: italic;">No conversation messages found.</p>`
	}

	return result.String()
}
