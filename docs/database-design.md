# 数据库设计 database-design.md

> 文档编号：DSH-EQ-DB · 版本：v0.1.0-phase0 · 日期：2026-09-07
> 依据：主章程 §7、`AGENTS.md` 决策基线（14 项）、`docs/data-analysis.md`。
> 存储：SQLite（`equipment.db`），单文件；GORM v2 + 纯 Go 驱动（`modernc.org/sqlite`，CGO 关闭）。
> 迁移：不使用 AutoMigrate 当唯一手段，采用**版本化迁移**（SQLite `PRAGMA user_version` + 有序 SQL 脚本，启动时在事务内执行）。

---

## 1. ER 总览

```text
category ─1:N─ equipment ─1:N─ transaction
team     ─1:N─ equipment(current_team_id) / transaction(from/to_team)
borrower ─1:N─ borrow_record ─1:N─ equipment(通过 borrow_record.equipment_id)
borrow_record ─1:1─ 一次外借生命周期；归还时 transaction 追加一条并关联 borrow_record
equipment 变更留痕 → audit_log（受限更正、敏感操作）
settings(key/value) → 系统参数
```

设计要点（对齐决策基线）：

- **equipment 保存"当前状态快照"**（快速回答 8 问）；**transaction 保存全部历史**；二者分离。
- **编号身份**：`id` 为主键；`equipment_no` 为外部真实编号（文本），`internal_code` 为系统内部码（`EQ-000001` 式，仅无编号设备生成）。
- **名称快照**（决策 14）：transaction 冗余流转当时的班组名/外借方名，杜绝改名导致历史漂移。
- **编号唯一性**（W-3/W-4 未决前的默认）：建**普通索引**而非全局唯一约束，唯一性校验逻辑放导入/新增服务层并给出可读错误 —— 待用户确认 W-3/W-4 后再决定是否收紧为 UNIQUE。
- **删除策略**（决策 6）：无 DELETE 能力；班组/外借方用 `is_active` 停用；equipment 无软删字段（无删除入口，报废即终态）。

---

## 2. 表结构

### 2.1 category（类别字典，决策 12）

| 字段 | 类型 | 说明 |
|---|---|---|
| id | INTEGER PK AUTOINCREMENT | |
| name | TEXT NOT NULL UNIQUE | 类别名（裁剪设备/缝纫设备…） |
| sort | INTEGER DEFAULT 0 | 排序 |
| remark | TEXT NULL | |
| created_at / updated_at | DATETIME | |

### 2.2 equipment（设备主表）

| 字段 | 类型 | 说明 |
|---|---|---|
| id | INTEGER PK | 系统主键（物理身份） |
| equipment_no | TEXT NULL | 真实设备编号（可空=无编号设备） |
| internal_code | TEXT NOT NULL UNIQUE | 系统内部码：无编号→`EQ-xxxxx`；有编号→可直接用编号或亦生成 EQ 码（取决策：仅无编号生成，有编号=编号） |
| name | TEXT NOT NULL | 名称（保留原文） |
| model | TEXT NULL | 型号 |
| category_id | INTEGER NULL FK→category | 类别（决策 12） |
| status | TEXT NOT NULL | 状态码：IN_STOCK/IN_TEAM/BORROWED/MAINTENANCE/SCRAPPED/OTHER |
| current_team_id | INTEGER NULL FK→team | 当前班组（IN_TEAM 时非空） |
| current_borrower_id | INTEGER NULL FK→borrower | 当前外借方（BORROWED 时非空，冗余便于快速查询） |
| current_borrow_record_id | INTEGER NULL FK→borrow_record | 当前未归还外借单 id（BORROWED 时非空） |
| current_since | DATETIME NULL | **到达当前状态的时间**（决策 8"交付时间"的数据来源；状态变更事务内同步） |
| remark | TEXT NULL | |
| created_at / updated_at | DATETIME | |

索引：`equipment_no`、`status`、`category_id`、`current_team_id`、`current_borrower_id`、`name/model`（配合台账模糊检索）。

### 2.3 team（班组，决策 5/6/14）

| 字段 | 类型 | 说明 |
|---|---|---|
| id | INTEGER PK | |
| name | TEXT NOT NULL UNIQUE | |
| department | TEXT NULL | 预留（Excel 无班组数据） |
| is_active | INTEGER DEFAULT 1 | 停用=0；历史引用不清除 |
| remark | TEXT NULL | |
| created_at / updated_at | DATETIME | |

### 2.4 borrower（外借方）

| 字段 | 类型 | 说明 |
|---|---|---|
| id | INTEGER PK | |
| name | TEXT NOT NULL | 公司名（含"双发分厂（华锦）"式内部厂，W-5 确认后再定是否与 team 打通） |
| contact / phone | TEXT NULL | 联系人/电话 |
| is_active | INTEGER DEFAULT 1 | 停用=0 |
| remark | TEXT NULL | |
| created_at / updated_at | DATETIME | |

唯一约束：`name` UNIQUE（导入遇重名自动复用）。

### 2.5 borrow_record（外借单，决策 1/2/3/9）

