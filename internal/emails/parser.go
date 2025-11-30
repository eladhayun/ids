package emails

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ids/internal/models"
)

// ParseEMLFile parses a single EML file
func ParseEMLFile(filename string) (*models.Email, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open EML file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Printf("Warning: Error closing file: %v\n", err)
		}
	}()

	return parseEmailMessage(file)
}

// ParseMBOXFile parses an MBOX file and returns all emails
func ParseMBOXFile(filename string) ([]*models.Email, error) {
	var allEmails []*models.Email

	err := ParseMBOXFileStreaming(filename, 100, func(batch []*models.Email, progress MBOXProgress) error {
		allEmails = append(allEmails, batch...)
		fmt.Printf("[MBOX_PARSER] Processed batch: %d emails (total: %d, %.1f%%)\n",
			len(batch), progress.EmailsProcessed, progress.PercentComplete)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return allEmails, nil
}

// MBOXProgress tracks the progress of MBOX file parsing
type MBOXProgress struct {
	BytesProcessed   int64
	TotalBytes       int64
	EmailsProcessed  int
	PercentComplete  float64
	CurrentBatchSize int
}

// MBOXBatchCallback is called for each batch of emails processed
type MBOXBatchCallback func(batch []*models.Email, progress MBOXProgress) error

// ParseMBOXFileStreaming parses an MBOX file in batches with progress tracking
// This is memory-efficient for large MBOX files (70GB+)
func ParseMBOXFileStreaming(filename string, batchSize int, callback MBOXBatchCallback) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open MBOX file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Printf("Warning: Error closing file: %v\n", err)
		}
	}()

	// Get file size for progress tracking
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}
	totalBytes := fileInfo.Size()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024) // 10MB max token size

	var currentBatch []*models.Email
	var currentEmail bytes.Buffer
	var emailCount int
	var bytesProcessed int64

	for scanner.Scan() {
		line := scanner.Text()
		lineBytes := int64(len(line) + 1) // +1 for newline
		bytesProcessed += lineBytes

		// MBOX format: each email starts with "From " (with space)
		if strings.HasPrefix(line, "From ") && currentEmail.Len() > 0 {
			// Parse the accumulated email
			email, err := parseEmailMessage(&currentEmail)
			if err != nil {
				fmt.Printf("[MBOX_PARSER] Warning: Failed to parse email #%d: %v\n", emailCount+1, err)
			} else {
				currentBatch = append(currentBatch, email)
			}
			emailCount++

			// Process batch if it reaches the batch size
			if len(currentBatch) >= batchSize {
				progress := MBOXProgress{
					BytesProcessed:   bytesProcessed,
					TotalBytes:       totalBytes,
					EmailsProcessed:  emailCount,
					PercentComplete:  float64(bytesProcessed) / float64(totalBytes) * 100,
					CurrentBatchSize: len(currentBatch),
				}

				if err := callback(currentBatch, progress); err != nil {
					return fmt.Errorf("batch processing error at email %d: %w", emailCount, err)
				}

				// Clear batch for next iteration
				currentBatch = nil
			}

			// Reset buffer for next email
			currentEmail.Reset()
			continue // Skip the "From " line itself
		}

		// Accumulate email content
		currentEmail.WriteString(line)
		currentEmail.WriteString("\n")
	}

	// Parse the last email
	if currentEmail.Len() > 0 {
		email, err := parseEmailMessage(&currentEmail)
		if err != nil {
			fmt.Printf("[MBOX_PARSER] Warning: Failed to parse last email #%d: %v\n", emailCount+1, err)
		} else {
			currentBatch = append(currentBatch, email)
			emailCount++
		}
	}

	// Process remaining batch
	if len(currentBatch) > 0 {
		progress := MBOXProgress{
			BytesProcessed:   bytesProcessed,
			TotalBytes:       totalBytes,
			EmailsProcessed:  emailCount,
			PercentComplete:  100.0,
			CurrentBatchSize: len(currentBatch),
		}

		if err := callback(currentBatch, progress); err != nil {
			return fmt.Errorf("final batch processing error: %w", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading MBOX file: %w", err)
	}

	fmt.Printf("[MBOX_PARSER] ✅ Complete: Processed %d emails from %s (%.2f GB)\n",
		emailCount, filepath.Base(filename), float64(totalBytes)/(1024*1024*1024))

	return nil
}

// ParseDirectory recursively parses all EML files in a directory
func ParseDirectory(dirPath string) ([]*models.Email, error) {
	var emails []*models.Email

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Process EML files
		if strings.HasSuffix(strings.ToLower(path), ".eml") {
			email, err := ParseEMLFile(path)
			if err != nil {
				fmt.Printf("Warning: Failed to parse %s: %v\n", path, err)
				return nil // Continue processing other files
			}
			emails = append(emails, email)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return emails, nil
}

// parseEmailMessage parses an email message from a reader
func parseEmailMessage(r io.Reader) (*models.Email, error) {
	msg, err := mail.ReadMessage(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read email message: %w", err)
	}

	header := msg.Header

	// Extract basic headers
	email := &models.Email{
		MessageID: header.Get("Message-ID"),
		Subject:   decodeHeader(header.Get("Subject")),
		From:      header.Get("From"),
		To:        header.Get("To"),
	}

	// Parse date
	dateStr := header.Get("Date")
	if dateStr != "" {
		date, err := mail.ParseDate(dateStr)
		if err == nil {
			email.Date = date
		} else {
			email.Date = time.Now() // Fallback
		}
	} else {
		email.Date = time.Now()
	}

	// Extract threading information
	if inReplyTo := header.Get("In-Reply-To"); inReplyTo != "" {
		email.InReplyTo = &inReplyTo
	}
	if references := header.Get("References"); references != "" {
		email.References = &references
	}

	// Extract body
	body, err := extractBody(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to extract body: %w", err)
	}
	email.Body = body

	// Determine if this is from a customer (simple heuristic)
	// You can customize this based on your domain
	fromAddr := strings.ToLower(email.From)
	email.IsCustomer = !strings.Contains(fromAddr, "israeldefensestore.com") &&
		!strings.Contains(fromAddr, "support@") &&
		!strings.Contains(fromAddr, "info@")

	return email, nil
}

// extractBody extracts the body text from an email message
func extractBody(msg *mail.Message) (string, error) {
	contentType := msg.Header.Get("Content-Type")
	if contentType == "" {
		// Plain text email
		body, err := io.ReadAll(msg.Body)
		if err != nil {
			return "", err
		}
		return string(body), nil
	}

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		// Fallback: read as plain text
		body, err := io.ReadAll(msg.Body)
		if err != nil {
			return "", err
		}
		return string(body), nil
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		// Multipart message
		return extractMultipartBody(msg.Body, params["boundary"])
	}

	// Single part message
	return extractSinglePartBody(msg.Body, mediaType, msg.Header.Get("Content-Transfer-Encoding"))
}

// extractMultipartBody extracts text from multipart email
func extractMultipartBody(body io.Reader, boundary string) (string, error) {
	mr := multipart.NewReader(body, boundary)
	var textParts []string
	var htmlParts []string

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		partContentType := part.Header.Get("Content-Type")
		mediaType, _, _ := mime.ParseMediaType(partContentType)
		transferEncoding := part.Header.Get("Content-Transfer-Encoding")

		content, err := extractSinglePartBody(part, mediaType, transferEncoding)
		if err != nil {
			continue
		}

		if strings.HasPrefix(mediaType, "text/plain") {
			textParts = append(textParts, content)
		} else if strings.HasPrefix(mediaType, "text/html") {
			htmlParts = append(htmlParts, content)
		} else if strings.HasPrefix(mediaType, "multipart/") {
			// Nested multipart (rare but possible)
			_, params, _ := mime.ParseMediaType(partContentType)
			if nestedBoundary, ok := params["boundary"]; ok {
				nested, err := extractMultipartBody(part, nestedBoundary)
				if err == nil {
					textParts = append(textParts, nested)
				}
			}
		}
	}

	// Prefer plain text over HTML
	if len(textParts) > 0 {
		return strings.Join(textParts, "\n\n"), nil
	}

	// Fallback to HTML (basic cleanup)
	if len(htmlParts) > 0 {
		html := strings.Join(htmlParts, "\n\n")
		return cleanHTML(html), nil
	}

	return "", nil
}

