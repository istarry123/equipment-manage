# 开发路线图 roadmap.md

> 文档编号：DSH-EQ-RD · 版本：v0.1.0-phase0 · 日期：2026-09-07
> 目标：从"当前状态"到 v1.0.0 的阶段路线、Phase 1 详细计划、风险登记与门禁。

---

## 1. 当前状态（2026-09-07）

- Git：`main`，基线提交 + 治理文档 + 决策基线 14 项（`AGENTS.md`）；Phase 0 六份文档待提交（本文档完成后）。
- 数据源：`设备借出总账.xlsx` 只读分析完毕（`docs/data-analysis.md`）。
- 决策基线：14 项锁定（含 Win7 SP1 兼容、类别字典、编号禁改、名称快照）。
- 待确认清单：W-1…W-15（其中 Blockers：W-1/2/3/4/5），**Phase 1 结束前须答复 W-1/2/3/4/5（至少 W-1/2）**，否则 Phase 2 导入无法放行。

## 2. 阶段总览（章程 §18/§20 + Git 版本建议）

| Phase | 目标 | 标签 |
|---|---|---|
| 0 | 需求与数据分析（本阶段） | v0.1.0-phase0 |
| 1 | 项目骨架：Go+SQLite+React 打通，migration，REST 框架，exe 可运行 | v0.2.0-phase1 |
| 2 | Excel 导入（Tier1 必须 + Tier2 按 W-2 决定） | v0.3.0-phase2 |
| 3 | 设备台账（列表/搜索/详情/新增/编辑/更正） | v0.4.0-phase3 |
| 4 | 设备流转全链路 + 历史时间线（最重要） | v0.5.0-phase4 |
| 5 | Dashboard/统计/导出/备份恢复 | v0.6.0-phase5 |
| 6 | Windows 打包（Win7+Win10/11 双回归）、使用说明 | v1.0.0 |

门禁：每 Phase 完成=测试通过→文档更新→commit+tag→阶段验收报告→**STOP 等用户指令**（Skill 铁律 2/10）。

## 3. Phase 1 详细计划（下一阶段）

### 3.1 环境与工具链（Win7 兼容约束前置）

| 项 | 动作 | 风险 |
|---|---|---|
| Go 工具链 | 交付构建锁定 **Go 1.20.x**（本机当前 1.26.5；开发可 1.26，发布构建切 1.20）→ 验证 `GOTOOLCHAIN`/双工具链脚本 | Go1.20 已 EOL（安全无更新）→ 记录风险，内网单机可接受 |
| 依赖矩阵 | 以 Go1.20 实际编译验证：gin、gorm、modernc.org/sqlite、excelize 的兼容版本并锁定（go.mod 精确版本） | 新版本库可能要求新 Go |
| 前端 | Vite/React/AntD/ECharts 版本锁定；构建 target 下调并按 Chrome109 实测 | AntD5 高版本可能超出 Chrome109 能力 → 版本/特性验证 |
| 浏览器自动打开 | exe 探测常见 chrome.exe 路径优先，兜底默认浏览器；Win7 引导安装随附 Chrome109 | IE 兜底不可用 → 必须处理 |

### 3.2 代码骨架（单体）

```text
equipment-manage/
├── cmd/equipment/main.go            # 入口：配置→日志→DB迁移→路由→启动→开浏览器
├── internal/
│   ├── config/   (config.yaml 读取：端口/备份保留数)
│   ├── db/       (sqlite 连接、migration 执行器 user_version)
│   ├── migrations/ (V001_init.sql ...)
│   ├── models/   (GORM 模型：见 database-design.md)
│   ├── api/      (gin 路由+handler，统一错误格式)
│   ├── service/  (业务层：状态机校验、事务)
│   └── backup/   (备份/恢复)
├── web/          (React+TS+Vite；go:embed 注入)
├── docs/  AGENTS.md  .agents/skills/  .gitignore  go.mod ...
└── equipment.db  backup/  logs/   (运行时生成，gitignore)
```

### 3.3 里程碑

1. M1 目录+配置+日志：`go build` 出 exe，启动输出日志。
2. M2 SQLite + migration：空库启动自动建表（user_version=1）；测试建表/重复启动幂等。
3. M3 REST 骨架：`/api/health`、统一错误体；示例设备 CRUD（含事务）跑通。
4. M4 前端骨架：Vite 空 Dashboard 通过 8080 由 Go 服务静态资源提供（go:embed）→ 前后端通信打通（fetch /api/health 展示）。
5. M5 基础测试：Go 单测（migration、health、示例 service）+ 前端类型检查/构建。
6. M6 兼容验证：Go1.20 交叉构建 amd64；产物在 Win10 运行冒烟；Chrome109 打开前端冒烟（可用 Win7 虚拟机者实测，无则 Win10+旧版内核标记延期）。