| 字段 | 类型 | 说明 |
|---|---|---|
| id | INTEGER PK | |
| equipment_id | INTEGER NOT NULL FK→equipment | 一设备一单 |
| borrower_id | INTEGER NOT NULL FK→borrower | |
| borrow_date | DATETIME NOT NULL | 外借日期 |
| expected_return_date | DATETIME NULL | 预计归还日期（可空：未约定） |
| actual_return_date | DATETIME NULL | 实际归还日期（归还时回填） |
| status | TEXT NOT NULL DEFAULT 'OUTSTANDING' | OUTSTANDING / RETURNED |
| remark | TEXT NULL | 含导入注释原文 |
| created_at / updated_at | DATETIME | |

- 逾期 = `status=OUTSTANDING AND expected_return_date < today`（派生，不落列；归还时若 actual>expected 亦可在界面上提示"曾逾期 N 天"，即时计算）。
- 归还：`actual_return_date=now,status=RETURNED` ＋ 追加 `transaction(action=RETURN_BORROW)` ＋ equipment 回 IN_STOCK（决策 3）——同事务。

### 2.6 transaction（流转历史，**永久保存、禁止删除**）

| 字段 | 类型 | 说明 |
|---|---|---|
| id | INTEGER PK | |
| equipment_id | INTEGER NOT NULL FK→equipment | |
| action | TEXT NOT NULL | IMPORT_INIT / OUT_TO_TEAM / RETURN_FROM_TEAM / HANDOVER / BORROW / RETURN_BORROW / TO_MAINTENANCE / FROM_MAINTENANCE / SCRAP / CORRECT(更正) |
| from_status / to_status | TEXT NOT NULL | 状态机边 |
| from_team_id / to_team_id | INTEGER NULL FK→team | |
| team_name_snapshot | TEXT NULL | **决策 14**：流转当时班组名（from/to 各自快照或合并单列，实现取 `from_team_name`/`to_team_name` 两列） |
| borrower_id | INTEGER NULL FK→borrower | |
| borrower_name_snapshot | TEXT NULL | **决策 14**：外借方名快照 |
| borrow_record_id | INTEGER NULL FK→borrow_record | 外借/归还关联外借单 |
| occurred_at | DATETIME NOT NULL | 业务发生时间（交付时间口径的基础） |
| operator | TEXT NOT NULL | 操作人（导入=固定"系统导入"，决策 W-6） |
| remark | TEXT NULL | |
| created_at | DATETIME | 记录写入时间 |

索引：`(equipment_id, occurred_at DESC)`、`action`、`borrow_record_id`。
补充说明：import 初始现状记录 `IMPORT_INIT` 的 from 语义（无历史）→ `from_status` 存 `-`（初始）并加说明；纠正类更正事件归属 `CORRECT` 并关联 audit_log（决策 13）。

### 2.7 audit_log（操作留痕，决策 13 + 数据安全）

| 字段 | 类型 | 说明 |
|---|---|---|
| id | INTEGER PK | |
| action | TEXT | CORRECT / IMPORT / BACKUP / RESTORE / OVERWRITE 等敏感操作 |
| target | TEXT | 对象（table:record_id） |
| detail | TEXT | 变更说明（old→new） |
| reason | TEXT | 受限更正必填原因 |
| operator | TEXT | |
| created_at | DATETIME | |

### 2.8 settings（系统设置）

| 字段 | 类型 | 说明 |
|---|---|---|
| key | TEXT PK | 如 backup_keep_count、port |
| value | TEXT | |

---

## 3. 外借建模决策的落库映射（决策 1/2/3 复核）

1. Excel 一条"借出编号 6061 6062、台数 2"→ **两条 borrow_record**（同一 borrower/日期），逐台归还（决策 2）。
2. 外借动作：`IN_STOCK/IN_TEAM → BORROWED`（决策 4 白名单），equipment 记 current_borrower_id/current_borrow_record_id/current_since。
3. 归还：`BORROWED → IN_STOCK`，回填外借单。
4. 无编号批量外借（J=无编号，W-8）：无法按编号建单 → 待 W-8 确认（可能以"批次外借"挂设备组或以备注处理，Phase 0 不落表）。

---

## 4. 关键查询的落库支撑（对照"8 问"）

| 问题 | 实现路径 |
|---|---|
| 6061 在哪 | equipment(current_team_id/borrower/status) 直接取 |
| 什么时候给的 | `current_since`（决策 8） |
| 以前在哪 | transaction 时间线（倒序，含名称快照） |
| 在库/某班组/外借 | status + 筛选 |
| 借给谁/借多久/逾期 | borrow_record 列表 + expected/actual |

---

## 5. 数据安全与一致性

- 状态变更、外借创建/归还 = **一个 SQLite 事务**内完成"更新 equipment + 追加 transaction（+更新 borrow_record）"；任何一步失败整体回滚。
- transaction/audit_log 无 UPDATE/DELETE 入口（API 层不提供）。
- 危险 API（恢复、覆盖导入、清空）在 Service 层做确认参数 + audit 留痕。
- 编号唯一性在 Service 层校验（W-3/W-4 结论更新前用非唯一索引 + 强校验错误提示）。

---

## 6. 迁移策略

- `PRAGMA user_version` 记录 schema 版本；`migrations/` 下按序 SQL（V001_init.sql …）。
- 启动时：读 user_version → 依次执行 > 当前版本的脚本（事务）→ 更新 user_version。
- v1 首版即包含上述全部表与索引（V001）。后续加字段一律新增 V00x 脚本，不改历史脚本。