// extractSinglePartBody extracts text from a single part
func extractSinglePartBody(body io.Reader, mediaType, transferEncoding string) (string, error) {
	reader := body

	// Handle transfer encoding
	switch strings.ToLower(transferEncoding) {
	case "quoted-printable":
		reader = quotedprintable.NewReader(body)
	case "base64":
		reader = base64.NewDecoder(base64.StdEncoding, body)
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// cleanHTML removes HTML tags (basic implementation)
func cleanHTML(html string) string {
	// Remove script and style tags with their contents
	html = removeTagsWithContent(html, "script")
	html = removeTagsWithContent(html, "style")

	// Replace common HTML entities
	html = strings.ReplaceAll(html, "&nbsp;", " ")
	html = strings.ReplaceAll(html, "&lt;", "<")
	html = strings.ReplaceAll(html, "&gt;", ">")
	html = strings.ReplaceAll(html, "&amp;", "&")
	html = strings.ReplaceAll(html, "&quot;", "\"")
	html = strings.ReplaceAll(html, "&#39;", "'")
	html = strings.ReplaceAll(html, "<br>", "\n")
	html = strings.ReplaceAll(html, "<br/>", "\n")
	html = strings.ReplaceAll(html, "<br />", "\n")
	html = strings.ReplaceAll(html, "</p>", "\n\n")
	html = strings.ReplaceAll(html, "</div>", "\n")

	// Remove all remaining HTML tags
	var result strings.Builder
	inTag := false
	for _, char := range html {
		if char == '<' {
			inTag = true
			continue
		}
		if char == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(char)
		}
	}

	// Clean up whitespace
	text := result.String()
	text = strings.TrimSpace(text)

	// Remove excessive newlines
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}

	return text
}

