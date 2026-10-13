package app

// developmentKeys is an intentionally public fixture for local educational/demo use,
// not a production credential. Production deployments must replace it with a private
// registry loaded from protected configuration or a secret store.
// Each invocation owns a new map;
// the HTTP boundary copies it and never exposes key values in logs or tool output.
// Remove or replace an entry and rebuild/restart MCP to revoke or rotate access.
func developmentKeys() map[string]string {
	return map[string]string{
		"aF8hlI_Fy9_6D5hVbghcf67W970uhDYqgPkMpEAGrO0": "local-codex",
	}
}
