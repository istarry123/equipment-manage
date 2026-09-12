# AGENTS.md · 设备资产与流转管理系统 — 项目治理约定

> 本文件对本仓库内所有开发 Agent（DSH / Codex 等）生效。
> 执行任何任务前，先阅读本文件与下方内嵌的两个 Skill。
> 记录日期：2026-09-07（需求澄清阶段）。

---

## 1. 本项目的两个治理 Skill（唯一权威副本，已内嵌于项目）

| Skill | 路径 | 管辖内容 |
|---|---|---|
| engineering-persona | `.agents/skills/engineering-persona/SKILL.md` | 工程师工作人格与纪律：优先级、不猜测原则、先理解再修改、小步开发、数据安全、STOP 规则 |
| equipment-management | `.agents/skills/equipment-management/SKILL.md` | 领域开发流程：技术栈、架构约束、数据模型、设备状态机、Excel 导入原则、Phase 0–6 制度与验收 |

**本项目所有开发工作一律遵循以上两个 Skill 执行。**

规则冲突时优先级：`equipment-management` 的强制约束（§32）> `engineering-persona` 的原则 > 其他文档描述。

## 2. 主章程文档

`设备资产与流转管理系统——DSH-Codex 总体开发 Prompt.md`

—— 详细需求、页面设计、验收标准与 Phase 定义以此为准。

## 3. 执行铁律（速查）

1. **不跨 Phase**：严格按 Phase 0 → 1 → 2 → 3 → 4 → 5 → 6 推进，禁止一次性生成整个项目。
2. 每个 Phase 循环：分析 → 实现 → 测试 → 修复 → 更新文档 → Git commit（+tag）→ 阶段报告 → **STOP 等待验收**。
3. 上一 Phase 未验收，不进入下一 Phase；即使下一阶段很容易。
4. 任何状态变化 = 更新 equipment 当前状态 + 新增 transaction，二者同一事务完成。
5. 流转历史（transaction）**禁止删除**；删除设备/记录、覆盖、清空、恢复、导入等危险操作必须有确认机制。
6. Excel（`设备借出总账.xlsx`）只作为**一次性初始化数据源**；正式数据只存 SQLite（`equipment.db`）。
7. **不猜测**：字段含义、业务规则无法确定时一律标记「待确认」并交用户确认，绝不擅自定夺。
8. 不引入章程之外的任何技术：无 Docker / PostgreSQL / Redis / K8s / 微服务 / 云服务 / 互联网依赖。
9. 提交信息遵循 Conventional Commits；阶段版本标签 `v0.1.0-phase0` … `v1.0.0`，决策 18 重构发布标签 `v1.1.0`（2026-09-09，用户确认）。
10. 每阶段完成输出验收报告并停止，等待用户明确指令再进入下一阶段。

## 4. 技术栈（已锁定）

- 后端：Go + Gin + SQLite + GORM + go:embed（内嵌前端静态资源）
- 前端：React + TypeScript + Vite + Ant Design + ECharts
- 前端补充（2026-09-12，决策 20）：界面改版新增 **`react-router-dom`（精确锁定 6.30.6，Phase 2 已引入）**（页面 URL 可寻址/刷新保持/前进后退）；**明确不引入 Next.js**（与 Win7 + Chrome 109 浏览器基线及单 exe + go:embed 交付形态硬冲突，证据见决策 20）
- 交付形态：Windows 单机 `equipment.exe`，双击启动 → 自动打开 `http://localhost:8080`
- **Win7 兼容约束（2026-09-07 新增目标）**：Win7 SP1 纳入交付支持范围后，后端须锁定 **Go ≤ 1.20**（最后支持 Win7 的版本）工具链、`CGO_ENABLED=0` 纯 Go 构建（SQLite 用 `modernc.org/sqlite`，避免 mingw/winpthread 运行时 DLL）；前端按 **Chrome 109 / Firefox ESR 115**（Win7 可用浏览器上限）能力构建与实测。

## 5. 已确认决策基线（需求澄清锁定，Phase 0 的输入）

1. **外借建模**：独立 `borrow_record` 外借单表；transaction 记录流转并关联外借单。
2. **外借粒度**：一设备一外借单（Excel「借出编号 6061 6062」拆两条独立单，共享公司/日期）。
3. **归还去向**：外借归一一律 `BORROWED → IN_STOCK`（回仓库）。
4. **状态机白名单**：入库/出库/班组转交/外借/归还/送修/维修完成/报废；报废为终态；OTHER 预留、UI 不开放。
5. **操作人**：操作表单「操作人」输入框 + 最近使用记忆（无登录体系）。
6. **删除策略**：设备禁止物理删除、报废收口；班组/外借方只允许停用，历史引用不清除。
7. **导入固化**：导入时按现状生成 `action=IMPORT_INIT` 初始流转记录（时间取 Excel 时间或导入日）。
8. **交付时间口径**：最近一次流转到当前位置的时间（班组转交即重置）。
9. **逾期口径**：预计归还日期已过且未实际归还 → 逾期，Dashboard/外借页标红。
10. **导入校验重点**：重复/空/多编号、数量与编号数量不一致、时间格式（如 `2022.6.14`）统一、台账数量与财务数量差异、编号跨台账列与借出列的一致性——异常项在 Phase 0 出「待确认清单」。
11. **Win7 兼容目标（2026-09-07 新增，用户确认）**：交付支持范围 = Win7 SP1 + Win10/11。已接受代价：Go ≤ 1.20（EOL、无安全更新）+ 全部依赖版本锁定 + 双环境兼容基线 + 前端按 Chrome 109 / Firefox ESR 115 能力降级。细节已定：目标 Win7 全为 **64 位**（构建矩阵 `GOARCH=amd64`）；交付物**随附 Chrome 109 离线安装包与安装说明**；主章程原文不改动，本决策以 AGENTS.md 为权威记录。Win7 不支持 IE11 渲染本系统。
12. **类别字典（页面可维护，2026-09-07）**：设备类别建成字典表（category），系统设置提供页面增改；设备录入/筛选从字典选择；Excel 导入遇新类别自动归入字典。
13. **设备编号不可修改（2026-09-07）**：编号是设备身份与历史锚点，页面禁止直接修改；录入错误走「受限更正」（填原因 + 写 audit 更正记录）或报废重建。
14. **历史名称快照（2026-09-07）**：transaction 冗余保存流转当时的班组名/外借方名称快照（如 from_team_name / to_team_name / borrower_name），班组或外借方改名后，历史记录仍显示当时的名称，保证可追溯语义不漂移。
15. **导入初始状态与内部单位语义（2026-09-08，用户确认 W-2/W-5）**：
    - W-2 已定：Excel 导入默认**全部设备初始为「在库」**；借出历史不自动还原当前外借状态，由用户在系统内**手动补录**外借/流转（方案 A）。
    - W-5 已定：名称含「双发」的单位（莒县双发、双发分厂（龙山贸易）、双发分厂（华锦）、双发分厂（夏津））属**本厂内部调拨** → 建模为**内部单位（team 字典）**，不使用 borrower；其余 15 个公司（泰和、刘家庄华欣、华和 X、贵华系、七级华泰、华山成衣五厂、沂水钰丽、青岛即阳、日照丁格、龙山针织厂染整等）= **外借方（borrower）**。内部事件 101 条、外部事件 43 条（按 H 列统计）。
