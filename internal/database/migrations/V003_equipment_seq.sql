-- V003 设备身份重构（v1.1，决策 18）
-- 目标：equipment_no 不再是任何唯一性约束的一部分；同 (name,model,equipment_no) 允许存在多台真机。
-- 变更：
--   1) equipment 新增 equipment_seq（组内展示序号，NOT NULL 默认 0；真实身份仍为 id）；
--   2) 删除 V002 的部分唯一索引 idx_equipment_name_model_no（同号多台放行）；
--   3) equipment_no 普通索引 idx_equipment_no 已在 V001 建立，保持不变。

ALTER TABLE equipment ADD COLUMN equipment_seq INTEGER NOT NULL DEFAULT 0;

DROP INDEX IF EXISTS idx_equipment_name_model_no;
