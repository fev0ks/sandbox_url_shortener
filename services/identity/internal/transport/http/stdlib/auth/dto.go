package auth

import "time"

type passwordRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

type csrfResponse struct {
	CSRFToken string `json:"csrf_token"`
	Header    string `json:"header"`
	Cookie    string `json:"cookie"`
}

type sessionResponse struct {
	Account accountResponse `json:"account"`
	Session sessionInfo     `json:"session"`
}

type accountResponse struct {
	ID            string   `json:"id"`
	Email         string   `json:"email"`
	DisplayName   *string  `json:"display_name,omitempty"`
	EmailVerified bool     `json:"email_verified"`
	Providers     []string `json:"providers"`
}

type sessionInfo struct {
	ExpiresAt time.Time `json:"expires_at"`
}
