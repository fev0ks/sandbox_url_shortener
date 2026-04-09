// Package eventing holds shared event-pipeline defaults used across services.
package eventing

const (
	DefaultKafkaBrokers           = "localhost:29092"
	DefaultLinkVisitedTopic       = "link.visited.v1"
	DefaultLinkVisitedDLQTopic    = "link.visited.v1.dlq"
	DefaultAnalyticsConsumerGroup = "analytics-link-visited-v1"
)
