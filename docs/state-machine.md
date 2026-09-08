# 设备状态机 state-machine.md

> 文档编号：DSH-EQ-SM · 版本：v0.1.0-phase0 · 日期：2026-09-07
> 状态定义与白名单已按 `AGENTS.md` 决策基线 4 锁定。本文件为设计与实现契约。

---

## 1. 状态定义（六态 + 备用）

| 状态码 | 中文 | 含义 | 前端色点 |
|---|---|---|---|
| IN_STOCK | 在库 | 存放于仓库，无占用 | 🟢 |
| IN_TEAM | 班组使用 | 在某班组手中 | 🔵 |
| BORROWED | 外借 | 在外借方处（含内部厂，W-5 后视确认而定） | 🟠 |
| MAINTENANCE | 维修 | 送修中 | 🔴 |
| SCRAPPED | 报废 | 终态，不可再流转 | ⚫ |
| OTHER | 其他 | 预留，第一版 UI 不开放 | 🟣 |

## 2. 允许的流转（白名单，决策 4）

| 动作(action) | from | to | 必填字段 | 说明 |
|---|---|---|---|---|
| OUT_TO_TEAM 出库给班组 | IN_STOCK | IN_TEAM | to_team_id、operator、occurred_at | 仓库→班组；记录交付时间=occurred_at |
| RETURN_FROM_TEAM 班组归还入库 | IN_TEAM | IN_STOCK | operator、occurred_at | 班组→仓库 |
| HANDOVER 班组转交 | IN_TEAM | IN_TEAM | to_team_id(新班组)≠from_team_id、operator | 原班组→新班组；交付时间重置为新转交时间（决策 8） |
| BORROW 外借 | IN_STOCK / IN_TEAM | BORROWED | borrower_id、borrow_date、operator、（expected_return_date 可空） | 建 borrow_record(OUTSTANDING)（决策 1/2） |
| RETURN_BORROW 外借归还 | BORROWED | IN_STOCK | operator、（occurred_at=实际归还时间） | 回填 borrow_record.actual_return_date+RETURNED（决策 3/9） |
| TO_MAINTENANCE 送修 | IN_STOCK / IN_TEAM | MAINTENANCE | operator、备注 | 外借中/报废设备不可直接送修 |
| FROM_MAINTENANCE 维修完成 | MAINTENANCE | IN_STOCK | operator | 一律回仓库 |
| SCRAP 报废 | 除 SCRAPPED 外任意 | SCRAPPED | operator、备注(必填原因) | 终态；不可再流转 |
| IMPORT_INIT 导入固化 | —(初始) | =导入现状 | operator=系统导入 | 仅导入创建（决策 7），非用户操作 |
| CORRECT 受限更正 | 任意 | 不变 | reason 必填 | 更正设备字段/编号错录（决策 13），非状态流转，记 audit |

**禁止示例**：BORROWED→IN_TEAM（必须先归还回仓库）；MAINTENANCE→IN_TEAM（必须先维修完成回库再出库）；SCRAPPED→任何状态。

## 3. 状态转移图

```text
                      ┌────────────┐
        ┌────────────>│  IN_STOCK   │<──────────┐
        │ 归还入库      │   (在库)    │           │
        │             └─────┬──────┘           │
        │ 出库给班组          │                 │ 维修完成
        │                   │送修               │
        │                   ▼                  │
   ┌────┴──────┐  送修   ┌─────────┐      ┌────┴──────────┐
   │  IN_TEAM  ├────────>│MAINTEN- │─────>│               │
   │ (班组使用) │         │ ANCE维修 │      │               │
   └────┬──────┘         └─────────┘      │               │
        │ 转交(同态)                        │               │
        ▼  (IN_TEAM→IN_TEAM)               │               │
   ┌────────────┐        外借              │               │
   │  班组 B     │────┐ (IN_STOCK/IN_TEAM)  │               │
   └────────────┘    ▼                     │               │
                ┌──────────┐   归还      ┌──┴──────────────┐
                │ BORROWED │───────────>│                │
                │  (外借)   │            │ (回到 IN_STOCK) │
                └──────────┘            └─────────────────┘
   *任意状态──报废──> SCRAPPED(终态)
```

## 4. 一致性实现契约（核心）

任何一次流转 = **单个事务**内：

1. 校验：设备存在；from_status == 设备当前 status；动作在（当前 status × 目标动作）允许集合内；必填字段完整（班组存在且启用、外借方存在且启用等）。
2. 更新 `equipment`：status、current_team_id / current_borrower_id / current_borrow_record_id、**current_since=occurred_at**（决策 8）。
3. 追加 `transaction`：记录 from/to（含状态与班组/外借方）＋ **名称快照**（决策 14）＋ occurred_at/operator/remark。
4. 外借/归还联动 borrow_record（建单 OUTSTANDING / 回填 RETURNED）。
5. 失败整单回滚，返回统一错误（含业务原因，如"当前状态非外借，无法归还"）。

更新与追加必须在同一 DB 事务内完成 —— 禁止只改当前状态不写历史，反之亦然（章程原则三/铁律 4）。

## 5. 交付时间（current_since）口径（决策 8）

- 设备到达 IN_TEAM（出库或转交）→ current_since=该次 occurred_at；
- 到达 IN_STOCK（归还/维修完成/班组归还）→ current_since=该次时间；
- BORROWED → 外借时间；
- 详情页展示："当前位置 + 交付/发生时间 = current_since"；时间线展示 transaction 全量。

## 6. 界面动作映射

设备详情按钮与可用状态：

| 按钮 | 当前状态要求 |
|---|---|
| 转交 | IN_TEAM |
| 外借 | IN_STOCK / IN_TEAM |
| 归还 | BORROWED（外借归还） |
| 入库 | IN_TEAM（班组归还） |
| 送修 | IN_STOCK / IN_TEAM |
| 维修完成 | MAINTENANCE |
| 报废 | 任意非报废 |
| 受限更正 | 任意（弹窗填原因，记 audit） |

外借管理页：归还/延期按钮（borrow_record OUTSTANDING 时）。

## 7. 校验示例（测试锚点）

| 用例 | 期望 |
|---|---|
| 6061（IN_STOCK）执行 OUT_TO_TEAM(裁剪一组) | 成功；current_since=now；写记录 |
| 6061 再次 OUT_TO_TEAM | 拒绝（from≠IN_STOCK） |
| BORROWED 设备点"归还" | 回 IN_STOCK，外借单实际归还日回填 |
| BORROWED 设备点"转交" | 拒绝 |
| SCRAPPED 设备任何操作 | 拒绝（终态） |
| 归还时无未结外借单 | 拒绝并提示先建外借单 |