### 3.4 Phase 1 验收

- [x] 双击 exe 自动打开 8080 并显示骨架页（实现：EQ_NO_BROWSER=1 可禁用；自动打开见 internal/browser）
- [x] 空库自动迁移成功、重复启动幂等（user_version 迁移器 + 单测）
- [x] 依赖矩阵在 Go1.20 下 `go build` 通过（GOTOOLCHAIN=go1.20.14 实测产出 exe）
- [x] 前端构建产物经 go:embed 内嵌生效（/api/health、SPA 回退均已冒烟）
- [x] 单测通过（`go test ./...`）
- [x] Git tag v0.2.0-phase1 + 阶段报告

**Phase 1 实现备注**：
- 流转历史概念表 transaction 因 SQLite 保留关键字，物理表名定为 `flow_record`（models.TableName 与迁移 SQL 一致，docs/database-design.md §2.6 已注明）。
- GORM 官方 `gorm.io/driver/sqlite` 依赖 CGO 的 mattn 驱动，无法满足 `CGO_ENABLED=0`；采用纯 Go dialector `github.com/glebarez/sqlite`（modernc 内核）。
- Go 依赖经 goproxy.cn 拉取；工具链 go1.20.14 由 GOTOOLCHAIN 自动下载验证。

## 4. 后续阶段要点（衔接）

- **Phase 2**：实现 import-rules（Tier1 先行）；提供导入预览与报告；导入后台账可见。前置：W-1/W-2。
- **Phase 3**：台账 UI（AntD Table/Form）；搜索=编号/名称/型号 contains；详情含 current_since 交付时间；新增/编辑/受限更正（audit）。
- **Phase 4**：service 状态机事务 + 8 动作 API + 外借单 + 时间线组件；全状态机用例测试（state-machine §7 锚点）。
- **Phase 5**：Dashboard ECharts、逾期高亮、导出、备份/恢复 API+UI。
- **Phase 6**：安装包目录、Chrome109 随附、Win7 实机回归清单、使用说明（含"哪些在页面改、哪些要升级"章节）。

## 5. 风险登记册

| 风险 | 等级 | 缓解 |
|---|---|---|
| Go1.20 EOL 无安全补丁 | 中 | 内网单机+无外联；文档明示；如未来升级需重建兼容基线 |
| 源数据质量问题（重复/缺日/O0/无归还） | 高 | W 清单门禁 + 导入 BLOCK 不放行 + 报告可导出 |
| 编号非全局唯一（短序号跨块） | 高 | W-3/W-4 确认唯一粒度；Service 层校验提示 |
| "当前在借"无法从 Excel 判定 | 高 | W-2 方案 A（默认全在库+手动补录） |
| Win7 浏览器上限 Chrome109 影响 AntD/ECharts | 中 | 构建 target 下调 + Phase1/3/5 各做一次兼容冒烟 |
| 纯 Go SQLite(modernc) 与 GORM 兼容性 | 低 | Go1.20 依赖矩阵实测（M2） |
| go:embed + 前端 hash 资源缓存 | 低 | 标准 embed 方案，刷新即新版本 |
| 误导入/误恢复 | 中 | 危险操作二次确认 + 恢复前自动备份 + audit |

## 7. Phase 2 完成情况（2026-09-08，tag v0.3.0-phase2）

**Blockers 已全部答复（决策 15/16）**：W-1（类别原样保留+其他）、W-2（方案 A 全在库）、W-3/W-4（同 name+model 内唯一、V002 部分唯一索引）、W-5（双发系=team）。W-6…W-15 按表内默认处理（见 import-rules §8）。

**真实 Excel 全链路指标**（`设备借出总账.xlsx`，E2E 冒烟与单测一致）：

| 指标 | 值 | 说明 |
|---|---|---|
| 源台账数量 | 2147 | D 合计 |
| 分组 | 198 | 同 name+model |
| 成功导入 | 1477 | 设备实体 + 逐台 IMPORT_INIT 初始流转 |
| 跳过（BLOCK） | 670 台 / 2 组 | 平车 DDL-9000B、DDL-9000S（疑似重复编号，21 次多录）→ 需用户人工处理 |
| 无编号 | 567 | 含 R16 描述文本 1 台（入备注） |
| 类别 | 8 | 裁剪/缝纫/包装/技术/买华诺/买发网厂/买针织厂/其他 |
| 内部码 | EQ-000001 ~ EQ-001477 | |

