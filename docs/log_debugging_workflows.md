# Log Debugging Workflows

## Purpose

This document captures the local debugging queries validated for the E07 log observability milestone.
Use it as the quick reference for Elasticsearch and Kibana searches.

## Index Pattern

- Elasticsearch index pattern: `shortener-logs-*`
- Kibana data view should point at the same pattern

## Query Style

Filter first by `service`, then by one of:

- `request_id`
- `short_code`
- `operation`
- `error_kind`
- `event_id`

## Validated Workflows

### Request Trace By `request_id`

Validated query:

```text
request_id : "4f662fd9-f683-5424-60ed-28cb0c6e91aa"
```

Expected result shape:

- one `operation=redirect_resolve` document
- one `operation=http_request` document

Equivalent Elasticsearch query:

```json
{
  "query": {
    "term": {
      "request_id.keyword": "4f662fd9-f683-5424-60ed-28cb0c6e91aa"
    }
  },
  "sort": [
    {
      "@timestamp": "desc"
    }
  ]
}
```

### Redirect Failure By `short_code`

Validated query:

```text
service : "shortener" and operation : "redirect_resolve" and short_code : "missing-e07-search"
```

Expected result shape:

- `error_kind=not_found`
- matching request correlation fields

### Kafka Publish Degradation By `operation`

Validated query:

```text
service : "shortener" and operation : "publish_link_visited" and error_kind : "publish_failed"
```

For one link:

```text
service : "shortener" and operation : "publish_link_visited" and error_kind : "publish_failed" and short_code : "oQCg7hp"
```

Expected result shape:

- async publish failure records with `event_id`, `link_id`, and `short_code`

### Analytics Consumer Failure By `operation` And `error_kind`

Validated query:

```text
service : "analytics" and operation : "consumer_fetch" and error_kind : "invalid_payload"
```

Related query for broker outages:

```text
service : "analytics" and operation : "consumer_fetch" and error_kind : "fetch_failed"
```

## Practical Notes

- `request_id` and `correlation_id` are the fastest entry points for one HTTP request
- `short_code` is the fastest entry point for redirect problems
- `operation` and `error_kind` are the fastest entry points for async path failures
- `event_id` is available on publish paths, but invalid payload failures may not have a decoded event id yet
