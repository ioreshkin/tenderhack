package app

const schemaSQL = `
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE IF NOT EXISTS cte_items (
    id BIGINT PRIMARY KEY,
    name TEXT NOT NULL,
    category TEXT NOT NULL,
    manufacturer TEXT NOT NULL,
    attrs_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    attrs_text TEXT NOT NULL DEFAULT '',
    name_norm TEXT NOT NULL,
    category_norm TEXT NOT NULL,
    manufacturer_norm TEXT NOT NULL,
    search_text TEXT NOT NULL,
    search_tsv tsvector GENERATED ALWAYS AS (
        to_tsvector(
            'simple',
            coalesce(name_norm, '') || ' ' ||
            coalesce(category_norm, '') || ' ' ||
            coalesce(manufacturer_norm, '') || ' ' ||
            coalesce(attrs_text, '')
        )
    ) STORED,
    contract_count INTEGER NOT NULL DEFAULT 0,
    avg_unit_price NUMERIC(18, 4),
    last_contract_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS contract_items (
    id BIGSERIAL PRIMARY KEY,
    procurement_name TEXT NOT NULL DEFAULT '',
    quantity NUMERIC(18, 5),
    unit TEXT NOT NULL DEFAULT '',
    contract_id BIGINT NOT NULL,
    method TEXT NOT NULL DEFAULT '',
    initial_cost NUMERIC(18, 5),
    final_cost NUMERIC(18, 5),
    discount_pct NUMERIC(18, 5),
    vat TEXT NOT NULL DEFAULT '',
    contract_date TIMESTAMPTZ,
    customer_inn TEXT NOT NULL DEFAULT '',
    customer_region TEXT NOT NULL DEFAULT '',
    supplier_inn TEXT NOT NULL DEFAULT '',
    supplier_region TEXT NOT NULL DEFAULT '',
    cte_id BIGINT NOT NULL,
    cte_name TEXT NOT NULL DEFAULT '',
    unit_price NUMERIC(18, 5) NOT NULL
);

CREATE TABLE IF NOT EXISTS dataset_meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS doc_versions (
    id BIGSERIAL PRIMARY KEY,
    cte_id BIGINT NOT NULL,
    version INTEGER NOT NULL,
    region TEXT NOT NULL DEFAULT '',
    months_back INTEGER NOT NULL,
    params_json JSONB NOT NULL,
    summary TEXT NOT NULL,
    file_path TEXT NOT NULL DEFAULT '',
    body_html TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (cte_id, version)
);

CREATE TABLE IF NOT EXISTS batch_doc_versions (
    id BIGSERIAL PRIMARY KEY,
    batch_key TEXT NOT NULL,
    batch_name TEXT NOT NULL,
    version INTEGER NOT NULL,
    region TEXT NOT NULL DEFAULT '',
    months_back INTEGER NOT NULL,
    item_count INTEGER NOT NULL DEFAULT 0,
    params_json JSONB NOT NULL,
    summary TEXT NOT NULL,
    file_path TEXT NOT NULL DEFAULT '',
    body_html TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (batch_key, version)
);

ALTER TABLE doc_versions
    ADD COLUMN IF NOT EXISTS file_path TEXT NOT NULL DEFAULT '';

ALTER TABLE batch_doc_versions
    ADD COLUMN IF NOT EXISTS file_path TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_batch_doc_versions_created_at ON batch_doc_versions (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_cte_items_category ON cte_items (category);
CREATE INDEX IF NOT EXISTS idx_cte_items_category_contract_count ON cte_items (category, contract_count DESC);
CREATE INDEX IF NOT EXISTS idx_cte_items_contract_count ON cte_items (contract_count DESC);
CREATE INDEX IF NOT EXISTS idx_cte_items_name_prefix ON cte_items (name_norm text_pattern_ops);
CREATE INDEX IF NOT EXISTS idx_cte_items_manufacturer_prefix ON cte_items (manufacturer_norm text_pattern_ops);
CREATE INDEX IF NOT EXISTS idx_cte_items_category_prefix ON cte_items (category_norm text_pattern_ops);
CREATE INDEX IF NOT EXISTS idx_cte_items_name_trgm ON cte_items USING GIN (name_norm gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_cte_items_category_trgm ON cte_items USING GIN (category_norm gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_cte_items_manufacturer_trgm ON cte_items USING GIN (manufacturer_norm gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_cte_items_search_trgm ON cte_items USING GIN (search_text gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_cte_items_search_tsv ON cte_items USING GIN (search_tsv);

CREATE INDEX IF NOT EXISTS idx_contract_items_cte_date ON contract_items (cte_id, contract_date DESC);
CREATE INDEX IF NOT EXISTS idx_contract_items_cte_customer_region_date ON contract_items (cte_id, customer_region, contract_date DESC);
CREATE INDEX IF NOT EXISTS idx_contract_items_cte_supplier_region_date ON contract_items (cte_id, supplier_region, contract_date DESC);
CREATE INDEX IF NOT EXISTS idx_contract_items_customer_region ON contract_items (customer_region, contract_date DESC);
CREATE INDEX IF NOT EXISTS idx_contract_items_supplier_region ON contract_items (supplier_region, contract_date DESC);
CREATE INDEX IF NOT EXISTS idx_contract_items_contract_date ON contract_items (contract_date DESC);
`

const resetImportSQL = `
DROP TABLE IF EXISTS doc_versions;
DROP TABLE IF EXISTS batch_doc_versions;
DROP TABLE IF EXISTS contract_items;
DROP TABLE IF EXISTS cte_items;
DROP TABLE IF EXISTS dataset_meta;
`
