-- V002 编号唯一粒度（决策 16，2026-09-08）
-- 规则：同一 name+model 内 equipment_no 唯一；不同设备允许同号（如各设备自身 001… 复用合法）。
-- 实现：部分唯一索引，仅约束 equipment_no IS NOT NULL 的行。

CREATE UNIQUE INDEX IF NOT EXISTS idx_equipment_name_model_no
    ON equipment(name, model, equipment_no)
    WHERE equipment_no IS NOT NULL;