16. **类别与编号唯一粒度（2026-09-08，用户确认 W-1/W-3/W-4 + Phase 1 验收通过）** —— ⚠️ 2026-09-09 起**部分被决策 18 取代**：同 (name,model,equipment_no) 不再要求唯一，重复编号按多台真机展开导入（V002 将在 V003 移除）；本条目保留作为旧决策记录。
    - W-1 已定：A 列标注「买发网厂/买华诺/买针织厂」（R344–R380，74 台）**原样保留为类别字典项**（不改变原文语义）；R429–R466 无类别行（181 台杂项）统一归入新类别「**其他**」（导入自动建）。
    - W-3/W-4 已定：编号唯一粒度 = **同 name+model 内唯一**（不同设备允许同号，如各设备自身 001… 复用合法）；同组内重复编号 = BLOCK，导入报告逐条列出待人工处理（改号或确认为两台同号机 → 以 id/internal_code 区分）。数据库以**部分唯一索引** `(name, model, equipment_no) WHERE equipment_no IS NOT NULL` 强制（V002 迁移）。
17. **班组/类别受控删除（2026-09-08，用户要求）**：班组管理、类别字典新增「删除」；仅当**未被当前设备引用**且（班组）**无任何历史流转引用**时可物理删除；被引用一律拒绝并提示改用「停用」（保护历史可追溯）。外借方仍仅停用；equipment 无删除入口。

18. **v1.1 设备身份与导入重构（2026-09-09，批准；执行依据：《设备管理系统 v1.1…最终版 Codex 修改 Prompt.md》根目录文件）**：
    - 身份原则：`equipment.id` 是唯一设备身份；`equipment_no` 原样 TEXT 保存（可重复/为空/含中文符号），严禁作为唯一键；禁止因重复编号整组跳过、禁止因中文拒绝设备。
    - 数据模型：新增 `equipment_seq`（组内展示序号，非身份）；删除 `idx_equipment_name_model_no` 唯一限制；保留 `equipment_no` 普通索引；`display_no` 由**后端计算下发**（编号组：同组多台才加 `（n）`；无编号组恒按 `型号（n）→名称（n）→未编号设备（n）`）。
    - 已确认参数（2026-09-09）：①终验用仓库内旧 Excel `设备借出总账.xlsx`，缺失少量人工补录；②seq 分组键 = **equipment_no + name + model**；③当前在借来源 = 导入默认 IN_STOCK，Preview 出「疑似在借」清单由用户勾选后再置 BORROWED（不猜、不造 RETURN）；④无编号展示按分组恒加序号；⑤生产导入执行「清空重导」（Phase 10，先备份）。
    - 导入与历史：J 列只产生流转事件、**不新建设备**；历史借出进 flow_record（可多次），borrow_record 仅在“可靠当前在借”时建档；手工借出/归还支持历史日期（默认今天、不得晚于今天）；借出记录保存设备/名称/型号快照与 source；导入批次幂等（import_batch + source key）；事务提交 + 审计 + Reconciliation。
    - 流程：Phase 0(备份)→1(V003)→2(seq/身份)→3(Parser)→4(Importer)→5(Borrow历史日期)→6(J列事件)→7(状态推导)→8(Frontend display_no)→9(Preview/Review)→10(真实Excel终验/清空重导)→11(对账)→12(回归)→13(文档)。每 Phase 测试+提交。

19. **v1.3 外借明细补充导入（2026-09-11，用户确认；起因：`工作簿1.xlsx` 导入预览显示「预计新增设备 5,693,048 台」事故）**：
    - 事故根因：解析器按 `设备借出总账.xlsx` 的固定列位（A–K）硬解析且**从不校验表头** → `工作簿1.xlsx`（版式：到达时间/外借方/数量/设备编号）的 B 列外借方被当作设备名称、C 列数量被当作型号、**D 列设备编号被当作台账数量**（其中 20 个数字型单元格合计 5,693,048），F 列（台账编号）为空 → 全部按「无编号」逐台展开。数据库未受影响（日志仅有 `/api/import/parse`，无 `/api/import/run`）。该文件实际 **230 台**（214 有编号 + 16 无编号）。
    - 用户口径（2026-09-11，4 条答复）：①**双发系**（莒县双发、双发分厂（龙山贸易/华锦/夏津））在本明细中**按外借方(borrower)处理、目标状态 BORROWED**——覆盖决策 15 的内部单位口径（仅限本明细，理由：双发→双发分厂属借出行为）；②台数**以 C 列（数量）为准 = 230 台**，源文件表末 C72=208 视为笔误；③**无编号 16 台跳过**（R44=14、R65=1、R66=1）；④D 列括号注释（带拖布轮）只保留编号、注释进备注。补充口径：⑤`0430701` = 2018.1.22 借给莒县双发（库中不存在，按新建处理，Phase 3 定稿）；⑥外借参数 = 外借日期取到达时间、预计归还日期留空、操作人「系统导入」、备注=源行号+原文。
    - 口径定稿（2026-09-11，Phase 4 前用户确认）：①同号多台**按候选顺序自动分配**（`equipment.id` 升序：第 1 条出现→第 1 台、第 2 条→第 2 台），预览可复核、可改选；②标签统一用 **外借1/外借2…**——按**外借方**各自从 1 起、对文件中**没有型号**的设备按出现顺序顺延（本次＝该司全部设备），标签只写外借单/流转备注，**不改台账真实名称型号**；③未匹配编号（`0430701`）**用其外借标签新建设备**（名称＝型号＝外借N）。
    - 分阶段（每 Phase 测试 + 提交 + STOP 等验收）：**Phase 1 模板校验 ✅** → Phase 2 外借明细解析器 → Phase 3 匹配/预览/确认（歧义与未匹配交人工）→ Phase 4 单事务写入（BORROWED + borrow_record + flow_record）+ 对账。

