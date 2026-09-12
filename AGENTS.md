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
    - 待确认（不猜测，Phase 3 前定稿）：库中同号多台 12 个编号（各 2 台）的消歧机制——用户初步意见「先用『预分配1/预分配2』暂代设备名称与型号」，具体匹配/落库规则待定稿。
    - 分阶段（每 Phase 测试 + 提交 + STOP 等验收）：**Phase 1 模板校验 ✅** → Phase 2 外借明细解析器 → Phase 3 匹配/预览/确认（歧义与未匹配交人工）→ Phase 4 单事务写入（BORROWED + borrow_record + flow_record）+ 对账。

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
- **v1.3 导入安全加固（2026-09-11，决策 19）**：**Phase 1 模板校验 ✅ 本次** —— `importer.Parse` 读取 R2 表头后先执行 `validateTemplate`（与模板 A–K 逐列比对），不符返回 `ErrTemplateMismatch`（API 400 + 中文提示，回显 R2 实际内容与 R1 标题行）；新增 V11 校验规则。效果：`工作簿1.xlsx` 由「预计新增 5,693,048 台」变为**明确拒绝并说明原因**，真实 `设备借出总账.xlsx` 解析结果不变（198 组 / 2147 台 / 144 借出事件 / 119 疑似 / 57 REVIEW）；新增 `internal/importer/template_test.go`（4 用例）+ `docs/import-rules.md` §2/V11/新增章节；`go test ./... -count=1` 与 `go vet ./...` 全绿。后续 Phase 2（外借明细解析器）待用户验收 Phase 1 后开始。
- 最终验收待办（目标电脑）：① Win10/11 全流程走查；② Win7 SP1 实机回归（需 Chrome109/FF115）；③ Chrome109 离线包放入 install/；④ v1.1 全流程（状态推导→display_no→Preview/Review→清空重导→对账→回归→文档）验收；⑤ v1.2 导出流程走查；⑥ v1.3 Phase 1 模板校验实测（上传非模板文件应被拒绝并提示）。