**实现要点**：
- 解析基于 excelize（Go1.20 兼容 v2.7.1），合并单元格锚点感知；数量 D/E 与编号 F 只读"自有单元格"，避免合并回填重复统计。
- 校验 V1/V2/V3/V5/V6/V10 落地为 BLOCK/WARN；同组重复编号=BLOCK 分组并给出清单（决策 16）。
- 导入为单事务 + 空库前置守卫（ErrDBNotEmpty）+ audit_log(IMPORT) 留痕；危险操作经前端 Modal 二次确认。
- 时间列 TEXT + models.Time/NullTime（Scanner/Valuer），统一 RFC3339Nano 存取。
- API：`POST /api/import/parse`、`POST /api/import/run`、`GET /api/equipment`（基础列表，供导入结果验证；完整台账在 Phase 3）。
- 前端「数据导入」页：选文件→摘要/问题表→确认导入（Modal）→报告 + 结果列表。

**用户需处理的唯一导入遗留**：平车 DDL-9000B / DDL-9000S 两组重复编号（如 11472、2003、2017、3263–3274 等）——若确为"两台同号机"请在 Excel 中为其中一台补记区分标记/真实编号；若为录入重复请删多余项；随后在空库重新导入（系统会自动带出完整报告）。

## 8. 后续 Phase 提示（从 Phase 3 起）

- Phase 3 前用户无需再答复 W 项（均按默认/已定执行）。
- 唯一持续待办：上文"重复编号"数据清理（不阻塞其他开发）。

## 9. Phase 3 完成情况（2026-09-08，tag v0.4.0-phase3）

**设备台账**（前端「设备台账」导航已启用）：
- 列表：编号/名称/型号/内部码/类别/状态(中文 Tag)/当前位置(在库→仓库、班组→班组名、外借→外借方)/到达时间；关键字 + 类别/状态/班组筛选 + 服务端分页。
- 详情：完整信息（含当前班组/外借方/到达时间/创建更新）。
- 新增设备（默认在库 + 初始流转；编号可空=无编号设备，内部码 EQ-xxxxx 自动分配）。
- 编辑基本信息（名称/型号/类别/备注；**编号不可直接修改**）。
- 受限更正（决策 13）：更正编号等字段须填原因，写 `audit_log(CORRECT)`；无原因 400、同组重复编号 409。
- 字典 API：`/api/categories`（增/查）、`/api/teams`（查）。
- 业务规则置于 `internal/service`（唯一性=同 name+model 内 equipment_no 唯一，决策 16；类别存在性校验）。
- 冒烟 17 项断言全过（搜 `6061` 返回"环形割刀/裁剪设备/在库/到达时间"）。

**修复记录**：API 入参 JSON tag 缺失（equipment_no/category_id 下划线字段未绑定）在冒烟中发现并修复，回归单测覆盖。
**遗留**：Phase 2 平车两组重复编号数据清理仍待用户处理（不影响台账功能）。

## 10. Phase 4 完成情况（2026-09-08，tag v0.5.0-phase4）

**设备流转（全项目核心）**：八类动作全部落地，遵循 state-machine.md 白名单与铁律 4：
- 出库给班组 / 班组归还入库 / 班组转交 / 外借（建 borrow_record，可填预计归还）/ 外借归还（回仓库，决策 3）/ 送修 / 维修完成 / 报废（须原因，终态）。
- 每次流转 = 单事务内"更新 equipment 当前状态（current_since）+ 追加 transaction（含班组/外借方名称快照，决策 14）+ 外借单联动"。
- 设备详情（Drawer）按当前状态提供可用按钮 + **完整流转时间线**（Timeline，最近在前，含操作人/备注）。
- 外借管理页：外借中/已逾期(标红+天数)/已归还筛选、归还、延期（改预计归还日期）；外借方（公司）增改停用。
- 班组/内部单位管理页：增改/停用（历史引用保留）。
- 非法边全部校验并给出可读错误（如外借中不可送修、报废终态不可再流转、同组不可转交）。
- 单测：完整链条（出库→转交→外借→归还→送修→维修完成→报废）与 8 组非法动作/必填校验；冒烟 23 断言全过（HTTP 全链路 + 时间线 8 条 + 外借单回填）。
- 修复记录：归还时设备当前占位未正确清除（service 单测发现）→ 修正快照赋值逻辑。

**API**：`POST /api/equipment/:id/flow`、`GET /api/equipment/:id/transactions`、`GET /api/borrows`、`POST /api/borrows/:id/return|extend`、班组/外借方 CRUD。
**下一阶段**：Phase 5（Dashboard/统计/备份恢复/导出）。

## 11. Phase 5 完成情况（2026-09-08，tag v0.6.0-phase5）

