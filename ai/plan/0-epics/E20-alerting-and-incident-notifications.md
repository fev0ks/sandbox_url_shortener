# E20 - Alerting And Incident Notifications

Status: active

## Summary

Operational readiness epic for detecting system failures or degradation and delivering actionable
alerts outside the platform.

## Goal

Add a baseline alerting layer that detects high-signal failures and degradation across the stack
and sends notifications to an external channel such as a Telegram bot.

## Scope

- alert rules for availability, error rate, latency, and pipeline degradation
- alert delivery routing to an external notification channel
- Telegram bot as the first notification sink
- severity levels, deduplication, and cooldown behavior
- alert payload structure with enough context to start incident triage
- excludes full incident management platforms and on-call scheduling systems

## Key Decisions

- use the observability stack as the alert source of truth instead of embedding alert logic in
  application services
- Telegram bot is the first delivery channel, not a permanent lock-in
- start with a small set of high-signal alerts to avoid noisy paging
- include runbook references or remediation hints where practical

## Task Links

- `E20-001-define-mvp-alert-rules-and-severity-model.md`
- `E20-002-wire-alert-evaluation-and-routing.md`
- `E20-003-add-telegram-bot-notification-channel.md`
- `E20-004-test-alert-delivery-and-noise-controls.md`
- `E20-005-document-alert-runbook-and-escalation-path.md`

## Open Questions

- whether alert evaluation should be owned by Prometheus Alertmanager or Grafana Alerting in the
  local stack
- which degradation thresholds are stable enough for MVP without creating noisy alerts
