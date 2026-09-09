-- V004 导入批次与来源追踪（v1.1，决策 18）
-- 目标：导入来源可追踪（§二十五）、重复导入幂等（§二十六）、
--       后续 Reconciliation（Phase 11）可依据 source key 对账。
-- 变更：
--   1) 新增 import_batch 表：一次文件导入的批次记录（source hash 唯一 → 幂等依据）；
--   2) equipment 新增 import_batch_id（该设备来自哪个批次，可空=手工/历史数据）与
--      source_key（来源定位，如 R141#N3，非身份，仅追溯用）。

CREATE TABLE import_batch (
    id             INTEGER PRIMARY KEY,
    source_name    TEXT    NOT NULL DEFAULT '',
    source_hash    TEXT    NOT NULL DEFAULT '',
    status         TEXT    NOT NULL DEFAULT 'DONE',
    total_rows     INTEGER NOT NULL DEFAULT 0,
    imported_count INTEGER NOT NULL DEFAULT 0,
    warning_count  INTEGER NOT NULL DEFAULT 0,
    error_count    INTEGER NOT NULL DEFAULT 0,
    created_at     TEXT    NOT NULL,
    updated_at     TEXT    NOT NULL
);

CREATE UNIQUE INDEX idx_import_batch_hash ON import_batch(source_hash);

ALTER TABLE equipment ADD COLUMN import_batch_id INTEGER;
ALTER TABLE equipment ADD COLUMN source_key TEXT NOT NULL DEFAULT '';

CREATE INDEX idx_equipment_batch ON equipment(import_batch_id);
