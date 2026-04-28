// Package service implements identity business use cases.
package service

import "time"

const (
	SessionCookieName = "sid"
	CSRFCookieName    = "csrf_token"
	CSRFHeaderName    = "X-CSRF-Token"

	SessionCookiePath = "/"
)

const (
	JSONEmptyObject = `{}`
	JSONTrueString  = "true"
)

const (
	OperationAuthAudit                = "auth_audit"
	OperationAuthSessionIssue         = "auth_session_issue"
	OperationIdentityRetentionCleanup = "identity_retention_cleanup"
)

const (
	AuditEventLoginFailure      = "login_failure"
	AuditEventLoginSuccess      = "login_success"
	AuditEventLogout            = "logout"
	AuditEventSessionRevocation = "session_revocation"
	AuditEventSignup            = "signup"
)

const (
	AuditReasonAccountNotActive = "account_not_active"
	AuditReasonBadPassword      = "bad_password"
	AuditReasonMissingSession   = "missing_session"
	AuditReasonRevoked          = "revoked"
	AuditReasonRotation         = "rotation"
	AuditReasonSessionNotFound  = "session_not_found"
	AuditReasonSuccess          = "success"
	AuditReasonUnknownAccount   = "unknown_account"
)

const (
	OAuthStateTTL       = 10 * time.Minute
	CSRFBootstrapTTL    = time.Hour
	OIDCClockSkewLeeway = 120 * time.Second

	PasswordMinLength = 12
	PasswordMaxBytes  = 72
)
