-- Local bootstrap credentials; this script is deliberately separate from application migrations.
\set ON_ERROR_STOP on
BEGIN;
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'mcp_reader') THEN
        CREATE ROLE mcp_reader;
    END IF;
END
$$;
ALTER ROLE mcp_reader WITH LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE
    NOINHERIT NOREPLICATION NOBYPASSRLS PASSWORD 'mcp_reader_local';
ALTER ROLE mcp_reader SET default_transaction_read_only = on;
ALTER ROLE mcp_reader SET search_path = pg_catalog, mcp_investigation;
ALTER ROLE mcp_reader SET statement_timeout = '5s';
ALTER ROLE mcp_reader SET lock_timeout = '500ms';

-- PUBLIC TEMP would allow writable temporary objects even in a read-only transaction.
REVOKE TEMPORARY ON DATABASE shortener FROM PUBLIC;
GRANT TEMPORARY ON DATABASE shortener TO shortener;
REVOKE ALL ON DATABASE shortener FROM mcp_reader;
GRANT CONNECT ON DATABASE shortener TO mcp_reader;
CREATE SCHEMA IF NOT EXISTS mcp_investigation AUTHORIZATION shortener;
REVOKE ALL ON SCHEMA mcp_investigation FROM PUBLIC, mcp_reader;
GRANT USAGE ON SCHEMA mcp_investigation TO mcp_reader;
REVOKE ALL ON SCHEMA public FROM mcp_reader;
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM mcp_reader;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM mcp_reader;

-- Only audited diagnostic columns cross this boundary. URLs, titles and account identifiers do not.
CREATE OR REPLACE VIEW mcp_investigation.links WITH (security_barrier = true) AS
SELECT id, short_code, created_at, updated_at, expires_at, max_clicks,
       redirect_count, disabled_at, deleted_at, last_accessed_at
FROM public.links;
REVOKE ALL ON ALL TABLES IN SCHEMA mcp_investigation FROM PUBLIC, mcp_reader;
GRANT SELECT ON mcp_investigation.links TO mcp_reader;
COMMIT;
