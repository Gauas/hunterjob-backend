CREATE TABLE search_preferences (
    id text PRIMARY KEY DEFAULT left(replace(gen_random_uuid()::text, '-', ''), 24),
    user_id text NOT NULL UNIQUE,
    role text NOT NULL,
    experience jsonb NOT NULL,
    locations text[] NOT NULL,
    keywords text[] NOT NULL,
    excluded_keywords text[] NOT NULL,
    frequency text NOT NULL,
    enabled boolean NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE job_matches (
    id text COLLATE "C" PRIMARY KEY DEFAULT left(replace(gen_random_uuid()::text, '-', ''), 24),
    user_id text NOT NULL,
    job_id text NOT NULL,
    dedupe_key text,
    preference_id text NOT NULL,
    match_level text NOT NULL,
    matched_keywords text[] NOT NULL,
    match_reason text NOT NULL,
    status text NOT NULL,
    matched_at timestamptz NOT NULL,
    delivered boolean NOT NULL DEFAULT false,
    delivered_at timestamptz,
    created_at timestamptz NOT NULL,
    job jsonb NOT NULL,
    UNIQUE (user_id, job_id),
    UNIQUE (user_id, dedupe_key)
);

CREATE INDEX job_matches_recent ON job_matches (user_id, matched_at DESC, id DESC);
CREATE INDEX job_matches_status_recent ON job_matches (user_id, status, matched_at DESC, id DESC);

CREATE TABLE chat_connections (
    id text PRIMARY KEY DEFAULT left(replace(gen_random_uuid()::text, '-', ''), 24),
    user_id text NOT NULL,
    provider text NOT NULL,
    status text NOT NULL,
    enabled boolean NOT NULL,
    external_user_id text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, provider)
);

CREATE UNIQUE INDEX chat_connections_recipient ON chat_connections (provider, external_user_id)
    WHERE status = 'connected' AND external_user_id <> '';

CREATE TABLE notification_deliveries (
    id text PRIMARY KEY DEFAULT left(replace(gen_random_uuid()::text, '-', ''), 24),
    user_id text NOT NULL,
    provider text NOT NULL,
    match_ids text[] NOT NULL,
    status text NOT NULL,
    attempts integer NOT NULL,
    sent_at timestamptz,
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL
);

CREATE INDEX notification_deliveries_recent ON notification_deliveries (user_id, created_at DESC);
