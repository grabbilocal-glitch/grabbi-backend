package utils

import (
	"fmt"
	"log"
	"net/smtp"
	"os"
	"strings"
)

type EmailConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

func GetEmailConfig() *EmailConfig {
	return &EmailConfig{
		Host:     os.Getenv("SMTP_HOST"),
		Port:     os.Getenv("SMTP_PORT"),
		Username: os.Getenv("SMTP_USERNAME"),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     os.Getenv("SMTP_FROM"),
	}
}

func SendEmail(to, subject, htmlBody string) error {
	config := GetEmailConfig()
	if config.Host == "" || config.Port == "" || config.From == "" {
		return fmt.Errorf("SMTP not configured")
	}

	headers := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n",
		config.From, to, subject)
	msg := []byte(headers + htmlBody)

	var auth smtp.Auth
	if config.Username != "" && config.Password != "" {
		auth = smtp.PlainAuth("", config.Username, config.Password, config.Host)
	}

	addr := config.Host + ":" + config.Port
	return smtp.SendMail(addr, auth, config.From, []string{to}, msg)
}

func SendWelcomeEmail(email, name string) {
	go func() {
		subject := "Welcome to Grabbi!"
		body := fmt.Sprintf(`<h2>Welcome to Grabbi, %s!</h2>
<p>Thank you for creating your account. You can now:</p>
<ul>
<li>Browse and order from local stores</li>
<li>Earn loyalty points on every order</li>
<li>Track your deliveries in real-time</li>
</ul>
<p>Happy shopping!</p>
<p>The Grabbi Team</p>`, strings.Split(name, " ")[0])
		if err := SendEmail(email, subject, body); err != nil {
			log.Printf("Failed to send welcome email to %s: %v", email, err)
		}
	}()
}

func SendOrderConfirmation(email, name, orderNumber string, total float64) {
	go func() {
		subject := fmt.Sprintf("Order Confirmed - %s", orderNumber)
		body := fmt.Sprintf(`<h2>Order Confirmed!</h2>
<p>Hi %s,</p>
<p>Your order <strong>%s</strong> has been placed successfully.</p>
<p>Order total: <strong>£%.2f</strong></p>
<p>We'll notify you when your order status changes.</p>
<p>The Grabbi Team</p>`, strings.Split(name, " ")[0], orderNumber, total)
		if err := SendEmail(email, subject, body); err != nil {
			log.Printf("Failed to send order confirmation to %s: %v", email, err)
		}
	}()
}

func SendOrderStatusUpdate(email, name, orderNumber, status string) {
	go func() {
		subject := fmt.Sprintf("Order %s - Status Update", orderNumber)
		body := fmt.Sprintf(`<h2>Order Status Update</h2>
<p>Hi %s,</p>
<p>Your order <strong>%s</strong> status has been updated to: <strong>%s</strong></p>
<p>The Grabbi Team</p>`, strings.Split(name, " ")[0], orderNumber, strings.ReplaceAll(status, "_", " "))
		if err := SendEmail(email, subject, body); err != nil {
			log.Printf("Failed to send status update email to %s: %v", email, err)
		}
	}()
}

func SendPasswordResetEmail(email, name, resetToken, frontendURL string) {
	go func() {
		resetLink := fmt.Sprintf("%s/reset-password?token=%s", frontendURL, resetToken)
		subject := "Reset Your Password - Grabbi"
		body := fmt.Sprintf(`<h2>Password Reset Request</h2>
<p>Hi %s,</p>
<p>We received a request to reset your password. Click the link below to set a new password:</p>
<p><a href="%s" style="display:inline-block;padding:12px 24px;background:#00D4AA;color:#1a1a2e;text-decoration:none;border-radius:8px;font-weight:bold;">Reset Password</a></p>
<p>This link will expire in 1 hour.</p>
<p>If you didn't request this, you can safely ignore this email.</p>
<p>The Grabbi Team</p>`, strings.Split(name, " ")[0], resetLink)
		if err := SendEmail(email, subject, body); err != nil {
			log.Printf("Failed to send password reset email to %s: %v", email, err)
		}
	}()
}

