-- 0001 — Baseline fondasi
-- Identity, organisasi, keanggotaan, dan infrastruktur idempotency/audit/outbox.
-- Lihat PRD §15 untuk model konseptual.

BEGIN;

-- Ekstensi uuid-ossp tidak diperlukan karena UUID dihasilkan aplikasi (UUIDv7, PRD §13.13).
CREATE EXTENSION IF NOT EXISTS citext;

-- =========================================================================
-- IDENTITY
-- =========================================================================

CREATE TABLE users (
    id              UUID PRIMARY KEY,
    email           CITEXT UNIQUE,
    phone           TEXT UNIQUE,
    password_hash   TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active', 'disabled')),
    email_verified_at TIMESTAMPTZ,
    phone_verified_at TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (email IS NOT NULL OR phone IS NOT NULL)
);

CREATE TABLE sessions (
    id               UUID PRIMARY KEY,
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    active_business_id UUID,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at       TIMESTAMPTZ NOT NULL,
    last_seen_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at       TIMESTAMPTZ,
    user_agent       TEXT,
    ip               INET
);
CREATE INDEX sessions_user_idx ON sessions(user_id) WHERE revoked_at IS NULL;

-- =========================================================================
-- ORG (TENANT = BUSINESS)
-- =========================================================================

CREATE TABLE businesses (
    id              UUID PRIMARY KEY,
    name            TEXT NOT NULL,
    business_type   TEXT NOT NULL
                    CHECK (business_type IN ('retail', 'restaurant', 'pharmacy')),
    enabled_modules TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    settings        JSONB NOT NULL DEFAULT '{}'::jsonb,
    first_sale_completed_at TIMESTAMPTZ, -- untuk mengunci perubahan jenis usaha (§5.3)
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by      UUID NOT NULL REFERENCES users(id)
);

CREATE TABLE branches (
    id              UUID PRIMARY KEY,
    business_id     UUID NOT NULL REFERENCES businesses(id) ON DELETE RESTRICT,
    code            TEXT NOT NULL,
    name            TEXT NOT NULL,
    timezone        TEXT NOT NULL DEFAULT 'Asia/Jakarta',
    address         TEXT,
    receipt_header  TEXT,
    receipt_footer  TEXT,
    status          TEXT NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active', 'archived')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (business_id, code)
);

-- Index unik komposit yang menjadi rujukan FK komposit tabel anak.
-- Harus dibuat sebelum tabel yang merujuknya (§13.13).
CREATE UNIQUE INDEX branches_business_id_id_key ON branches(business_id, id);

-- Terminal (register) — perangkat kasir, dasar shift.
CREATE TABLE registers (
    id              UUID PRIMARY KEY,
    business_id     UUID NOT NULL,
    branch_id       UUID NOT NULL,
    name            TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active', 'inactive', 'archived')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- FK komposit menyertakan business_id agar relasi lintas tenant tidak mungkin (§13.13).
    FOREIGN KEY (business_id, branch_id)
        REFERENCES branches(business_id, id)
);

-- =========================================================================
-- MEMBERSHIP & RBAC
-- =========================================================================

CREATE TABLE memberships (
    id              UUID PRIMARY KEY,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    business_id     UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    role            TEXT NOT NULL
                    CHECK (role IN ('owner', 'manager', 'cashier', 'warehouse', 'kitchen', 'pharmacist')),
    status          TEXT NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active', 'disabled')),
    pin_hash        TEXT,
    pin_locked_until TIMESTAMPTZ,
    pin_fail_count  INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, business_id)
);

CREATE TABLE membership_branches (
    membership_id   UUID NOT NULL REFERENCES memberships(id) ON DELETE CASCADE,
    business_id     UUID NOT NULL,
    branch_id       UUID NOT NULL,
    PRIMARY KEY (membership_id, branch_id),
    FOREIGN KEY (business_id, branch_id)
        REFERENCES branches(business_id, id) ON DELETE CASCADE
);

