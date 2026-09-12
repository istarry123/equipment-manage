# v1.4 界面改版方案（UI Redesign Plan）

- 记录日期：2026-09-12
- 状态：**已批准，Phase 0 进行中**
- 目标版本：v1.4.0
- 参照范例：<https://www.deepseek.com/harness/en/>（DeepSeek Harness 产品页）+ 本机 DSH 控制台（同一设计系统用于交互式控制台的活样本）
- 治理依据：`AGENTS.md` 决策 20；`engineering-persona`；`equipment-management` §19/§27/§32

---

## 1. 背景与目标

用户提出：「优化界面，加上 Next.js，以 DeepSeek Harness 页面为范例」。

本方案把这个诉求拆成两个独立问题并分别作答：

| 诉求 | 真实问题 | 本方案的处理 |
|---|---|---|
| 「加上 Next.js」 | 用户以为需要换框架才能得到该观感 | **不引入**（第 3 节给出与已锁定约束的硬冲突证据） |
| 「界面以该页为范例」 | 真正需要的是**设计语言**（配色/字阶/留白/边框/动效） | 落地为**设计令牌 + 逐页改版**（第 6 节 Phase 0–6） |

**目标**：在**不改数据层、不改 API、不改交付形态**的前提下，让界面获得范例站的视觉语言；默认深色，支持一键切换浅色；页面可寻址（URL 化）。

---

## 2. 已确认决策（2026-09-12，用户答复）

| # | 决策点 | 用户选择 | 含义 |
|---|---|---|---|
| 1 | 是否引入 Next.js | **① 不引入**，走方案 A（设计系统改版） | 技术栈保持 React + Vite；只做设计令牌与版式优化 |
| 2 | 配色模式 | **③ 默认深色 + 一键切换浅色** | 两套语义令牌（浅/深）+ 顶栏切换按钮 + localStorage 记忆 |
| 3 | 是否引入 `react-router` | **引入** | 页面 URL 可寻址、刷新保持当前页、支持浏览器前进/后退（Phase 2 实施） |
| 4 | 推进方式 | **① 先只做 Phase 0–1**，看过整体观感再决定后续 | 本方案批准范围 = Phase 0–1；Phase 2–6 待阶段性验收后再批准 |

**决策 3 的治理影响**：`react-router-dom` 属章程外依赖，已由用户明确批准并记入 `AGENTS.md` 决策 20；除此之外不新增任何前端依赖（不引入 Tailwind、CSS-in-JS 库、UI 组件库）。

---

## 3. 为什么不引入 Next.js（证据）

### 3.1 与已锁定约束的冲突

| # | 现状（已锁定） | Next.js 16（当前稳定版 16.3.5，2026-08-25） | 结论 |
|---|---|---|---|
| 1 | 交付支持 Win7 SP1 + **Chrome 109 / Firefox ESR 115**（`AGENTS.md` 决策 11） | 官方基线 **Chrome 111+ / Edge 111+ / Firefox 111+ / Safari 16.4+** | ❌ 硬冲突：Chrome 109 < 111，Win7 目标浏览器不在支持基线内 |
| 2 | 单 exe 交付，目标机不装 Node | 构建需 **Node.js 20.9+**（Node 18 不再支持） | ❌ Node 20 无法装在 Win7 上，Win7 机器永久失去构建能力 |
| 3 | React **18.2.0** + antd **5.8.6**（`web/package.json`） | App Router 基于 **React 19.2 canary** | ⚠️ 被迫升级 React 19 + antd 大版本，波及全部 12 个页面组件与 ECharts 集成 |
| 4 | `base: './'` + `target: 'chrome109'`；`go:embed` 内嵌 `internal/webui/dist`；`api.go` 的 `NoRoute` 做 SPA 回退 | 静态导出产出 `out/`，资源默认绝对路径 `/_next/...`；每路由一个 HTML + RSC 载荷 | ⚠️ 需重配 `basePath/assetPrefix/trailingSlash`、改 `embed.go` 布局、重写 SPA 回退路由表 |
| 5 | — | 静态导出下**不可用**：Server Actions、Cookies/Headers、Rewrites/Redirects、ISR、动态路由、默认 `next/image` 优化 | ❌ 只剩下「文件路由 + 另一套构建工具」，纯成本 |

