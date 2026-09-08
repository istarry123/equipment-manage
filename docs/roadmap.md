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

- [ ] 双击 exe 自动打开 8080 并显示骨架页
- [ ] 空库自动迁移成功、重复启动幂等
- [ ] 依赖矩阵在 Go1.20 下 `go build` 通过
- [ ] 前端构建产物经 go:embed 内嵌生效
- [ ] 单测通过（`go test ./...`）
- [ ] Git tag v0.2.0-phase1 + 阶段报告

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

## 6. Phase 1 前需用户完成

1. 答复 Blockers：W-1（类别）、W-2（当前在借导入策略）——影响 Phase 2 设计，可在 Phase 1 期间并行确认；
2. W-3/W-4/W-5 可延至 Phase 2 前；
3. 确认 Phase 1 开始（本阶段 STOP 后等待指令）。
