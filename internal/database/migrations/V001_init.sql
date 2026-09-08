-- V001 初始 schema
-- 设计依据：docs/database-design.md
-- 约定：DATETIME 列存本地时间；流转历史禁止删除；FK 一律 RESTRICT 语义。

-- ============ 基础字典（无外键依赖，先建） ============

CREATE TABLE category (
    id          INTEGER PRIMARY KEY,
    name        TEXT    NOT NULL UNIQUE,
    sort        INTEGER NOT NULL DEFAULT 0,
    remark      TEXT    NOT NULL DEFAULT '',
    created_at  TEXT    NOT NULL,
    updated_at  TEXT    NOT NULL
);

CREATE TABLE team (
    id          INTEGER PRIMARY KEY,
    name        TEXT    NOT NULL UNIQUE,
    department  TEXT    NOT NULL DEFAULT '',
    is_active   INTEGER NOT NULL DEFAULT 1,
    remark      TEXT    NOT NULL DEFAULT '',
    created_at  TEXT    NOT NULL,
    updated_at  TEXT    NOT NULL
);

CREATE TABLE borrower (
    id          INTEGER PRIMARY KEY,
    name        TEXT    NOT NULL UNIQUE,
    contact     TEXT    NOT NULL DEFAULT '',
    phone       TEXT    NOT NULL DEFAULT '',
    is_active   INTEGER NOT NULL DEFAULT 1,
    remark      TEXT    NOT NULL DEFAULT '',
    created_at  TEXT    NOT NULL,
    updated_at  TEXT    NOT NULL
);

CREATE TABLE settings (
    key         TEXT PRIMARY KEY,
    value       TEXT    NOT NULL DEFAULT ''
);

CREATE TABLE audit_log (
    id          INTEGER PRIMARY KEY,
    action      TEXT    NOT NULL,
    target      TEXT    NOT NULL DEFAULT '',
    detail      TEXT    NOT NULL DEFAULT '',
    reason      TEXT    NOT NULL DEFAULT '',
    operator    TEXT    NOT NULL DEFAULT '',
    created_at  TEXT    NOT NULL
);

-- ============ 设备主表 ============

CREATE TABLE equipment (
    id                       INTEGER PRIMARY KEY,
    equipment_no             TEXT,               -- 真实编号；无编号设备为 NULL
    internal_code            TEXT    NOT NULL UNIQUE, -- EQ-xxxxx
    name                     TEXT    NOT NULL,
    model                    TEXT    NOT NULL DEFAULT '',
    category_id              INTEGER REFERENCES category(id),
    status                   TEXT    NOT NULL,
    current_team_id          INTEGER REFERENCES team(id),
    current_borrower_id      INTEGER REFERENCES borrower(id),
    current_borrow_record_id INTEGER REFERENCES borrow_record(id), -- 建表顺序见下：此处先声明，随后补建 borrow_record（SQLite 允许后置目标表）
    current_since            TEXT,               -- 到达当前状态时间
    remark                   TEXT    NOT NULL DEFAULT '',
    created_at               TEXT    NOT NULL,
    updated_at               TEXT    NOT NULL
);

-- ============ 外借单 ============

CREATE TABLE borrow_record (
    id                   INTEGER PRIMARY KEY,
    equipment_id         INTEGER NOT NULL REFERENCES equipment(id),
    borrower_id          INTEGER NOT NULL REFERENCES borrower(id),
    borrow_date          TEXT    NOT NULL,
    expected_return_date TEXT,
    actual_return_date   TEXT,
    status               TEXT    NOT NULL DEFAULT 'OUTSTANDING',
    remark               TEXT    NOT NULL DEFAULT '',
    created_at           TEXT    NOT NULL,
    updated_at           TEXT    NOT NULL
);

-- ============ 流转历史（永久保存，禁止删除） ============
-- 说明：概念名为 transaction；物理表名 flow_record 以避开 SQLite 保留关键字 TRANSACTION。

CREATE TABLE flow_record (
    id                  INTEGER PRIMARY KEY,
    equipment_id        INTEGER NOT NULL REFERENCES equipment(id),
    action              TEXT    NOT NULL,
    from_status         TEXT    NOT NULL,
    to_status           TEXT    NOT NULL,
    from_team_id        INTEGER REFERENCES team(id),
    to_team_id          INTEGER REFERENCES team(id),
    from_team_name      TEXT    NOT NULL DEFAULT '',  -- 名称快照
    to_team_name        TEXT    NOT NULL DEFAULT '',  -- 名称快照
    borrower_id         INTEGER REFERENCES borrower(id),
    borrower_name       TEXT    NOT NULL DEFAULT '',  -- 名称快照
    borrow_record_id    INTEGER REFERENCES borrow_record(id),
    occurred_at         TEXT    NOT NULL,
    operator            TEXT    NOT NULL,
    remark              TEXT    NOT NULL DEFAULT '',
    created_at          TEXT    NOT NULL
);

-- ============ 索引 ============

CREATE INDEX idx_equipment_no        ON equipment(equipment_no);
CREATE INDEX idx_equipment_name      ON equipment(name);
CREATE INDEX idx_equipment_category  ON equipment(category_id);
CREATE INDEX idx_equipment_status    ON equipment(status);
CREATE INDEX idx_equipment_team      ON equipment(current_team_id);
CREATE INDEX idx_equipment_borrower  ON equipment(current_borrower_id);

CREATE INDEX idx_borrow_equipment    ON borrow_record(equipment_id);
CREATE INDEX idx_borrow_borrower     ON borrow_record(borrower_id);
CREATE INDEX idx_borrow_status       ON borrow_record(status);

CREATE INDEX idx_txn_equipment_time  ON flow_record(equipment_id, occurred_at DESC);
CREATE INDEX idx_txn_action          ON flow_record(action);
CREATE INDEX idx_txn_borrow_record   ON flow_record(borrow_record_id);

CREATE INDEX idx_audit_action        ON audit_log(action);
