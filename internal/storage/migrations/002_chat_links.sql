CREATE TABLE chat_link_tokens (
    provider text NOT NULL,
    user_id text NOT NULL,
    token_hash bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (provider, user_id),
    UNIQUE (provider, token_hash)
);
