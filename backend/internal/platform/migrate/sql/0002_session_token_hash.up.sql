-- 0002 — Hash token sesi
--
-- Sesi server-side menyimpan SHA-256 token, bukan token mentah (PRD §16.5).
-- Token mentah hanya ada di cookie browser, sehingga dump database tidak
-- cukup untuk membajak sesi.
--
-- Pola expand: kolom baru ditambahkan nullable, lalu diisi, lalu diberi
-- constraint NOT NULL. Tidak ada kolom lama yang diubah atau dihapus, sehingga
-- versi aplikasi sebelumnya tetap dapat berjalan (PRD §16.6).

BEGIN;

ALTER TABLE sessions
    ADD COLUMN token_hash TEXT;

-- Sesi lama (dari skema sebelumnya) tidak punya token yang dapat dihitung
-- ulang; cabut saja agar tidak ada sesi tanpa verifikasi.
UPDATE sessions
   SET revoked_at = COALESCE(revoked_at, now())
 WHERE token_hash IS NULL;

ALTER TABLE sessions
    ALTER COLUMN token_hash SET NOT NULL;

-- Lookup sesi pada setiap request harus satu indeks unik.
CREATE UNIQUE INDEX sessions_token_hash_key ON sessions(token_hash);

-- Pemindaian sesi kedaluwarsa oleh worker.
CREATE INDEX sessions_expires_idx
    ON sessions(expires_at)
    WHERE revoked_at IS NULL;

-- Bisnis aktif disimpan di sesi (kolom sudah ada di baseline); index ini
-- mempercepat pembersihan sesi per pengguna.
CREATE INDEX sessions_user_active_idx
    ON sessions(user_id)
    WHERE revoked_at IS NULL;

COMMIT;