20. **v1.4 界面改版（2026-09-12，用户确认 4 项决策；方案 `docs/ui-redesign-plan.md`，令牌 `docs/ui-design-tokens.md`）**：
    - 起因：用户要求「优化界面，加上 Next.js，以 https://www.deepseek.com/harness/en/ 为范例」。方案阶段结论：**观感来自设计令牌，不来自框架**。
    - **决策 ①：不引入 Next.js**。证据（Next.js 16.3.5，2026-08-25 文档）：官方浏览器基线 **Chrome 111+ / Firefox 111+**，而本项目 Win7 目标上限为 **Chrome 109 / Firefox ESR 115**（决策 11）→ 硬冲突；构建需 **Node.js 20.9+**（Node 20 无法装在 Win7）；App Router 基于 **React 19.2 canary**，而本项目锁定 React 18.2 + antd 5.8.6；静态导出（`output:'export'`）下 Server Actions/Cookies/Headers/Rewrites/ISR/动态路由/默认 `next/image` 优化**均不可用**，且需重配 `basePath/assetPrefix`、改 `go:embed` 布局与 `api.go` 的 `NoRoute` SPA 回退。降级 Next 14 可避开前两条但须锁定两代前的旧框架，收益为零 → 不采纳。
    - **决策 ②：默认深色 + 一键切换浅色**（两套语义令牌 + 顶栏切换 + localStorage 记忆）。
    - **决策 ③：引入 `react-router-dom`**（页面 URL 可寻址、刷新保持、浏览器前进/后退），Phase 2 实施；除此之外不新增任何前端依赖（不引入 Tailwind / CSS-in-JS 库 / UI 组件库）。
    - **决策 ④：先只批准 Phase 0–1**（视觉基准与令牌 + 令牌落地换肤）；Phase 2–6 需阶段性验收后另行批准。
    - 设计令牌来源：本机 DSH 设计系统 `@deepseek-ai/dsh-client-ui-theme`（**实测取得，非猜测**）+ 范例站版式语言。深色模式六状态色经 WCAG 实测全部达 AA（≥4.5:1）。
    - Phase 划分：Phase 0（视觉基准/令牌，只读）→ Phase 1（令牌落地换肤 + 清理硬编码色，页面结构不动）→ Phase 2（布局骨架/导航 + react-router）→ Phase 3（Dashboard 改版）→ Phase 4（列表页统一）→ Phase 5（关键流程/危险操作）→ Phase 6（交付回归 + v1.4.0）。
    - 不变量：数据层/API/状态机/导入对账/备份恢复/导出/Win7 兼容/单 exe + go:embed 交付**全程不动**。

## 6. 当前项目状态（每次阶段推进后更新）

