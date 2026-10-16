CREATE SCHEMA IF NOT EXISTS platform;

CREATE TABLE platform.leads (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    phone TEXT NOT NULL,
    business_name TEXT NOT NULL,
    business_type TEXT NOT NULL CHECK (business_type IN ('retail','restaurant','pharmacy','other')),
    city TEXT NOT NULL,
    email TEXT NOT NULL DEFAULT '',
    branches TEXT NOT NULL DEFAULT '',
    terminals TEXT NOT NULL DEFAULT '',
    current_system TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    contact_time TEXT NOT NULL DEFAULT '',
    consent BOOLEAN NOT NULL CHECK (consent),
    marketing_consent BOOLEAN NOT NULL DEFAULT false,
    consent_version TEXT NOT NULL,
    consent_text TEXT NOT NULL,
    privacy_version TEXT NOT NULL,
    consent_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    source_page TEXT NOT NULL DEFAULT '/',
    source_tab TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'new' CHECK (status IN ('new','contacted','scheduled','completed','irrelevant')),
    assigned_to UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX leads_dedup_idx ON platform.leads(phone,business_type,created_at DESC);

CREATE TABLE platform.lead_requests (
    idempotency_key UUID PRIMARY KEY,
    lead_id UUID NOT NULL REFERENCES platform.leads(id),
    request_hash BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE platform.lead_limits (
    peer_hash BYTEA PRIMARY KEY,
    window_start TIMESTAMPTZ NOT NULL DEFAULT now(),
    requests INT NOT NULL DEFAULT 1 CHECK (requests BETWEEN 1 AND 6)
);
