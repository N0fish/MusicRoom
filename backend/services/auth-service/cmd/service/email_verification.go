package main

import (
	"fmt"
	"log"
	"strings"
)

func (s *Server) buildVerificationURL(token string) string {
	// Если задан EMAIL_VERIFICATION_URL
	base := strings.TrimSpace(s.verificationURLBase)
	if base != "" {
		sep := "?"
		if strings.Contains(base, "?") {
			sep = "&"
		}
		return fmt.Sprintf("%s%stoken=%s", base, sep, token)
	}

	return s.frontendURL + "?mode=verify-email&token=" + token
}

func (s *Server) sendVerificationEmail(user AuthUser, token string) {
	link := s.buildVerificationURL(token)

	subject := "Verify your MusicRoom email"
	body := fmt.Sprintf(
		"Hi!\n\nTo verify your email for MusicRoom, please click the link:\n\n%s\n\n"+
			"If you didn’t request this, you can safely ignore this email.\n",
		link,
	)

	if err := s.emailSender.Send(user.Email, subject, body); err != nil {
		log.Printf("sendVerificationEmail: send error: %v", err)
	} else {
		log.Printf("[auth-service] email verification for %s: %s", user.Email, link)
	}
}

func (s *Server) buildResetURL(token string) string {
	base := strings.TrimSpace(s.resetURLBase)
	if base != "" {
		sep := "?"
		if strings.Contains(base, "?") {
			sep = "&"
		}
		return fmt.Sprintf("%s%stoken=%s", base, sep, token)
	}
	return s.frontendURL + "?mode=reset-password&token=" + token
}

func (s *Server) sendResetPasswordEmail(user AuthUser, token string) {
	link := s.buildResetURL(token)

	subject := "Reset your MusicRoom password"
	body := fmt.Sprintf(
		"Hi!\n\nTo reset your MusicRoom password, please click the link:\n\n%s\n\n"+
			"If you didn’t request a password reset, you can safely ignore this email.\n",
		link,
	)

	if err := s.emailSender.Send(user.Email, subject, body); err != nil {
		log.Printf("sendResetPasswordEmail: send error: %v", err)
	} else {
		log.Printf("[auth-service] password reset for %s: %s", user.Email, link)
	}
}