- Git：`main` 分支已初始化；基线提交完成（主章程 + 本治理文件 + 内嵌 Skill + `设备借出总账.xlsx` 入库）。
- 数据源：`设备借出总账.xlsx` 已在工作区（单 Sheet「设备借出总账」；R2 表头 11 列：类别/设备名称/设备型号/台账数量/财务数量/台账设备编号/时间/公司/台数/借出设备编号/备注；含合并单元格、跨行主块、多行/空格分隔编号）。
- 进度：**v1.1 重构（决策 18）已全部完成（Phase 0–13）**：Phase 0(备份/快照 1477/1478) ✅ → Phase 1(V003) ✅ → Phase 2(equipment_seq/display_no 后端) ✅ `18ed98f` → Phase 3(Parser) ✅ `14fd6e6` → Phase 4(Importer) ✅ `49123e8` → Phase 5(Borrow/Flow 历史日期) ✅ `8be0afe` → Phase 6(J 列借出事件解析) ✅ `3f4b0e1` → Phase 7(最终状态推导) ✅ `70762a5` → Phase 8(Frontend display_no) ✅ `92def2a` → Phase 9(Import Preview/Review) ✅ `3b325d3` → Phase 10(真实Excel终验/清空重导) ✅ `b61793d` → Phase 11(Reconciliation 对账) ✅ `ea13029` → Phase 12(Regression Tests) ✅ `23bd76c` → **Phase 13(Documentation) ✅ 本次**。
- v1.1 Phase 3（Parser）要点：F 列逐 token 分类（编号原样保留可中文/重复=多台真机展开不 BLOCK；"无编号"按数量展开；描述文本→无编号+原文备注；无法判断→REVIEW 人工确认前该组不导入，绝不静默丢）；真实 Excel 审计：198 组/D 2147 = 编号 1580 + 无编号 567（含描述 1），组级数量自洽、无 mismatch；重复编号降为 WARN 后真实文件可全量导入 2147 台（原 V3 把平车 670 台误 BLOCK）。
- v1.1 Phase 4（Importer）要点：V004 迁移新增 `import_batch` 表与 equipment 来源列（import_batch_id/source_key）；导入=单事务（批次→类别→设备行→逐台 IMPORT_INIT→回填批次计数→事务内 RenumberAllSeq→audit）；设备级展开每 token 一台（同号多台真机 seq 1..n）、无编号逐台展开；同文件指纹幂等（ErrBatchImported）、非空库守卫（ErrDBNotEmpty）；真实 Excel 导入 2147 台全绿，batch/seq/source_key 断言通过。
- v1.1 Phase 5（Borrow/Flow 历史日期）要点（§二十二/二十三）：`FlowRequest`/`Transition` 支持可选 `OccurredAt`（默认今天、不得晚于当前，ErrOccurredFuture）；借出/归还/所有流转的发生时间、borrow_date/actual_return_date、current_since 均按历史日期落库；API 借出（/flow occurred_at）与归还（/borrows/:id/return actual_return_date）透传历史日期；前端借出/归还弹窗新增"发生日期/实际归还日期"（可留空=今天）；service+api 测试覆盖历史借出/归还/未来日期拒绝。无新迁移（occurred_at 等列已存在）。
- v1.1 Phase 6（J 列借出事件解析）要点（§十四~§二十一）：G 锚定 144 事件（内部 101 + 外部 43，决策15 口径）→ 结构化 `BorrowEvent`（BorrowCellParser）；J 支持纯编号/编号(注释)/括号列表/嵌套异常(11612（11369（带拖布轮）)/无编号/入南库归还线索；无法可靠解析 → REVIEW+保留原文，绝不静默丢；**只解析不写库**。commit `3f4b0e1`。
- v1.1 Phase 7（最终状态推导）要点（§十九/§二十）：`DeriveEquipmentStatus` 将 BorrowEvents 按块匹配 F 设备全集 → 每台建议状态（默认 IN_STOCK；最后借出带可靠归还日期→已回库；无归还证据→疑似在借 SUSPECTED 供 Preview 勾选；缺日/需人工→REVIEW）；绝不自动 BORROWED、绝不伪造 RETURN；`ParseResult.Derived` 汇总。真实文件：设备 2147、疑似 352、REVIEW 71、未匹配 1、归还线索 2。commit `70762a5`。
- v1.1 Phase 8（Frontend display_no）要点（§三十四~§三十七）：后端补齐 display_no 下发（班组视图/Dashboard/外借/导出，同组计数统一 `service.EquipmentSelectNoGrp`）；搜索支持 display_no（剥离（n）后缀匹配）；前端台账/班组/Dashboard/外借/导入结果统一显示 display_no；rowKey 审计均为 equipment.id。commit `92def2a`。
- v1.1 Phase 9（Import Preview/Review）要点（§三十一~§三十三 + 决策18③）：Parse 返回设备级预览/疑似候选/REVIEW 项；疑似=外部公司且组内可唯一（同号多台转 REVIEW 不猜）；REVIEW 门禁（未清点 run 422）；`ImportWithOptions` 勾选设备同事务置 BORROWED+borrower+borrow_record+flow；前端 ImportPage 支持勾选/清点/门禁。真实 Preview：外部疑似 119、REVIEW 57、事件 144。commit `3b325d3`。
- v1.1 Phase 10（真实 Excel 终验 / 清空重导）要点（决策18⑤ + §四十六）：`POST /api/import/reset`（confirm 二次确认 → 自动备份 → `service.ClearImportData` 单事务清空 equipment/flow_record/borrow_record/import_batch → `IMPORT_RESET` audit；保留字典/settings/audit 历史；空库幂等）；前端 ImportPage 清空重导危险入口；e2e 测试：非空库 reset → parse → REVIEW 全清点 → 全量导入 2147 台。commit `b61793d`。
- v1.1 Phase 11（Reconciliation 对账）要点（§四十七）：`importer.Reconcile(db,res)` 以 source_hash 关联批次，Excel 可导入(source_key 全集) ↔ DB 该批次设备逐台比对：缺失/多出逐台列出、无编号/同号多台双侧计数、历史借出未匹配；全等 PASS 否则 FAIL；`POST /api/import/reconcile {parse_id}`（run 后保留会话供对账）；前端 ImportPage 导入后对账卡片；真实文件 e2e 2147==2147 PASS。commit `ea13029`。
- v1.1 Phase 12（Regression Tests）要点：全仓 `go test ./... -count=1`（api/database/importer/service/config 全绿）+ `go vet` + `npm run build` 通过；§四十八 旧逻辑审计（无 equipment_no unique/无 duplicate BLOCK/无 strings.Fields/无 rowKey=equipment_no）；回归关键链路重跑（真实文件导入 2147、REVIEW 门禁、清空重导、对账 PASS、重复编号展开 seq、历史日期、display_no API/班组/外借）；增强 TestExportAndDashboard 断言导出「显示编号」列与 Dashboard display_no（无编号 EBK-SA（n））。
- v1.1 Phase 13（Documentation）要点：版本号升至 **v1.1.0**（后端 + 前端页脚，用户确认）；`docs/user-guide.md` 重写为 v1.1 语义（显示编号规则、导入五步流程：解析→预览→REVIEW 审阅/疑似在借→导入→对账、清空重导、历史日期、FAQ Q4–Q8）；`README.md` 版本表新增 v1.1.0；`docs/database-design.md` 补 V003/V004 迁移与 import_batch/equipment_seq/display_no；`docs/roadmap.md` 补决策 18 完成记录；AGENTS 进度线收口 Phase 13 → tag `v1.1.0`（本次提交）。
- 基线（v1.0.0，2026-09-08）：Phase 0–6 完成，tag v1.0.0；发布构建脚本产出 `release/equipment/`；用户手册/FAQ/Win7 浏览器说明就绪；Win7 实机 0xc0000005 问题待用户回传 EQ_STEPLOG。
- **v1.2 设备流转情况导出（2026-09-09，执行依据：《Equipment Flow Export v1.1——完整 Codex 实现 Prompt.md》根目录文件）**：Phase 1 查询层（`service/flow_export.go`：FlowExportFilter team_id/status/borrower_id + include_current/history/summary，三组批量查询无 N+1，current_location/historyLocation 推导，历史按 equipment_id 关联）→ Phase 2 XLSX（`service/flow_export_xlsx.go`：三 Sheet 当前流转情况/流转历史/统计汇总，标题合并加粗/表头冻结+AutoFilter/列宽/编号文本写入 SetCellStr/日期格式统一/隐藏设备ID列/空数据占位）→ Phase 3 API（`GET /api/export/flow`，中文文件名 RFC5987，审计 EXPORT_EQUIPMENT_FLOW 记筛选+数量不存二进制）→ Phase 4 前端（台账页「导出设备台账」「导出流转情况」两按钮 + `FlowExportModal` 筛选/数据范围/含统计 + Blob 下载 + loading/防重复）→ Phase 5 测试与对账（13 用例 + 集成 + 2000 台 + 真实库对账 2147↔Sheet1、2266↔Sheet2 PASS）→ **Phase 6 收尾（本次）**：版本升 **v1.2.0** + tag `v1.2.0`。口径锁定：状态用词「在库」、历史显示当前名称（不新增快照字段）、导出记审计。
- **v1.3 导入安全加固（2026-09-11，决策 19）**：
  - **Phase 1 模板校验 ✅** —— `importer.Parse` 读取 R2 表头后先执行 `validateTemplate`（与模板 A–K 逐列比对），不符返回 `ErrTemplateMismatch`（API 400 + 中文提示，回显 R2 实际内容与 R1 标题行）；新增 V11 校验规则。效果：`工作簿1.xlsx` 由「预计新增 5,693,048 台」变为**明确拒绝并说明原因**，真实 `设备借出总账.xlsx` 解析结果不变（198 组 / 2147 台 / 144 事件 / 119 疑似 / 57 REVIEW）；`internal/importer/template_test.go`（4 用例）。commit `d108937`。
  - **Phase 2 外借明细解析器 ✅ 本次** —— 新增独立通道 `importer.ParseBorrowDetail`（`internal/importer/borrow_detail.go`）：版式 `A 到达时间 | B 外借方 | C 数量 | D 设备编号`（可选 `E 设备名称 / F 设备型号`，决策 19 消歧方案 A 预留）；块 = C 锚定行（合并感知、**不硬编码行号**）；D 列复用 `parseJCell` 提取编号（嵌套/前导括号按编号展开、注释只留备注）；**无编号按口径③跳过**；表末合计行与 C 合计不一致 WARN（**以 C 列为准**）；V-D1/D2/D3 **BLOCK**、V-D8 REVIEW、V-D4/D5/D9 WARN；只读不写库，设备带 `R{from}-R{to}#N{i}` 来源键与 `raw_lines` 原文。真实 `工作簿1.xlsx`：57 块 / C 合计 **230**（214 编号 + 16 无编号）/ 0 BLOCK / 0 REVIEW / 8 WARN；与库匹配 198 唯一命中 + 15 同号多台（12 个编号）+ 1 未匹配（0430701）；`borrow_detail_test.go`（7 用例，含真实文件回归、误传台账版式拒绝、可选列解析）。
  - **Phase 3 匹配 / 预览 ✅ 本次** —— 新增只读匹配层 `importer.MatchBorrowDetail`（`borrow_detail_match.go`，一次性查询无 N+1）：分桶 `UNIQUE`（编号唯一命中）/ `BY_LABEL`（文件给出 名称/型号 或 `预分配N` 标签唯一定位）/ `AMBIGUOUS`（同号多台 → 系统按 `equipment.id` 升序给 **预分配1、预分配2…** 占位标签；文件内出现次数=库中台数时给「按出现顺序对应 预分配N」**建议**，仍需用户确认）/ `MISSING`（不在库，附同长度差 1 位的**旁证**）；设备当前非在库给 `V-M1` WARN；外借方按块核对（已存在/需新建）。API：`POST /api/import/borrow-detail/parse`（解析+匹配预览，只读）。前端：新增菜单「外借明细导入」`web/src/ImportDetailPage.tsx`（摘要 / 待人工选择 / 未匹配 / 外借方计划 / 明细预览 / 块级预览 / 问题清单；页面明示本阶段为预览，写入在 Phase 4）。真实 `工作簿1.xlsx` ↔ 生产库：唯一 **198**、待人工选择 **15**、未匹配 **1**（0430701）、外借方 4 个双发系**均需新建**（95/94/20/5 台）。测试：`borrow_detail_match_test.go`（2 用例，含真实文件对照）+ `import_detail_api_test.go`（2 用例，含误传总账文件必须 400 并提示改用台账通道）。顺带修复：明细通道误传台账版式时按台账表头行（R2）判定并给引导；错误回显对横向合并标题行去重。
  - **Phase 4 外借明细补录写入 ✅ 本次** —— 单事务写入 `importer.ImportBorrowDetail`（`borrow_detail_import.go`）：解析批次（复用 `import_batch`，同文件指纹幂等 409）→ 新建设备（未匹配编号，名称＝型号＝**外借N**，写 `IMPORT_INIT`+`BORROW`）→ 外借方字典复用/新建 → **置 BORROWED + 外借单（borrow_date＝到达时间，预计归还留空）+ 流转记录（BORROW，occurred_at＝到达时间，borrower_name 快照，operator＝系统导入）** → `RenumberAllSeq` → `IMPORT_BORROW_DETAIL` audit。新增 API `POST /api/import/borrow-detail/run`（需 `confirm:true`；`choices` 覆盖同号多台选择且只接受候选内 id；`skip` 跳过）。前端 ImportDetailPage 增加「同号多台改选 / 跳过 / 确认补录（二次确认弹窗）+ 补录结果逐台报告」。
    - **用户追加口径（2026-09-11）**：标签统一用 **外借1/外借2…**（按**外借方**各自从 1 起、对文件中**没有型号**的设备按出现顺序顺延；本次＝该司全部设备），**只写外借单/流转备注，不改台账真实名称型号**；同号多台**按候选顺序自动分配**（可复核改选）；未匹配的 `0430701` **用其外借标签新建设备**。
    - **安全约束**：BLOCK 块整体不写入；设备已有未归还外借时只补历史不覆盖现状（`BORROW_HISTORY_ONLY`+`V-M1`）；同文件只能补录一次。
    - **生产库预演（在 `equipment.db` 副本上执行，生产库未改动）**：214 台全部置外借 + 新建 1 台（`0430701` → `外借94` / `EQ-002148` / 2018-01-22）+ 新建 4 个双发系外借方；补录后 2148 台（在库 1815 / 外借 333）、外借单 333、流转 2481。测试：`borrow_detail_import_test.go`（2 用例）+ `import_detail_api_test.go` 增补 run 端到端（含未确认 400、重复补录 409）。
  - **Phase 5 对账 + 收尾 ✅ 本次** —— ①**分批补录**（用户要求「选择外借方导入 + 全选」）：`/run` 新增 `companies` 范围参数，批次键改为 `sha256(文件指纹+本次范围)` → 同文件同范围幂等 409、不同范围可分批；逐台写入前按外借单备注来源键查重（`ALREADY_WRITTEN`），分批不会重复建单。②**对账** `ReconcileBorrowDetail` + `POST /api/import/borrow-detail/reconcile`：按备注来源键逐台核对（一致 / 未补录（不算差异）/ 不一致），前端「执行对账」按钮 + 差异清单。③**版本收尾**：升 **v1.3.0**（`cmd/equipment/main.go` + 前端页脚 + README 版本表），`docs/user-guide.md` 增补「外借明细导入（v1.3）」章节，tag `v1.3.0`。测试：`borrow_detail_reconcile_test.go`（含补录前/分批后 PASS、篡改日期 FAIL）。
  - **界面口径（2026-09-12，用户要求）**：列表分页统一——**设备台账每页 20 条**（总查看），**其他查看类列表每页 10 条**（班组设备/外借/导入预览/备份/设置/明细补录等），不足一页不显示分页控件；实现集中在 `web/src/pagination.ts`（`PAGE_SIZE_LEDGER=20` / `PAGE_SIZE_LIST=10` / `listPagination()`），各页面统一引用；`docs/user-guide.md` §3 已写明。release/equipment/ 已重建（版本号仍 v1.3.0）。
  - **数据放置提示**：`release/equipment/` 是**交付目录**，其中的 `equipment.db` 曾被 9/10 测试版留成空库（本次已用工作区生产库 `equipment.db`（2147 台）覆盖，原空库备份在 `.tmp/release-db-backup/`）。给已有数据的电脑升级时务必用 `scripts/update-app.ps1` 或只替换 `equipment.exe`，不要整体覆盖程序目录（用户手册 §9 已写明）。
  - 构建：`scripts/build-release.ps1` 产出 `release/equipment/`（含 Phase 1–5 与界面分页口径；**未覆盖根目录 `equipment.exe`**，按用户要求）。
