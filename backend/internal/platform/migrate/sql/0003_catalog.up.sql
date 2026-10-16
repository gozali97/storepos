-- 0003 — Katalog: kategori, produk, varian, barcode (CORE-007, CORE-008, CORE-011)
-- Semua tabel membawa business_id; FK komposit mencegah relasi lintas tenant (§13.13).

BEGIN;

CREATE TABLE categories (
    id          UUID PRIMARY KEY,
    business_id UUID NOT NULL REFERENCES businesses(id),
    name        TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    status      TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    version     INT  NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (business_id, id)
);
CREATE UNIQUE INDEX categories_name_key ON categories(business_id, lower(name)) WHERE status = 'active';

CREATE TABLE products (
    id          UUID PRIMARY KEY,
    business_id UUID NOT NULL REFERENCES businesses(id),
    category_id UUID,
    name        TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    track_stock BOOLEAN NOT NULL DEFAULT true,
    status      TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'archived')),
    version     INT  NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (business_id, id),
    FOREIGN KEY (business_id, category_id) REFERENCES categories(business_id, id)
);
CREATE INDEX products_category_idx ON products(business_id, category_id);

CREATE TABLE variants (
    id                 UUID PRIMARY KEY,
    business_id        UUID NOT NULL REFERENCES businesses(id),
    product_id         UUID NOT NULL,
    name               TEXT NOT NULL DEFAULT '',   -- '' = varian default
    sku                TEXT NOT NULL CHECK (length(btrim(sku)) > 0),
    base_unit          TEXT NOT NULL DEFAULT 'pcs',
    price_cents        BIGINT NOT NULL CHECK (price_cents >= 0),
    price_includes_tax BOOLEAN NOT NULL DEFAULT false,
    status             TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'archived')),
    version            INT  NOT NULL DEFAULT 1,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (business_id, id),
    FOREIGN KEY (business_id, product_id) REFERENCES products(business_id, id)
);
-- SKU unik per bisnis, tanpa membedakan huruf besar/kecil (CORE-008).
CREATE UNIQUE INDEX variants_sku_key ON variants(business_id, lower(sku));
CREATE INDEX variants_product_idx ON variants(business_id, product_id);

CREATE TABLE variant_barcodes (
    business_id UUID NOT NULL,
    barcode     TEXT NOT NULL CHECK (length(btrim(barcode)) > 0),
    variant_id  UUID NOT NULL,
    PRIMARY KEY (business_id, barcode),   -- unik per bisnis
    FOREIGN KEY (business_id, variant_id) REFERENCES variants(business_id, id) ON DELETE CASCADE
);
CREATE INDEX variant_barcodes_variant_idx ON variant_barcodes(business_id, variant_id);

COMMIT;
