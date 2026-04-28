CREATE TABLE accounts (
    id uuid PRIMARY KEY,
    email text NOT NULL,
    normalized_email text GENERATED ALWAYS AS (lower(btrim(email))) STORED,
    display_name text,
    status text NOT NULL DEFAULT 'active',
    email_verified_at timestamptz,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT accounts_status_check CHECK (status IN ('active', 'disabled', 'deleted')),
    CONSTRAINT accounts_email_not_blank CHECK (length(trim(email)) > 0)
);

CREATE UNIQUE INDEX accounts_normalized_email_uq
    ON accounts (normalized_email)
    WHERE status <> 'deleted';

CREATE TABLE auth_identities (
    id uuid PRIMARY KEY,
    account_id uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    provider text NOT NULL,
    provider_subject text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT auth_identities_provider_check CHECK (provider IN ('password', 'google', 'apple')),
    CONSTRAINT auth_identities_subject_not_blank CHECK (length(trim(provider_subject)) > 0)
);

CREATE UNIQUE INDEX auth_identities_provider_subject_uq
    ON auth_identities (provider, provider_subject);

CREATE INDEX auth_identities_account_id_idx
    ON auth_identities (account_id);

CREATE TABLE password_credentials (
    account_id uuid PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    password_hash text NOT NULL,
    password_algo text NOT NULL,
    password_params jsonb NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT password_credentials_algo_not_blank CHECK (length(trim(password_algo)) > 0)
);

CREATE TABLE sessions (
    id uuid PRIMARY KEY,
    account_id uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    token_hash text NOT NULL,
    issued_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL,
    revoked_at timestamptz,
    absolute_expires_at timestamptz NOT NULL,
    CONSTRAINT sessions_token_hash_not_blank CHECK (length(trim(token_hash)) > 0)
);

CREATE UNIQUE INDEX sessions_token_hash_active_uq
    ON sessions (token_hash)
    WHERE revoked_at IS NULL;

CREATE INDEX sessions_account_id_idx
    ON sessions (account_id);

CREATE INDEX sessions_active_lookup_idx
    ON sessions (token_hash, expires_at)
    WHERE revoked_at IS NULL;

CREATE TABLE oauth_states (
    id uuid PRIMARY KEY,
    provider text NOT NULL,
    state_hash text NOT NULL,
    pkce_verifier_hash text NOT NULL,
    nonce_hash text NOT NULL,
    redirect_url text NOT NULL,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at timestamptz NOT NULL,
    CONSTRAINT oauth_states_provider_check CHECK (provider IN ('google', 'apple')),
    CONSTRAINT oauth_states_state_hash_not_blank CHECK (length(trim(state_hash)) > 0)
);

CREATE UNIQUE INDEX oauth_states_provider_state_hash_uq
    ON oauth_states (provider, state_hash);

CREATE INDEX oauth_states_expiry_idx
    ON oauth_states (expires_at)
    WHERE consumed_at IS NULL;
