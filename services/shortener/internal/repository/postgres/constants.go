package postgres

const (
	shortenerService = "shortener"

	pgErrCodeUniqueViolation = "23505"

	pgOpLinkCreate              = "link_create"
	pgOpLinkListRecent          = "link_list_recent"
	pgOpLinkGetByID             = "link_get_by_id"
	pgOpLinkDisable             = "link_disable"
	pgOpLinkEnable              = "link_enable"
	pgOpLinkSoftDelete          = "link_soft_delete"
	pgOpLinkResolveAndIncrement = "link_resolve_and_increment"
	pgOpLinkTouchRedirect       = "link_touch_redirect"
	pgOpLinkClassifyMiss        = "link_classify_miss"

	pgResultSuccess            = "success"
	pgResultError              = "error"
	pgResultNotFound           = "not_found"
	pgResultShortCodeCollision = "short_code_collision"
)