- **v1.4 界面改版（2026-09-12，决策 20；方案 `docs/ui-redesign-plan.md`、令牌 `docs/ui-design-tokens.md`）**：
  - **方案落档 ✅**：确认「不引入 Next.js」（三条硬冲突证据：Chrome 111 基线 vs Chrome 109 目标、Node 20.9 构建 vs Win7、React 19.2 canary vs 锁定 React 18.2/antd 5.8.6）；确认「默认深色 + 一键切换浅色」；确认 Phase 2 引入 `react-router-dom`；确认**先只批准 Phase 0–1**。产出 `docs/ui-redesign-plan.md`（Phase 0–6 划分、不变量、风险对策）。
  - **Phase 0 视觉基准与设计令牌 ✅**：从本机 DSH 设计系统 `@deepseek-ai/dsh-client-ui-theme` **实测**取得静态色板与语义别名（非猜测，附可复现采样命令）；对全部候选色做 WCAG 实测 —— **深色模式六状态色全部达 AA（5.23–9.53）**；实测发现既有硬编码色在深色底**不合格**（`#cf1322` 3.27、`#666` 3.18、`#722ed1` 2.63），列入 Phase 1 清理。产出 `docs/ui-design-tokens.md`（令牌表、状态映射、字阶、间距/圆角、antd 映射、组件规范、已知偏差 §11）。
  - **Phase 1 令牌落地（换肤）✅ 本次** —— 新增 `web/src/theme.ts`（**全站唯一色值来源**：深浅两套语义别名 + `applyCssVariables()` + `buildAntdTheme()`）、`themeMode.tsx`（模式 Context + localStorage 记忆 + 首屏前预写入防闪烁）、`status.ts`（六状态文案/配色唯一来源：CSS 变量版给 DOM、实色版给 ECharts 画布）、`echartsTheme.ts`（深浅两套主题，模块加载即注册）、`global.css`（**只引用 `var(--...)`**，不写字面色值）；`main.tsx` 挂 `ThemeModeProvider` + 动态 `ConfigProvider`；`App.tsx` 顶栏新增深浅切换按钮并清理 `#001529`/`#fff`/`#aaa`；`DashboardPage` 图表改传 `ECHARTS_THEME[mode]` 并改用 `status.ts` 供色；`BorrowsPage`/`ImportPage`/`ImportDetailPage` 清理 `#cf1322`/`#722ed1`/`#fa8c16`/`#666`/`#888`。**页面结构、表格列、分页口径（20/10）、业务逻辑、后端与 API 全部未改**。
    - 实施中发现并修正的 antd 5.8.6 约束（由 TS 编译期拦截）：Layout 组件令牌实为 `colorBgHeader`/`colorBgBody`/`colorBgTrigger`；**Table 组件令牌接口为空**（`ComponentToken {}`），表头底色/文字色改由 `global.css` 语义变量接管；Sider 底色用内联 `var(--bg-layer-1)`。
    - 验证：`tsc --noEmit` ✅、`vite build`（`target: chrome109` 不变）✅、`go build ./...` ✅、`go test ./... -count=1` 全绿（api/config/database/importer/service）✅；`web/src` 除 `theme.ts` 外 `#hex` 扫描 **0 处**（55 个色值全部集中）；`#001529`/`theme="dark"` 残留 0 处。**深浅切换与对比度的浏览器实机目视待用户验收**（本环境无浏览器，不做未验证的断言）。
    - 留待后续：浅色模式「外借」文字色偏差（tokens §11 第 1 项）→ Phase 4；其余页面状态映射收敛到 `status.ts` → Phase 4；`docs/user-guide.md` 界面说明 → 按方案由 Phase 6 随发布补写。
    - 交付物**未重建**（`release/equipment/` 与根目录 `equipment.exe` 仍为 v1.3.0），前端页脚版本号未改 —— 按 Phase 6 统一收口。
  - **Phase 2 布局骨架与导航 ✅ 本次** —— 新增 `web/src/routes.tsx`（**路由与菜单唯一来源**：key/path/label/description/icon，一处驱动侧栏菜单、页面头标题与说明、`document.title`；路径匹配容忍结尾斜杠）、`web/src/PageHeader.tsx`（标题 + 一句话说明）；`App.tsx` 重构为应用外壳（顶栏含品牌名/版本/深浅切换 + 可折叠侧栏 200/56px + 内容区 + `<Routes>`），未登记路径统一 `<Navigate>` 回默认页；`main.tsx` 加 `BrowserRouter`；`theme.ts` 补 `Layout.colorBgTrigger`；`global.css` 增页面头样式与侧栏菜单滚动。跨页打开设备详情改走**路由 state**，`EquipmentPage`/`TeamViewPage` 的 props 未变 —— **10 个页面组件零改动**。
    - 依赖选型：`react-router-dom` **精确锁定 6.30.6**（未选 v7：v7 已转向 framework/data-router 模式，对本项目 10 页 SPA 属多余复杂度；v6 行为面更小）。安装时系统盘 npm 缓存目录 EPERM，改用仓库内 `web/.npm-cache`；`npm install` 报告移除 87 个 on-disk 冗余包 —— 已核对：`package-lock` 仅 +43 行（只新增 react-router / react-router-dom / @remix-run/router）、顶层依赖齐全、esbuild 二进制在位、构建产物一致，**构建链未被破坏**。
    - 与初版方案的偏差（已记入方案文档）：`PageHeader` 由**外壳驱动**（标题/说明取自 `routes.tsx`）而非逐页嵌入，页面级操作按钮留待 Phase 4/5。
    - **新增后端回归测试** `internal/api/spa_test.go`（3 用例）：10 条前端路由 + `/` + 未登记路径全部 200 且回退 `index.html`（守住**深链接刷新**）；`/api` 未知路径仍 JSON 404 不被 SPA 回退吞成 HTML；从 `index.html` 动态提取 `./assets/*` 逐个请求均 200 非空（不硬编码构建哈希）。
    - 验证：`tsc --noEmit` ✅、`vite build` ✅（3336 模块 / JS 2.28 MB / gzip 739 kB）、`go build ./...` ✅、`go test ./... -count=1` 全绿 ✅；路由表一致性脚本校验：`routes.tsx` 10 条路径 ↔ `App.tsx` `<Route>` 12 条（含 `/` 与 `*`）双向无遗漏。**1366×768 布局、10 个菜单逐项打开、浏览器前进/后退与刷新待用户实机验收**（本环境无浏览器）。
  - **Phase 3 Dashboard 改版 ✅ 本次** —— 新增 `web/src/dashboardModel.ts`（Dashboard 派生计算的唯一来源，**刻意做成零 import 的纯函数**，因此可直接在 Node 里用真实 API 响应跑数值断言）、`web/src/dashboard.css`（Hero / KPI 网格 / 3 列与 2 列栅格 / 响应式断点，仅引用语义变量）；`DashboardPage.tsx` 改为 Hero（设备总数 40px + 摘要行）+ KPI 卡片网格（6 状态 + 逾期）+ 3 列图表 + 2 列表格 + 班组×类别矩阵；Dashboard 响应类型收敛到 model（删除页面内重复 interface）。**后端与 `cmd/` 零改动**。
    - **数值一致性核验（真实生产数据 2148 台，全部 PASS）**：KPI 条目数 === `by_status` 条数（不丢条目）；每个状态 count 与后端**逐项**一致；`by_status` 求和 === `total`（2148 vs 2148）；逾期值与高亮判定 === `overdue_count`（0，不标红）；`percentOf` 0 除兜底（无 NaN/Infinity）与保留 1 位小数；未知状态 → 0。派生值实测：在库 **84.5%** / 外借 **15.5%**。
    - 核验手段（可复现）：预览实例抓真实 `GET /api/dashboard` → 仓库内 esbuild 打包纯函数模块 → Node 运行断言；命令与逐项结果见方案文档 Phase 3 节。
    - 呈现取舍：**不加甜甜圈中心数字**（圆心定位受 legend 影响、无法在本环境目视校准，改为卡片标题写「共 N 台」）；`by_category` 补空态（原版仅 `by_team` 有；非空数据下无差异）。
    - 验证：`tsc --noEmit` ✅、`vite build` ✅（3338 模块 / JS 2.28 MB / gzip 739 kB / CSS 2.65 kB）、`go build ./...` ✅、`go test ./... -count=1` 全绿 ✅；预览实例端到端自检（深链接全部 200、`/api/dashboard` 返回 total=2148）；**全程生产库 SHA256 未变**（`B50615C1…`）。
    - 预览程序已重建：`release/preview-v1.4/equipment-preview-v1.4.exe`（含 Phase 0–3，端口 8090，数据为生产库副本）。**Hero/KPI 在 1366×768 的折行留白与深色对比观感待用户实机验收**。
  - **Phase 4 列表页统一 ✅ 本次** —— ①**状态映射收敛**：删除 5 个文件各自维护的状态清单（`EquipmentPage`/`TeamViewPage` 的 `STATUS_OPTIONS`+`statusMeta`、`ImportPage`/`ImportDetailPage` 的 `statusText`、`FlowExportModal` 的 `STATUS_OPTIONS`），统一到 `status.ts` 并新增 `STATUS_FILTER_OPTIONS`；②**编号等宽**：`global.css` 增 `.ant-table-tbody > tr > td.num-cell`（只作用表体，避免表头也变等宽），7 个页面编号/内部码列加 `className: 'num-cell'`，非表格处用 `.num`；③**吸顶表头**：6 个主列表页共 **9 个 `<Table>` 全部加 `sticky`**；④**台账空态**：按有无筛选条件区分「无匹配结果，请调整或清空筛选条件」与「暂无设备数据」；⑤**tokens §11 第 1 项修复**：ImportPage「勾选确认当前外借」计数由纯彩色文字（浅色下 2.79:1 未达 AA）改为 `<Tag color="orange">` 胶囊承载。本阶段**净减 25 行**。
    - **收敛副产物（修复既有缺陷）**：`ImportPage` 状态列原用嵌套三元 `IN_STOCK→green / BORROWED→orange / 其余→blue`，导致**维修、报废都显示蓝色**；收敛后自动修正为 red / default。
    - **主动判定"无需改动"三项（附证据）**：工具条本就一致（各页筛选行均为 `<Space wrap style={{marginBottom:12}}>`），故未新增未使用的 `.page-toolbar`；表格密度已统一（台账/外借 `middle`，字典/备份/预览 `small`）且 antd 5.8.6 Table 组件令牌为空无法用主题统一内边距；加载态沿用「首屏 `Card loading` 骨架 + 有数据刷新用 Table `loading`」（刷新不应清空内容换骨架）。另：`ImportDetailPage` 候选列把编号拼在模板字符串里，加等宽需改 JSX，收益低，留待需要时处理。
    - **新增可复用守卫脚本** `scripts/check-web-conventions.ps1`（UTF-8 **BOM**，与仓库既有脚本一致）：6 组断言 —— 色值唯一来源 / 状态文案唯一来源 / 分页口径（`PAGE_SIZE_LEDGER=20`、`PAGE_SIZE_LIST=10`、无硬编码 pageSize）/ 编号等宽 / 吸顶表头 / 后端边界说明。该脚本首跑即抓到 `TeamViewPage` 漏改 1 个表格的**真实遗漏**，同时自身也暴露过一次按"同行匹配"判断导致的**误判**（已改为按文件计数）。
    - 验证：约定脚本 **6 组全 PASS**（exit 0）；`tsc --noEmit` ✅、`vite build` ✅（3338 模块 / CSS 2.77 kB）、`go build ./...` ✅、`go test ./... -count=1` 全绿 ✅；**接线回归审计**（逐行 `git diff`）确认未改动任何 `fetch(` / `/api/` / `params.set` / `onSearch` / `setPage(` / `pageSize`；**后端 `internal/ cmd/ go.mod` 零改动**。
    - 实机回归（预览实例，参数与页面一致）：`limit=20&offset=0` total 2148/首条 6040；`offset=20` 首条 277205（**翻页生效**）；`status=IN_STOCK` total **1815** 且全为在库；`q=6061` total 6；`category=1` total 61；`/api/borrows?limit=10` total **333**；dashboard/teams/categories/backups/teams-equipment/borrowers 全 200；两个导出端点 200（106,485 / 221,038 字节）。**交叉勾稽**：`status=IN_STOCK` 的 1815 与 Dashboard 在库数一致、外借 333 与外借数一致；全程生产库 SHA256 未变。
    - **待用户实机验收**：吸顶表头滚动表现、等宽编号观感、1366×768 工具条折行。
  - **Phase 5 关键流程与危险操作 ✅ 本次** —— ①**新增安全不变量测试** `internal/api/safety_test.go`（4 用例，锁住此前**未被任何测试覆盖**的危险操作性质）：`TestNoEquipmentDeleteRoute`（4 条 DELETE 路径全 404，设备禁止物理删除）、`TestEquipmentNumberNotEditableViaPut`（PUT **夹带 `equipment_no` 也被忽略**，决策 13 由"请求结构体不含该字段"结构强制，用测试锁住）、`TestRestoreRequiresConfirm`（缺 filename / confirm=false → 400 且统一错误体）、`TestScrappedIsTerminalViaAPI`（报废后 4 种动作全 400，且**状态未改写、流转历史条数未增加**）。②**新增 `web/src/DangerNotice.tsx`** 统一危险操作文案层级（做什么 → 后果与可回退性），应用于**报废 / 清空重导 / 恢复备份**；组件注释明确边界：**只统一呈现，不改任何确认门槛**。③**修掉流转弹窗两个真实缺口**：报废**无终态后果说明**（已补）、报废确认按钮**非危险色**（已加 `danger`）。`ImportDetailPage` 补录确认的后果说明本就比通用模板更细（含范围清单），**不做降级**替换。
    - **安全约束逐条实测（全部通过）**：无设备删除入口 → 404（测试+实机）；编号不可直接修改 → 实机 PUT 夹带 `equipment_no=SMOKE-9999` 后编号仍为 `6040`；恢复需 confirm → 400（测试+实机）；报废终态 → 4 动作全 400 且无副作用；REVIEW 门禁 **422**（既有 `TestImportRunReviewGate` 断言 `StatusUnprocessableEntity`）；未来日期 → 400（既有测试+实机 `2099-01-01`）；清空重导二次确认（既有 `TestImportResetAndReimport`）；班组/类别受控删除（既有 `TestDeleteDictAPI`）。
    - **发现的安全强度不对称（只提出建议，未擅自改）**：恢复备份要求**输入 `RESTORE` 确认词**，而清空重导（同属清空全部业务数据）**只需点一次确认**。属安全门槛变更、会影响操作习惯 → 交用户决定。
    - **本阶段如实未做**：~~设备详情抽屉的流转历史仍为表格形态，未改为可视化时间线~~ → **该判断有误，已在追加迭代中更正（见下）**。
    - **Phase 5 追加迭代 ✅（用户批准）**：①**更正我先前的错误结论** —— 抽屉的流转历史**本来就是 antd `<Timeline>`**，我当时未读该段代码即下结论（教训：报告结论必须以读过的代码为依据）。②读码后确认时间线的**真实问题**并修复：**静默截断**（`slice(0, 40)` 只画最近 40 条而标题写「共 N 条」；真实均值约 1.16 条/台，几乎不触发，但一旦触发即不实呈现）→ 已去掉截断（后端 `limit=200` 为上界）；**圆点仅两档颜色**（报废红/其余蓝）→ 改为按目标状态取语义色 `statusFillVar(t.to_status)`。③**安全门槛统一**（用户批准）：「清空重导」提升为与「恢复备份」同级，**必须输入确认词 `RESET`**（输入正确前确认按钮禁用）；**仅提高前端强度，后端门槛未动**（仍要求 `confirm:true`，既有测试继续守护）；顺带把恢复的确认输入框由原生 `<input>` 换成 antd `<Input>`。验证：`tsc`/`vite build` ✅、约定脚本 6 组全 PASS ✅、`slice(0, 40)` 残留 0；**交互与观感待用户实机验收**。
    - 验证：`tsc --noEmit` ✅、`vite build` ✅（3339 模块）、`go build ./...` ✅、`go test ./... -count=1` **全绿**（含 4 个新安全用例）✅、约定检查脚本 6 组全 PASS ✅；实机安全实测 4 项符合预期；冒烟写入的预览数据副本已**还原为与生产库字节一致**（`B50615C1…`），生产库全程未变。
    - **待用户实机验收**：`DangerNotice` 弹窗观感、报废确认按钮危险色显示。
- 最终验收待办（目标电脑）：① Win10/11 全流程走查；② Win7 SP1 实机回归（需 Chrome109/FF115）；③ Chrome109 离线包放入 install/；④ v1.1 全流程（状态推导→display_no→Preview/Review→清空重导→对账→回归→文档）验收；⑤ v1.2 导出流程走查；⑥ v1.3 Phase 1 模板校验实测（上传非模板文件应被拒绝并提示）。
