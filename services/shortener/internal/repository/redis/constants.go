package redis

const (
	shortenerService = "shortener"

	redisOpLinkCacheGet    = "link_cache_get"
	redisOpLinkCacheSet    = "link_cache_set"
	redisOpLinkCacheDelete = "link_cache_delete"
	redisOpRateLimitAllow  = "rate_limit_allow"
	redisOpIdempotencyGet  = "idempotency_get"
	redisOpIdempotencySet  = "idempotency_set"

	redisResultSuccess      = "success"
	redisResultError        = "error"
	redisResultMiss         = "miss"
	redisResultHit          = "hit"
	redisResultCorrupt      = "corrupt"
	redisResultSkipped      = "skipped"
	redisResultMarshalError = "marshal_error"
	redisResultAllowed      = "allowed"
	redisResultLimited      = "limited"
)