CREATE TABLE invitations (
    id              UUID PRIMARY KEY,
    business_id     UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    contact         TEXT NOT NULL, -- email atau phone
    role            TEXT NOT NULL,
    branch_ids      UUID[] NOT NULL DEFAULT ARRAY[]::UUID[],
    invited_by      UUID NOT NULL REFERENCES users(id),
    token_hash      TEXT NOT NULL,
    expires_at      TIMESTAMPTZ NOT NULL,
    accepted_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX invitations_contact_idx ON invitations(business_id, contact) WHERE accepted_at IS NULL;

-- =========================================================================
-- AUDIT (append-only)
-- =========================================================================

CREATE TABLE audit_logs (
    id              UUID PRIMARY KEY,
    business_id     UUID,          -- NULL untuk event platform (login global, dll)
    branch_id       UUID,
    register_id     UUID,
    actor_user_id   UUID,
    approved_by_user_id UUID,
    action          TEXT NOT NULL,
    entity_type     TEXT NOT NULL,
    entity_id       TEXT,
    before_jsonb    JSONB,
    after_jsonb     JSONB,
    reason          TEXT,
    ip              INET,
    user_agent      TEXT,
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX audit_business_time_idx ON audit_logs(business_id, occurred_at DESC);
CREATE INDEX audit_entity_idx ON audit_logs(entity_type, entity_id);

-- =========================================================================
-- IDEMPOTENCY & OUTBOX & COUNTERS
-- =========================================================================

CREATE TABLE idempotency_keys (
    business_id     UUID NOT NULL,
    key             TEXT NOT NULL,
    endpoint        TEXT NOT NULL,
    request_hash    BYTEA NOT NULL,
    response_status INT,
    response_body   JSONB,
    status          TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'completed')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (business_id, key)
);
CREATE INDEX idempotency_expires_idx ON idempotency_keys(expires_at);

CREATE TABLE outbox_jobs (
    id              UUID PRIMARY KEY,
    business_id     UUID, -- NULL untuk job platform (misal notifikasi lead)
    kind            TEXT NOT NULL,
    payload         JSONB NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    attempts        INT NOT NULL DEFAULT 0,
    last_error      TEXT,
    available_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ
);
CREATE INDEX outbox_ready_idx ON outbox_jobs(available_at)
    WHERE status IN ('pending', 'processing');

-- Counter dokumen untuk nomor struk tanpa celah (CORE-046).
-- (business_id, branch_id, doc_type, date) → last_number
CREATE TABLE document_counters (
    business_id     UUID NOT NULL,
    branch_id       UUID NOT NULL,
    doc_type        TEXT NOT NULL,
    doc_date        DATE NOT NULL,
    last_number     INT NOT NULL DEFAULT 0,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (business_id, branch_id, doc_type, doc_date),
    FOREIGN KEY (business_id, branch_id)
        REFERENCES branches(business_id, id)
);

-- Trigger yang mencegah UPDATE/DELETE pada audit_logs.
-- Catatan: role aplikasi juga akan REVOKE UPDATE/DELETE pada tabel ini di migrasi
-- berikutnya (setelah role aplikasi dibuat). Trigger adalah lapisan pertahanan kedua.
CREATE OR REPLACE FUNCTION audit_logs_block_mutation()
RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs bersifat append-only';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_logs_no_update
    BEFORE UPDATE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION audit_logs_block_mutation();

CREATE TRIGGER audit_logs_no_delete
    BEFORE DELETE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION audit_logs_block_mutation();

-- =========================================================================
-- VERSI SKEMA
-- =========================================================================
-- Tabel schema_migrations TIDAK dibuat di sini.
-- Runner migrasi (backend/internal/platform/migrate) yang memilikinya dan
-- membuatnya sendiri sebelum menjalankan migrasi apa pun, dengan kolom
-- (version INT, name TEXT, applied_at TIMESTAMPTZ). Mendefinisikannya lagi di
-- sini akan menabrak definisi tersebut.

COMMIT;