// removeTagsWithContent removes HTML tags and their content
func removeTagsWithContent(html, tag string) string {
	openTag := "<" + tag
	closeTag := "</" + tag + ">"

	for {
		start := strings.Index(strings.ToLower(html), strings.ToLower(openTag))
		if start == -1 {
			break
		}

		// Find the closing tag
		end := strings.Index(strings.ToLower(html[start:]), strings.ToLower(closeTag))
		if end == -1 {
			break
		}
		end += start + len(closeTag)

		// Remove the section
		html = html[:start] + html[end:]
	}

	return html
}

// decodeHeader decodes MIME encoded headers
func decodeHeader(header string) string {
	dec := new(mime.WordDecoder)
	decoded, err := dec.DecodeHeader(header)
	if err != nil {
		return header
	}
	return decoded
}

// GenerateThreadID generates a thread ID from email headers
func GenerateThreadID(email *models.Email) string {
	// Try to extract thread ID from References or In-Reply-To
	if email.References != nil && *email.References != "" {
		// Take the first Message-ID in References as the thread root
		refs := strings.Fields(*email.References)
		if len(refs) > 0 {
			return cleanMessageID(refs[0])
		}
	}

	if email.InReplyTo != nil && *email.InReplyTo != "" {
		return cleanMessageID(*email.InReplyTo)
	}

	// This is a new thread - use its own Message-ID
	return cleanMessageID(email.MessageID)
}

// cleanMessageID removes < and > from Message-IDs
func cleanMessageID(msgID string) string {
	msgID = strings.TrimPrefix(msgID, "<")
	msgID = strings.TrimSuffix(msgID, ">")
	return msgID
}