// SendStaffInvitationEmail sends an invitation email to a newly created staff member
func SendStaffInvitationEmail(email, name, franchiseName, role, password, franchiseURL string) {
	go func() {
		displayName := name
		if displayName == "" {
			displayName = "Team Member"
		}
		
		roleDisplay := "Staff Member"
		if role == "manager" {
			roleDisplay = "Manager"
		}
		
		subject := fmt.Sprintf("You've been added as %s at %s Franchise - Grabbi", roleDisplay, franchiseName)
		body := fmt.Sprintf(`<h2>Welcome to %s Franchise!</h2>
<p>Hi %s,</p>
<p>You have been added as a <strong>%s</strong> at <strong>%s Franchise</strong> on Grabbi.</p>
<p>Your account has been created with the following details:</p>
<div style="background:#f5f5f5;padding:15px;border-radius:8px;margin:20px 0;">
<p style="margin:5px 0;"><strong>Email:</strong> %s</p>
<p style="margin:5px 0;"><strong>Temporary Password:</strong> %s</p>
</div>
<p><a href="%s" style="display:inline-block;padding:12px 24px;background:#00D4AA;color:#1a1a2e;text-decoration:none;border-radius:8px;font-weight:bold;">Access Franchise Portal</a></p>
<p><strong>Important:</strong> Please log in and change your password immediately for security purposes.</p>
<p>As a %s, you can:</p>
<ul>
<li>Manage products (create, update, delete)</li>
<li>Create and manage promotions</li>
<li>Manage orders and update order status</li>
<li>View dashboard analytics</li>
</ul>
<p>If you have any questions, please contact the franchise owner.</p>
<p>The Grabbi Team</p>`, 
			franchiseName,
			strings.Split(displayName, " ")[0], 
			roleDisplay, 
			franchiseName, 
			email, 
			password, 
			franchiseURL,
			roleDisplay)
		
		if err := SendEmail(email, subject, body); err != nil {
			log.Printf("Failed to send staff invitation email to %s: %v", email, err)
		} else {
			log.Printf("Staff invitation email sent successfully to %s", email)
		}
	}()
}

// SendStaffAddedEmail sends a notification email to an existing user who was added as staff
func SendStaffAddedEmail(email, name, franchiseName, role, franchiseURL string) {
	go func() {
		displayName := name
		if displayName == "" {
			displayName = "Team Member"
		}
		
		roleDisplay := "Staff Member"
		if role == "manager" {
			roleDisplay = "Manager"
		}
		
		subject := fmt.Sprintf("You've been added as %s at %s Franchise - Grabbi", roleDisplay, franchiseName)
		body := fmt.Sprintf(`<h2>You've been added to %s Franchise!</h2>
<p>Hi %s,</p>
<p>You have been added as a <strong>%s</strong> at <strong>%s Franchise</strong> on Grabbi.</p>
<p><a href="%s" style="display:inline-block;padding:12px 24px;background:#00D4AA;color:#1a1a2e;text-decoration:none;border-radius:8px;font-weight:bold;">Access Franchise Portal</a></p>
<p>As a %s, you can:</p>
<ul>
<li>Manage products (create, update, delete)</li>
<li>Create and manage promotions</li>
<li>Manage orders and update order status</li>
<li>View dashboard analytics</li>
</ul>
<p>Log in with your existing Grabbi account to get started.</p>
<p>If you have any questions, please contact the franchise owner.</p>
<p>The Grabbi Team</p>`, 
			franchiseName,
			strings.Split(displayName, " ")[0], 
			roleDisplay, 
			franchiseName, 
			franchiseURL,
			roleDisplay)
		
		if err := SendEmail(email, subject, body); err != nil {
			log.Printf("Failed to send staff added email to %s: %v", email, err)
		} else {
			log.Printf("Staff added email sent successfully to %s", email)
		}
	}()
}
