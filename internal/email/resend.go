package email

import (
	"fmt"
	"log"
	"time"

	"github.com/resendlabs/resend-go"

	"dawhub/internal/config"
	"dawhub/pkg/common"
)

const (
	sendTimeout = 10 * time.Second
)

type ResendService struct {
	client *resend.Client
	from   string
}

func NewResendService(cfg config.ResendConfig) *ResendService {
	client := resend.NewClient(cfg.APIKey)

	return &ResendService{
		client: client,
		from:   cfg.FromEmail,
	}
}

func (s *ResendService) SendEmail(to string, subject string, htmlContent string) error {
	if to == "" || subject == "" || htmlContent == "" {
		return common.ErrInvalidInput
	}

	// Create the email request
	params := &resend.SendEmailRequest{
		From:    s.from,
		To:      []string{to},
		Subject: subject,
		Html:    htmlContent,
	}

	log.Printf("[DEBUG] Sending email to %s with subject: %s", to, subject)

	// Send the email using the Resend client
	_, err := s.client.Emails.Send(params) // Pass only params
	if err != nil {
		log.Printf("[ERROR] Failed to send email: %v", err)
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Printf("[INFO] Email sent successfully to %s", to)
	return nil
}

func (s *ResendService) SendBetaSignupEmail(email string) error {
	htmlContent := `
		<p>Dear User,</p>
		<p>Thank you for signing up for the DawHub Beta program. We are excited to have you on board!</p>
		<p>As a beta user, you will have the opportunity to explore our platform and provide valuable feedback that will help us improve your experience.</p>
		<p>Stay tuned for updates and further instructions on how to access the platform.</p>
		<p>If you have any questions, feel free to reach out to our support team.</p>
		<p>Best regards,</p>
		<p>The DawHub Team</p>
	`

	return s.SendEmail(email, "Welcome to DawHub Beta!", htmlContent)
}

func (s *ResendService) SendPasswordResetEmail(email, resetToken string) error {
	htmlContent := fmt.Sprintf(`
		<h1>Reset Your Password</h1>
		<p>Click the link below to reset your password:</p>
		<a href="https://dawhub.com/reset-password?token=%s">Reset Password</a>
		<p>If you didn't request this, please ignore this email.</p>
	`, resetToken)

	return s.SendEmail(email, "Password Reset Request", htmlContent)
}