证据来源：[Next.js 16 升级指南（Node/浏览器基线）](https://nextjs.org/docs/app/guides/upgrading/version-16)、[静态导出文档（不支持特性清单）](https://nextjs.org/docs/app/guides/static-exports)。

### 3.2 关键判断

> **视觉观感来自 CSS 与设计令牌，不来自框架。** 换 Next.js 不会让任何一个像素变好看，却会引入构建链替换、全站路由重写、React/antd 大版本升级与 Win7 兼容破坏。

降级到 Next 14 可避开冲突 1、2，但要把前端框架锁死在两代以前的旧版本，仍承担冲突 4 的改造，收益为零 —— 违反 `engineering-persona` §20/§21（技术选型/依赖原则）与 `equipment-management` §32.4（不引入未要求的基础设施），故不采纳。

---

## 4. 现状盘点（改版前基线）

### 4.1 前端家底

| 项 | 现状 |
|---|---|
| 框架 | React 18.2.0 + TypeScript 5.1.6 + Vite 4.4.9 |
| UI 库 | antd 5.8.6 + @ant-design/icons 5.2.6 + ECharts 5.4.3 + dayjs 1.1.9 |
| 页面 | 12 个组件（`web/src/*.tsx`，约 3,500 行）：Dashboard / 设备台账 / 班组设备 / 外借 / 班组管理 / 数据导入 / 外借明细导入 / 备份 / 设置 + 弹窗组件 |
| 路由 | **无路由**：`App.tsx` 用 `useState` 切菜单，刷新即回 Dashboard |
| 主题 | **无主题层**：无全局 CSS 文件、`ConfigProvider` 仅设中文 locale，未启用 `darkAlgorithm` |
| 颜色 | 混用 antd 预设 Tag 色 + **硬编码 hex**（见 4.2） |
| 构建 | `tsc --noEmit && vite build` → `internal/webui/dist`（已 gitignore），`target: chrome109` |
| 交付 | Go `go:embed` 内嵌 dist，`NoRoute` SPA 回退，单 exe，离线 |

### 4.2 深色化的既有障碍（已实测）

硬编码颜色在深色底上**对比度不合格**（WCAG AA 要求正文 ≥ 4.5:1）：

| 位置 | 现有色值 | 深色底 #151517 对比度 | 判定 |
|---|---|---|---|
| `DashboardPage.tsx` L151、`BorrowsPage.tsx` L182、`ImportPage.tsx` L501 | `#cf1322` | 3.27 | ❌ 不合格 |
| `ImportDetailPage.tsx` L476 | `#666` | 3.18 | ❌ 不合格 |
| `ImportPage.tsx` L500 | `#722ed1` | 2.63 | ❌ 不合格 |
| `DashboardPage.tsx` L165 | `#999` | 6.40 | ✅ |
| `ImportDetailPage.tsx` L567/614/687/702 | `#888` | 5.14 | ✅ |

→ 结论：**深色化必须同时清理硬编码颜色**，否则 Phase 1 交付的是"有对比度缺陷的深色主题"。这一清理纳入 Phase 1 范围（机械替换为语义令牌，不改结构与逻辑）。

---

## 5. 设计语言来源

### 5.1 采用方式

| 来源 | 用途 |
|---|---|
| DeepSeek Harness 产品页 | **版式与节奏**：大标题 + 一句说明、卡片网格、发丝线分区、大留白、动效克制 |
| 本机 DSH 控制台设计系统 | **具体令牌值**：从 `@deepseek-ai/dsh-client-ui-theme` 实测取得静态色板与语义别名（见 `docs/ui-design-tokens.md`），非猜测 |

### 5.2 可迁移 / 不可迁移

范例站是**营销落地页**，本系统是**高信息密度管理控制台**。必须区分对待，否则会损害 `engineering-persona` §19（清晰、准确、高效）的要求：

| 页面类型 | 迁移程度 |
|---|---|
| Dashboard | **可 80% 落地页化**：KPI Hero、卡片网格、大留白、图表统一主题 |
| 列表页（台账/班组/外借/导入…） | **只取令牌，不取版式**：深色底、发丝线、等宽编号、统一状态胶囊、统一空/加载态，但保持高信息密度与分页口径（20/10 不变） |
| 弹窗/表单/危险操作 | 统一样式与文案层级（标题—说明—后果—确认），危险操作红色强调 |

---

## 6. Phase 计划

> 每 Phase 循环（`equipment-management` §27）：实现 → 测试 → 修复 → 更新文档 → Git commit → tag → 阶段报告 → **STOP 等验收**。
> 标签命名：`v1.4.0-phase0`、`v1.4.0-phase1` …
> **本次批准范围 = Phase 0–1**；Phase 2–6 需另经批准。

### Phase 0 — 视觉基准与设计令牌（只读，不改代码）✅
- **范围**：采样范例站视觉语言 + 从本机 DSH 设计系统实测令牌值；建立色板、字阶、间距、圆角规范；对全部候选状态色做 WCAG 对比度校验。
- **产出**：`docs/ui-design-tokens.md`。
- **验收**：令牌值可复现（附采样命令）、对比度全部达标、状态映射与业务状态机一一对应。
- **回退**：仅新增文档，无代码影响。

### Phase 1 — 设计令牌落地（换肤，页面结构不动）
- **范围**：
  1. 新增 `web/src/theme.ts`（色板 + 语义别名 + `applyCssVariables` + `buildAntdTheme`）、`web/src/themeMode.tsx`（模式状态 + localStorage + `useThemeMode`）、`web/src/status.ts`（六状态单一来源）、`web/src/echartsTheme.ts`（注册深浅两套 ECharts 主题）、`web/src/global.css`（基础层，只使用 `var(--...)`，不写字面色值）。
  2. `main.tsx` 挂载主题；`App.tsx` 顶栏接入深浅切换按钮（最小改动；版式重构属 Phase 2）。
  3. 清理 4.2 节列出的硬编码颜色（机械替换为语义令牌）。
  4. Dashboard 图表接入统一主题（否则坐标轴/图例在深色底不可读）。
- **不改**：任何页面结构、表格列、分页口径、业务逻辑、后端与 API。
- **验收**：`npm run build` 通过；逐页走查对比度；深浅两模式切换与刷新记忆；Chrome 109 实测（**由用户在有浏览器的环境确认，见阶段报告**）。
- **回退**：`git revert` 单个提交即回到 v1.3.0 观感。

### Phase 2 — 布局骨架与导航 ✅（2026-09-12 完成）
- **范围**：`App.tsx` 重构为顶栏 + 可折叠侧栏 + 内容区；统一 `PageHeader` 组件；引入 `react-router-dom`（页面可寻址、刷新保持、前进后退）；`/api` 代理与 `NoRoute` 回退保持兼容。
- **验收**：10 个菜单逐项打开；1366×768（Win7 常见分辨率）不塌陷；URL 直接访问/刷新/后退均正确；后端 SPA 回退不需改动。
- **风险**：`react-router-dom` 需联网安装（若离线，需先确认 npm 源可用）。

**实施记录：**

- 新增 `web/src/routes.tsx`：**路由与菜单的唯一来源**（key/path/label/description/icon），一处驱动侧栏菜单、页面头标题与说明、`document.title`；路径匹配容忍结尾斜杠。
- 新增 `web/src/PageHeader.tsx` + `global.css` 对应样式。
- `App.tsx` 改为应用外壳：顶栏（品牌名可点击回默认页 + 版本 + 深浅切换）+ 可折叠侧栏（`collapsible`，展开 200px / 收起 56px）+ 内容区（`PageHeader` + `<Routes>`）；未登记路径统一 `<Navigate>` 回默认页。
- `main.tsx` 增加 `BrowserRouter`（放在 `ConfigProvider` 内层）。
- 跨页打开设备详情改为**路由 state 传递**（`navigate('/equipment', { state: { openDeviceId } })`），`App.tsx` 不再持有跳转状态；`EquipmentPage` / `TeamViewPage` 的 props 接口未变，**页面组件零改动**。
- `theme.ts` 补 `Layout.colorBgTrigger`（折叠触发器底色）。

**与初版方案的两处偏差（已记录）：**

1. **`PageHeader` 由外壳驱动，而非逐页嵌入**：标题与一句话说明取自 `routes.tsx`，10 个页面组件**完全不改**（降低回归面）。页面级操作按钮（导出、刷新等）留待 Phase 4/5 统一。
2. **依赖版本选 `react-router-dom@6.30.6`（精确锁定）而非 v7**：v7 已转向 framework/data-router 模式，对本项目 10 页 SPA 属多余复杂度；v6 行为面更小、与 React 18 组合最稳。理由记入 `AGENTS.md` 决策 20。

**新增后端回归测试** `internal/api/spa_test.go`（3 用例）——深链接刷新是**后端行为**，必须由后端测试守住：

| 用例 | 断言 |
|---|---|
| `TestSPAFallbackDeepRoutes` | 10 条前端路由 + `/` + 未登记路径，全部 200 且回退到 `index.html` |
| `TestAPINotSwallowedBySPAFallback` | `/api/...` 未知路径仍返回 JSON 404，不被 SPA 回退吞成 HTML |
| `TestStaticAssetsServedFromEmbed` | 从 `index.html` 动态提取 `./assets/*` 引用并逐个请求，均 200 且非空（不硬编码构建哈希） |

### Phase 3 — Dashboard 改版 ✅（2026-09-12 完成）
- **范围**：KPI Hero（大字号数字 + 说明 + 占比）、卡片网格、逾期高亮、最近流转时间线、图表美化。**只用现有 `/api/dashboard` 字段，不改后端**。
- **验收**：改版前后**数值逐项一致**（防止"改样式改错数"）；逾期标红正确。

**实施记录：**

- 新增 `web/src/dashboardModel.ts`：Dashboard 派生计算的唯一来源。刻意**不含任何 import**（纯函数、无副作用），因此可以直接在 Node 里用真实 API 响应跑数值断言，不必拉起浏览器或 antd。
- 新增 `web/src/dashboard.css`：Dashboard 专用布局（Hero / KPI 网格 / 3 列与 2 列栅格 / 响应式断点），只引用语义变量。
- `dashboardModel.ts` 的既有 TS 接口成为 Dashboard 响应类型的唯一定义处（消除页面内重复 interface）。
- `DashboardPage.tsx` 改为：Hero（设备总数 40px + 摘要）+ KPI 卡片网格（6 状态 + 逾期）+ 3 列图表栅格 + 2 列表格栅格 + 班组×类别矩阵。
- **后端零改动**：本 Phase 未触碰 `internal/` 与 `cmd/` 任何文件。

**逐项数值审计（改版前 → 改版后，真实生产数据实测）：**

| 旧版元素 | 数据字段 | 新版位置 | 实测值 |
|---|---|---|---|
| 「设备总数」Statistic | `total` | Hero 大字号数字 | **2148** |
| 各状态 Statistic | `by_status[].count` | KPI 卡片（逐条对应，**不按固定清单过滤**） | 在库 1815 / 班组使用 0 / 外借 333 / 维修 0 / 报废 0 / 其他 0 |
| 「逾期外借」Statistic | `overdue_count` | KPI 第 7 张卡片（>0 标红）+「当前外借」卡右上 Tag | **0**（未逾期，不标红） |
| 「状态分布」甜甜圈 | `by_status` | 同图，标题补「共 N 台」 | 6 段 |
| 「各班组/内部单位设备数」 | `by_team` | 同图 + 空态 | 0 行 → 显示「暂无数据」 |
| 「各类别设备数」 | `by_category` | 同图 + **新增**空态 | 9 行 |
| 「最近流转」表（5 列） | `recent_flows` | 列与字段完全不变 | 8 行 |
| 「当前外借」表（4 列 + 逾期 Tag） | `current_borrows` + `overdue_count` | 列与字段完全不变 | 20 行（后端返回上限 20） |
| 「各班组设备使用情况」矩阵 | `by_team_category` | 表结构、合计列、排序完全不变 | 0 行 → 空态文案 |
| 分页口径 | — | `listPagination()`（10 条/页，不足一页不显示） | 不变 |

**本次唯一新增的数字全部是纯派生值**（不来自后端、不改写任何业务数值）：各状态占比 %、Hero 摘要行「在库 x% · 外借 x% · 逾期 N 台」、甜甜圈标题的「共 N 台」。

**核验方式（可复现）：** 启动预览实例 → 抓取真实 `/api/dashboard` 原始响应 → 用仓库内 esbuild 打包纯函数模块 → Node 运行断言：

```powershell
web\node_modules\@esbuild\win32-x64\esbuild.exe .tmp\dashboard-model-check.ts --bundle --platform=node --format=cjs --outfile=.tmp\dashboard-model-check.cjs
node .tmp\dashboard-model-check.cjs
```

**实测断言结果（全部 PASS，真实生产数据 2148 台）：**

| 断言 | 结果 |
|---|---|
| KPI 卡片条目数 === 后端 `by_status` 条数（不丢条目） | 6 vs 6 ✅ |
| 每个状态设备数与后端**逐项**一致 | ✅ |
| `by_status` 求和 === `total`（状态口径自洽） | 2148 vs 2148 ✅ |
| 逾期卡片数值 / 高亮判定 === `overdue_count` | 0，不高亮 ✅ |
| `percentOf` 0 除兜底（不产生 NaN / Infinity） | ✅ |
| `percentOf` 保留 1 位小数（33.3 / 12.5） | ✅ |
| `countOfStatus` 未知状态 → 0 | ✅ |

派生值实测：在库 **84.5%**、外借 **15.5%**，各状态占比之和 100%。

**两点呈现层面的取舍（已记录）：**

1. **不加甜甜圈中心数字**：ECharts 圆心定位受 `legend` 布局影响，无法在本环境目视校准，宁可不在圆心放数字，改为把「共 N 台」写进卡片标题（可核验、不会错位）。
2. `by_category` 补了空态（原版只有 `by_team` 有）—— 非空数据下无任何差异，属纯健壮性改进。

**未验证项**：Hero/KPI 在 1366×768 下的折行与留白观感、深色模式下的对比观感 —— 需浏览器目视（本环境无浏览器）。

### Phase 4 — 列表页统一 ✅（2026-09-12 完成）
- **范围**：台账/班组设备/外借/班组管理/备份/设置/导入预览共用页头、工具条、表格密度、吸顶表头、空状态、加载骨架、分页样式；编号使用等宽字体。
- **验收**：分页口径（20/10）、筛选、搜索、导出行为全部回归；`display_no` 显示规则不变。

**实施记录（净减 25 行——收敛重复定义才是本阶段的主要收益）：**

| 交付项 | 做法 |
|---|---|
| **状态映射收敛** | 删除 5 个文件里各自维护的状态清单（`EquipmentPage`/`TeamViewPage` 的 `STATUS_OPTIONS`+`statusMeta`、`ImportPage`/`ImportDetailPage` 的 `statusText`、`FlowExportModal` 的 `STATUS_OPTIONS`），统一到 `status.ts`；新增 `STATUS_FILTER_OPTIONS` 供各页筛选器复用 |
| **编号等宽** | `global.css` 增 `.ant-table-tbody > tr > td.num-cell`（**只作用于表体**，避免表头字体也变等宽而与其它表头不一致）；7 个页面的编号/内部码列加 `className: 'num-cell'`；非表格处（Dashboard、班组设备编号按钮）用 `.num` |
| **吸顶表头** | 6 个主列表页的全部 `<Table>` 加 `sticky`（**共 9 个表格**，由约定脚本逐文件核对）。表头底色已是不透明语义色，吸顶后不会透出内容 |
| **台账空态** | 台账表加 `locale.emptyText`：有筛选条件时提示「无匹配结果，请调整或清空筛选条件」，否则「暂无设备数据」——用户在"筛错了"和"真没数据"之间不再需要猜 |
| **浅色「外借」文字色偏差** | tokens §11 第 1 项修复：ImportPage 的「勾选确认当前外借」计数由**纯彩色文字**（浅色下 2.79:1，未达 AA）改为 `<Tag color="orange">`（antd 胶囊自带底色，两模式均可读），符合 §11 原定的处置方向 |

**发现并修复的一处既有缺陷（收敛的副产物）**：`ImportPage` 的状态列原先用嵌套三元 `IN_STOCK→green / BORROWED→orange / 其余→blue`，导致**维修显示为蓝色、报废也显示为蓝色**（与状态语义不符）。收敛到 `statusTagColor()` 后自动修正为 red / default。

**主动判定「无需改动」的三项（附证据，不做无用功）：**

1. **工具条**：核实各页筛选行本就是 `<Space wrap style={{marginBottom:12}}>`（`EquipmentPage` L412、`BorrowsPage` L249、`TeamViewPage` L145/255）——**本来就一致**，故未新增 `.page-toolbar` 类（避免留下未使用的死 CSS）。
2. **表格密度**：各页主表格已是 `size="middle"`（台账/外借）或 `size="small"`（字典/备份/预览），且 antd 5.8.6 的 Table 组件令牌接口为空、无法用主题统一内边距，改动收益不足。
3. **加载态**：沿用「首次加载用 `Card loading` 骨架屏 + 有数据后刷新用 Table `loading`」——这是正确取舍（刷新时不应清空内容换成骨架），未强改为统一骨架屏。

**未纳入本阶段（如实记录）**：`ImportDetailPage` 的「候选」列把 `display_no` 拼在模板字符串里，加等宽需改成 JSX；该表已有 `设备编号` 等宽列，收益低，留待需要时处理。

**新增可复用守卫脚本** `scripts/check-web-conventions.ps1`（UTF-8 BOM，与仓库既有脚本一致）：

| 检查组 | 断言 |
|---|---|
| 1 色值唯一来源 | `web/src` 除 `theme.ts` 外无硬编码 `#hex` |
| 2 状态文案唯一来源 | 无页面重复定义状态字典（精确匹配 `IN_STOCK: '在库'`，避免误伤说明文字） |
| 3 分页口径 | `PAGE_SIZE_LEDGER=20`、`PAGE_SIZE_LIST=10`；页面内无硬编码 `pageSize: 10/20`；台账使用 `PAGE_SIZE_LEDGER` |
| 4 编号等宽 | 5 个主列表页使用 `num-cell` |
| 5 吸顶表头 | 6 个主列表页 `<Table>` 数 ≤ `sticky` 数 |

> 该脚本首次运行即抓到一次**真实遗漏**（`TeamViewPage` 的 `<Table>` 属性换行书写，导致 3 个表格漏改 1 个）——已修复；同时它自身也暴露过一次**误判**（初版按"同行匹配"判断，把写法正确的跨行 JSX 判为失败），已改为按文件计数比较。这个故事本身就是"把约定变成断言"的价值证明。

**验证结果：**

| 验证项 | 结果 |
|---|---|
| 约定检查脚本（6 组） | **全部 PASS**（exit 0） |
| `tsc --noEmit` / `vite build` | ✅ 3338 模块 / CSS 2.77 kB |
| `go build ./...` / `go test ./... -count=1` | ✅ 全绿 |
| 接线回归审计（`git diff` 逐行） | ✅ **未改动任何** `fetch(` / `/api/` / `params.set` / `onSearch` / `setPage(` / `pageSize` |
| 后端零改动 | ✅ `git diff --name-only v1.4.0-phase3..HEAD -- internal cmd go.mod` 为空 |
| 实机回归（预览实例，页面同参数） | ✅ 见下表 |

**实机回归（真实生产数据副本，参数与页面完全一致）：**

| 请求 | 结果 |
|---|---|
| `/api/equipment?limit=20&offset=0` | total **2148**，返回 20 台，首条 `6040` |
| `/api/equipment?limit=20&offset=20` | 返回 20 台，首条 `277205`（**翻页确实换页**） |
| `/api/equipment?status=IN_STOCK` | total **1815**，全部为在库 ✅ |
| `/api/equipment?q=6061` | total **6**（关键词搜索生效） |
| `/api/equipment?category=1` | total 61 |
| `/api/borrows?limit=10&offset=0` | total **333**，返回 10 条 |
| `/api/dashboard`、`/api/teams`、`/api/categories`、`/api/backups`、`/api/teams/equipment`、`/api/borrowers` | 全部 200 |
| `/api/export/equipment`、`/api/export/flow` | 200，106,485 / 221,038 字节（**导出仍产出真实文件**） |

**交叉勾稽（两处独立口径互证）**：`status=IN_STOCK` 的 total **1815** 与 Dashboard 的「在库」数一致；`/api/borrows` 的 total **333** 与 Dashboard 的「外借」数一致。全程生产库 SHA256 未变（`B50615C1…`）。

**未验证项**：吸顶表头在真实滚动中的表现、等宽编号的观感、1366×768 下的工具条折行 —— 需浏览器目视。

### Phase 5 — 关键流程与危险操作 ✅（2026-09-12 完成）
- **范围**：设备详情抽屉时间线、流转弹窗、导入五步流程、清空重导二次确认的统一样式与文案层级。
- **验收**：安全约束逐条实测（REVIEW 门禁 422、清空重导二次确认、历史日期不得晚于今天、无删除入口）。

**实施记录：**

1. **新增安全不变量测试** `internal/api/safety_test.go`（4 用例）—— 把铁律「危险操作必须受控」中**此前未被任何测试锁定**的部分固化：
   - `TestNoEquipmentDeleteRoute`：`DELETE /api/equipment/:id`、`/api/equipment`、`/equipment/:id/flow`、`/equipment/:id/transactions` 全部 404（设备禁止物理删除，决策 6）
   - `TestEquipmentNumberNotEditableViaPut`：`PUT /api/equipment/:id` **夹带 `equipment_no` 也被忽略**（决策 13）。该性质目前由「请求结构体不含该字段」这一**结构设计**保证，此测试把它锁住，防止日后被无意放开；同时断言 name/model 正常更新（证明不是整个请求被丢弃）
   - `TestRestoreRequiresConfirm`：缺 filename → 400；`confirm` 缺省/为 false → 400；并断言 400 响应是**统一错误体**（`error.code` 为 HTTP 状态码、`error.message` 可读）
   - `TestScrappedIsTerminalViaAPI`：报废设备尝试 4 种流转动作全部 400，且**状态未被改写、流转历史条数未增加**（失败请求必须无副作用）
2. **新增 `web/src/DangerNotice.tsx`**：统一危险操作文案层级（`message` = 做什么 → `description` = 后果与可回退性），应用于**报废 / 清空重导 / 恢复备份**三处。
   - 边界写进组件注释：**只统一呈现，不改变任何确认门槛**（不新增/不取消二次确认、确认词、必填原因）——安全门槛的调整必须单独提出并由用户确认。
3. **修掉流转弹窗两个真实缺口**（Phase 5 实测发现）：
   - 报废动作**没有任何终态后果说明**（只有"报废原因"必填）→ 补 `DangerNotice`：「报废为终态：设备报废后不可再流转；本操作不可撤销（如需继续使用只能重新录入）」；
   - 报废的 Modal **确认按钮不是危险色** → `okButtonProps={{ danger: flowAction === 'SCRAP' }}`。

**安全约束逐条实测结果（全部通过）：**

| 验收项 | 锁定方式 | 实测结果 |
|---|---|---|
| 无设备删除入口 | 🆕 `TestNoEquipmentDeleteRoute` + 实机 | 4 条路径全部 **404**；实机 `DELETE /api/equipment/1`、`/api/equipment` 亦 404 ✅ |
| 设备编号不可直接修改（决策 13） | 🆕 `TestEquipmentNumberNotEditableViaPut` + 实机 | PUT 夹带 `equipment_no=SMOKE-9999` → 200 但编号仍为 `6040` ✅ |
| 恢复需确认（铁律 5） | 🆕 `TestRestoreRequiresConfirm` + 实机 | 缺 filename / `confirm:false` → **400** 且统一错误体；实机 400 ✅ |
| 报废为终态（决策 4） | 🆕 `TestScrappedIsTerminalViaAPI` | 4 种动作全 400；状态仍 SCRAPPED；**流转历史条数未变** ✅ |
| 导入 REVIEW 门禁 422 | 既有 `TestImportRunReviewGate`（断言 `StatusUnprocessableEntity`） | PASS ✅ |
| 历史日期不得晚于今天 | 既有 `TestDoFlowHistoricalDate` + 实机 | PASS；实机未来日期 `2099-01-01` → **400** ✅ |
| 清空重导需二次确认 | 既有 `TestImportResetAndReimport` | PASS ✅ |
| 班组/类别受控删除 | 既有 `TestDeleteDictAPI` | PASS ✅ |

**未改（因为已经比统一模板更好）**：`ImportDetailPage` 的「确认补录」弹窗后果说明本就比通用模板更细（含补录范围与外借方台数清单），**不做降级**替换。

**发现的安全强度不对称（只提出，未擅自改）**：「恢复备份」（覆盖全部数据）要求**输入 `RESTORE` 确认词**，而「清空重导」（清空全部业务数据）**只需点一次确认**。两者都属于最高风险档，建议把清空重导也改为输入确认词。由于这是**安全门槛变更**（会增加操作步骤、影响既有使用习惯），本阶段仅提出建议，等用户决定。

**本阶段如实未做**：设备详情抽屉的流转历史仍为**表格**形态，未改为可视化时间线。原因：属纯呈现重构、改动集中在 `EquipmentPage` 抽屉区，且本环境无法目视校准；建议作为 Phase 5 的追加迭代，或并入 Phase 6 一并验收（见阶段报告待决项）。

**验证**：`tsc --noEmit` ✅、`vite build` ✅（3339 模块 / JS 2.28 MB）、`go build ./...` ✅、`go test ./... -count=1` **全绿**（含 4 个新安全用例）✅、前端约定检查脚本 **6 组全 PASS** ✅；实机安全实测 4 项全部符合预期；冒烟测试写过的预览数据副本已**还原为与生产库字节一致**（SHA256 `B50615C1…`），生产库全程未变。

**未验证项**：`DangerNotice` 在弹窗中的视觉观感、报废弹窗危险色确认按钮的实际显示 —— 需浏览器目视。

### Phase 6 — 交付与回归
- **范围**：Win10/11 + Win7 SP1（Chrome 109 / FF115）实测；`scripts/build-release.ps1` 重建 `release/equipment/`；版本升 v1.4.0；更新 `docs/user-guide.md`（界面说明）；tag `v1.4.0`。

---

## 7. 不变量（全程不动）

- SQLite 数据与迁移、GORM 模型、`transaction`/`flow_record` 写入语义。
- 状态机与全部业务规则；导入/对账链路；备份恢复；导出。
- Win7 兼容策略（Go ≤1.20、`CGO_ENABLED=0`、`target: chrome109`）。
- 交付形态：单 exe + `go:embed` + `NoRoute` SPA 回退 + 离线运行。
- 技术栈：React / TypeScript / Vite / Ant Design / ECharts（+ 经批准的 `react-router-dom`）。

## 8. 风险与对策

| 风险 | 对策 |
|---|---|
| 深色下长时间阅读表格疲劳 | 提供浅色模式一键切换（决策 2③），两套令牌均过对比度校验 |
| 状态色在深色底对比度不足 | Phase 0 即完成对比度实测（已在 `ui-design-tokens.md` 记录结论） |
| 「改样式改错业务」 | 每 Phase 用同数据逐项比对数值/状态/显示编号 |
| antd 5.8.6 在 Chrome 109 的表现 | Phase 1 结束即实测，不推迟到 Phase 6 |
| 范围蔓延（Phase 1 变成全站重构） | 严格限定 Phase 1 只做令牌落地 + 硬编码色清理；版式重构留给 Phase 2+ |
| 双模式令牌与 antd 令牌漂移 | 令牌**单一来源**：仅 `theme.ts` 定义色值，运行时写入 CSS 变量；`global.css` 只引用 `var(--...)` |

## 9. 治理合规

- 本文件与 `AGENTS.md` 决策 20 同步记录；技术栈变更（`react-router-dom`）已获用户批准。
- Phase 0 产出 `docs/ui-design-tokens.md`；Phase 1 结束后更新 `AGENTS.md` §6 状态与阶段报告。
- commit 遵循 Conventional Commits（`style(web):` / `feat(web):` / `docs:`）。