- **Dashboard**：设备总数/六状态卡片/逾期外借数；ECharts 状态分布（环形）、各班组与各类别设备数（条形）；最近流转、当前外借（逾期标红）列表。
- **Excel 导出**：`GET /api/export/equipment`（筛选与台账一致）输出 .xlsx（设备编号/名称/型号/类别/状态/当前位置/到达时间/内部码/备注/更新时间）。
- **备份**：`backup/` 目录；**启动自动备份** + 手动备份（VACUUM INTO 一致性快照，WAL 安全）；默认保留最近 30 份（可修剪）；同秒冲突自动加序号。
- **恢复**：危险操作需前端输入 RESTORE 确认；**恢复前自动备份当前库**（安全快照）→ 关连接 → 原子替换 → 重开并迁移 → 审计留痕；恢复后可回退（从快照再恢复）。
- **系统设置页**：类别字典新增/列表 + 系统信息与运行边界提示。
- 验证：service 单测（备份/修剪/防目录穿越/导出/统计）全绿；HTTP 冒烟全过（统计口径、导出 xlsx 魔数、手动备份、未确认恢复被拒、恢复 3→2 回滚、安全备份保留）；`go test ./...` 全绿。
- 修复记录：恢复安全备份前缀校验过严（仅接受 equipment-*/pre-restore-*）→ 放宽为 backup/ 内任意 .db 且保留防穿越校验。

**剩余（下一 Phase 6）**：Windows 打包（Win7+Win10 双环境回归、Chrome109 随附、自动开浏览器、使用说明）→ v1.0.0。

## 12. Phase 6 完成情况（2026-09-08，tag v1.0.0）

- 版本号统一 **v1.0.0**（后端 version + 前端标识 + 日志）。
- `scripts/build-release.ps1`：前端构建 → `GOTOOLCHAIN=go1.20.14` + `CGO_ENABLED=0` 编译 → 组装 `release/equipment/`（exe/config/用户手册/backup/logs/install）。产物 27.9MB。
- `docs/user-guide.md`：用户手册（运行环境/Win7 浏览器、首次 Excel 导入、页面功能、数据与备份、FAQ：端口占用/白屏强刷、防火墙、卸载导出）。
- Chrome 109 离线包：本环境无法访问 dl.google.com，**未实机打包**；已在手册/交付说明给出获取 URL 与放 `install/` 的步骤（联网机器一次即可）。
- 产物冒烟：健康检查 `version=1.0.0`、首页与资源加载、空库 Dashboard（6 状态 + 空数组非 null）全过；`go test ./...` 全绿。
- 交付待办（需在目标电脑完成，见最终验收报告）：Win7 实机回归、Chrome109/FF115 安装、首次导入真实 Excel、重复编号数据清理。

## 13. v1.0.0 功能增量：班组设备视图（2026-09-08，feat commit）

- 页面「班组设备」（导航于 设备台账 与 设备流转 之间）：三级结构 班组→设备类别→设备；顶部统计（班组数量/设备总数/班组使用设备/未分配设备）与筛选（班组/类别/状态/关键字 查询·重置）。
- 数据原则：复用 `equipment.current_team_id/status/category_id`，未新增任何关系表；仅 `IN_TEAM` 且归属班组计入班组分组；`current_team_id IS NULL`（在库/外借/维修）进入「未分配设备」；外借设备带状态标签不误入班组。
- API：`GET /api/teams/equipment`（team/category/status/q），单次查询取数避免 N+1，编号按自然序（数字段数值比较）排序。
- 复用：设备编号点击 → 既有设备详情抽屉（App 级跨页打开）；Dashboard 新增「各班组设备使用情况」矩阵 + 跳转，与台账/页面统计同源一致（三方一致验证 A班=4）。
- 测试：Case1-7 全过（班组/类别筛选/关键字/未分配/外借不计入/流转迁移/历史不受影响）+ API 契约测试 + 无头 UI 验收（零 console 错误、编号点击进详情）。

## 14. v1.0.0 功能增量：班组/类别受控删除（2026-09-08，feat commit）

- 班组管理、系统设置-类别字典新增「删除」（Popconfirm 二次确认）。
- 规则（决策 17）：班组仅当「无设备当前占用 current_team_id」且「无历史流转引用 from/to_team_id」才可物理删除（可清建错空项）；类别仅当「无设备引用 category_id」才可删除；被引用返回 409 并提示改用停用/先调整。
- API：`DELETE /api/teams/:id`、`DELETE /api/categories/:id`；错误统一 JSON（409/404）。
- 测试：service 用例（未引用可删/占用拒绝/历史引用拒绝/归还后仍拒绝）+ API 契约（409/200/404）全绿。
