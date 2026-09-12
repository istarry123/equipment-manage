# 界面设计令牌规范（UI Design Tokens）

- 记录日期：2026-09-12（v1.4 界面改版 Phase 0 产出）
- 状态：**待用户确认**（Phase 0 验收物）
- 上游方案：`docs/ui-redesign-plan.md`
- 参照：<https://www.deepseek.com/harness/en/>（版式语言）+ 本机 DSH 控制台设计系统（令牌值）

> **不猜测原则**：本文所有色值、字阶、字族均**实测取得**，非目测或记忆。采样方法见第 1 节，可原样复现。

---

## 1. 令牌来源与可复现采样方法

设计系统令牌位于本机 DSH 包内（同一品牌的交互式控制台实现）：

```
<dsh>/node_modules/@deepseek-ai/dsh-client-ui-theme/lib/client.js
```

复现命令（PowerShell）：

```powershell
$p = "$env:APPDATA\npm\node_modules\@deepseek-ai\dsh\node_modules\@deepseek-ai\dsh-client-ui-theme\lib\client.js"
$t = Get-Content -Raw $p
[regex]::Matches($t, '--dsw-static-[a-zA-Z0-9_-]+\s*:\s*[^;,\}"'']{1,40}') | ForEach-Object { $_.Value } | Sort-Object -Unique
[regex]::Matches($t, '--dsw-alias-[a-zA-Z0-9_-]+\s*:\s*[^;,\}"'']{1,70}')  | ForEach-Object { $_.Value } | Sort-Object -Unique
```

语义别名（`--dsw-alias-*`）每个键有**两个值**（先浅色、后深色），判定依据：`--dsw-alias-bg-base` 的深色值为 `neutral-bluish-950`（`#151517`），是深色调。

对比度校验方法：WCAG 2.1 相对亮度公式（`contrast = (L1+0.05)/(L2+0.05)`），在 PowerShell 中实算，结果见第 4、5 节。

---

## 2. 静态色板（实测原值，不改写）

### 2.1 中性（bluish 系 —— 本项目主用中性）

| 令牌 | 值 | 用途（深色模式） |
|---|---|---|
| `neutral-bluish-00` | `#ffffff` | 浅色底 |
| `neutral-bluish-50` | `#f9fafb` | 深色主文字 / 浅色页底 |
| `neutral-bluish-75` | `#f1f3f5` | 浅色 hover |
| `neutral-bluish-100` | `#ebeef2` | 浅色边框/填充 |
| `neutral-bluish-150` | `#e9ecf2` | 浅色 overlay |
| `neutral-bluish-200` | `#e1e5ee` | 浅色滚动条 |
| `neutral-bluish-300` | `#cfd3d6` | 深色次文字 |
| `neutral-bluish-400` | `#adb2b8` | 深色三级文字 |
| `neutral-bluish-500` | `#979da6` | 深色报废/中性 |
| `neutral-bluish-600` | `#81858c` | 深色弱化文字 / 浅色三级文字 |
| `neutral-bluish-700` | `#61666b` | 浅色次文字 |
| `neutral-bluish-750` | `#43454a` | 深色 dimmed |
| `neutral-bluish-800` | `#353638` | 深色 layer-3 |
| `neutral-bluish-850` | `#2c2c2e` | 深色 layer-2 |
| `neutral-bluish-875` | `#232324` | 深色 layer-1 |
| `neutral-bluish-900` | `#1b1b1c` | 深色代码块 |
| `neutral-bluish-950` | `#151517` | **深色页面底** |
| `neutral-bluish-1000` | `#0f1115` | **浅色主文字** |

### 2.2 品牌与语义色

| 令牌 | 值 | 说明 |
|---|---|---|
| `deepseek-400` | `#679efe` | 深色链接/品牌 |
| `deepseek-450` | `#5686fe` | 品牌中间值 |
| `deepseek-500` | `#4176e6` | 浅色品牌填充 |
| `deepseek-600` | `#4868b2` | 浅色链接（文字用） |
| `deepseek-800` | `#34415b` | 品牌浅底 |
| `deepseek-900` | `#283142` | 品牌极浅底（深色模式） |
| `green-400` | `#4ed17e` | 深色成功 |
| `green-500` | `#22c55e` | 成功填充 |
| `green-900` | `#233c2c` | 成功浅底（深色）/ 浅色成功文字 |
| `amber-400` | `#f7ad31` | 深色警告 |
| `amber-500` | `#f59e0b` | 警告填充 |
| `amber-600` | `#dd8629` | 浅色警告文字 |
| `amber-900` | `#27241f` | 警告浅底（深色） |
| `red-400` | `#f25a5a` | 深色危险 |
| `red-500` | `#ef4444` | 危险填充 |
| `red-600` | `#ec1313` | 浅色危险（系统值） |
| `red-900` | `#570c0c` | 危险浅底（深色） |
| `file-type-violet` | `#8b76f6` | 深色「其他/待确认」 |

### 2.3 原生色（补充，仅用于兼容既有页面语义）

| 值 | 说明 |
|---|---|
| `#cf1322` | **既有**浅色危险文字（沿用，AA 通过 5.57） |
| `#722ed1` | **既有**浅色 REVIEW/审阅文字（沿用，AA 通过 6.94） |

---

## 3. 语义令牌（本项目映射表）

命名规则：`--` + 语义名（小写连字符）。**浅色为 `:root` 默认，深色由 `[data-theme="dark"]` 覆盖。**

| CSS 变量 | 浅色 | 深色 | 用途 |
|---|---|---|---|
| `--bg-base` | `#ffffff` | `#151517` | 页面底色 |
| `--bg-layer-1` | `#ffffff` | `#232324` | 卡片/顶栏/侧栏 |
| `--bg-layer-2` | `#ffffff` | `#2c2c2e` | 浮层/下拉/tooltip |
| `--bg-layer-3` | `#ffffff` | `#353638` | 更高层 |
| `--bg-sunken` | `#f9fafb` | `#1b1b1c` | 代码块/内嵌区 |
| `--bg-hover` | `#f1f3f5` | `rgba(255,255,255,0.08)` | 行/项 hover |
| `--border-l1` | `rgba(0,0,0,0.04)` | `rgba(255,255,255,0.06)` | 最浅分隔 |
| `--border-l2` | `rgba(0,0,0,0.10)` | `rgba(255,255,255,0.12)` | 卡片描边 |
| `--border-l3` | `rgba(0,0,0,0.12)` | `rgba(255,255,255,0.16)` | 强描边 |
| `--text-primary` | `#0f1115` | `#f9fafb` | 主文字 |
| `--text-secondary` | `#61666b` | `#cfd3d6` | 次文字 |
| `--text-tertiary` | `#81858c` | `#adb2b8` | 三级文字 |
| `--text-caption` | `#81858c` | `#81858c` | 说明/图注 |
| `--text-dimmed` | `#61666b` | `#43454a` | 极弱（占位） |
| `--brand` | `#4176e6` | `#679efe` | 品牌填充/主按钮 |
| `--link` | `#4868b2` | `#679efe` | 链接与强调文字 |
| `--status-in-stock` | `#22c55e` | `#4ed17e` | 在库（填充） |
| `--status-in-team` | `#4176e6` | `#679efe` | 班组使用（填充） |
| `--status-borrowed` | `#f59e0b` | `#f7ad31` | 外借（填充） |
| `--status-maintenance` | `#ef4444` | `#f25a5a` | 维修（填充） |
| `--status-scrapped` | `#979da6` | `#979da6` | 报废（填充） |
| `--status-other` | `#722ed1` | `#8b76f6` | 其他（填充） |
| `--status-in-stock-text` | `#233c2c` | `#4ed17e` | 在库（文字） |
| `--status-in-team-text` | `#4868b2` | `#679efe` | 班组使用（文字） |
| `--status-borrowed-text` | `#dd8629` | `#f7ad31` | 外借（文字） |
| `--status-maintenance-text` | `#cf1322` | `#f25a5a` | 维修/逾期（文字） |
| `--status-scrapped-text` | `#61666b` | `#979da6` | 报废（文字） |
| `--status-other-text` | `#722ed1` | `#8b76f6` | 其他（文字） |
| `--danger-text` | `#cf1322` | `#f25a5a` | 错误/逾期文字 |
| `--radius-card` | `12px` | `12px` | 卡片/块圆角（系统值） |
| `--radius-control` | `8px` | `8px` | 控件圆角（antd 基线，明示取值） |
| `--shadow-lv1` | `0 2px 4px rgba(0,0,0,0.05)` | `0 2px 4px rgba(0,0,0,0.35)` | 轻浮层 |
| `--shadow-lv2` | `0 4px 12px rgba(0,0,0,0.02)` | `0 4px 12px rgba(0,0,0,0.45)` | 卡片悬浮 |

> **分层设计原则（来自范例）**：深色模式靠 **1px 极淡描边**（`--border-l2`）而非阴影区分层次；浅色模式同理，阴影只用于真正的浮层。

---

## 4. 业务状态映射（与状态机一一对应）

对应 `equipment-management` §6 的六种状态，前端显示中文不变，**只换配色来源**：

| 状态值 | 中文 | 填充令牌 | 文字令牌 | antd Tag（既有语义，保持不变） |
|---|---|---|---|---|
| `IN_STOCK` | 🟢 在库 | `--status-in-stock` | `--status-in-stock-text` | `green` |
| `IN_TEAM` | 🔵 班组使用 | `--status-in-team` | `--status-in-team-text` | `blue` |
| `BORROWED` | 🟠 外借 | `--status-borrowed` | `--status-borrowed-text` | `orange` |
| `MAINTENANCE` | 🔴 维修 | `--status-maintenance` | `--status-maintenance-text` | `red` |
| `SCRAPPED` | ⚫ 报废 | `--status-scrapped` | `--status-scrapped-text` | `default` |
| `OTHER` | 🟣 其他 | `--status-other` | `--status-other-text` | `purple` |
| （派生） | 逾期 | `--status-maintenance` | `--status-maintenance-text` | `red` |

### 4.1 状态色对比度实测（深色底 `#151517`）

| 状态 | 填充色 | 填充在深色底 | 文字色 | 文字在深色底 | 判定 |
|---|---|---|---|---|---|
| 在库 | `#4ed17e` | 9.33 | `#4ed17e` | 9.33 | ✅ AA |
| 班组使用 | `#679efe` | 6.86 | `#679efe` | 6.86 | ✅ AA |
| 外借 | `#f7ad31` | 9.53 | `#f7ad31` | 9.53 | ✅ AA |
| 维修 | `#f25a5a` | 5.55 | `#f25a5a` | 5.55 | ✅ AA |
| 报废 | `#979da6` | 6.68 | `#979da6` | 6.68 | ✅ AA |
| 其他 | `#8b76f6` | 5.23 | `#8b76f6` | 5.23 | ✅ AA |

→ **深色模式六状态全部达到 WCAG AA（≥4.5:1）**。

### 4.2 状态色对比度实测（浅色底 `#ffffff`）

| 状态 | 填充在浅色底 | 文字色 | 文字在浅色底 | 判定 |
|---|---|---|---|---|
| 在库 | 2.28 | `#233c2c` | 11.97 | ✅ AA（填色仅作非文本，3:1 规则另见 §10） |
| 班组使用 | 4.10 | `#4868b2` | 5.39 | ✅ AA |
| 外借 | 2.15 | `#dd8629` | **2.79** | ⚠️ **未达 AA（见 §11 已知偏差）** |
| 维修 | 3.76 | `#cf1322` | 5.57 | ✅ AA |
| 报废 | 2.73 | `#61666b` | 5.80 | ✅ AA |
| 其他 | 3.49 | `#722ed1` | 6.94 | ✅ AA |

---

## 5. 文字色阶对比度实测

| 令牌 | 深色值 | 深色底对比度 | 浅色值 | 浅色底对比度 |
|---|---|---|---|---|
| `--text-primary` | `#f9fafb` | 17.45 ✅ | `#0f1115` | 18.90 ✅ |
| `--text-secondary` | `#cfd3d6` | 12.11 ✅ | `#61666b` | 5.80 ✅ |
| `--text-tertiary` | `#adb2b8` | 8.54 ✅ | `#81858c` | 3.71 ⚠️（系统原值，见 §11） |
| `--text-caption` | `#81858c` | 4.92 ✅ | `#81858c` | 3.71 ⚠️ |
| `--link` | `#679efe` | 6.86 ✅ | `#4868b2` | 5.39 ✅ |
| `--danger-text` | `#f25a5a` | 5.55 ✅ | `#cf1322` | 5.57 ✅ |

### 5.1 既有硬编码色的深色底表现（本次必须清理的原因）

| 位置 | 现有值 | 深色底对比度 | 处理 |
|---|---|---|---|
| `DashboardPage.tsx` L151、`BorrowsPage.tsx` L182、`ImportPage.tsx` L501 | `#cf1322` | 3.27 ❌ | 替换为 `--danger-text` |
| `ImportPage.tsx` L500 | `#722ed1` | 2.63 ❌ | 替换为 `--status-other-text` |
| `ImportDetailPage.tsx` L476 | `#666` | 3.18 ❌ | 替换为 `--text-tertiary` |
| `ImportDetailPage.tsx` L567/614/687/702 | `#888` | 5.14 ✅ | 替换为 `--text-tertiary`（统一来源） |
| `DashboardPage.tsx` L165 | `#999` | 6.40 ✅ | 替换为 `--text-tertiary` |
| `DashboardPage.tsx` L27–28 | 状态色 6 值 | — | 由 `web/src/status.ts` 统一提供 |
| `DashboardPage.tsx` L96 | `#1677ff` | 4.44 ⚠️ | 替换为 `--brand` |
| `App.tsx` L84/85/90 | `#001529`/`#fff`/`#aaa` | — | 替换为 `--bg-layer-1`/`--text-primary`/`--text-tertiary` |

---

## 6. 字体

系统字族（实测原值）：

```
--dsw-font-family:
  -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Hiragino Sans GB",
  "Microsoft YaHei", "Helvetica Neue", Helvetica, Arial, sans-serif

--ds-font-family-code:
  "SF Mono", "JetBrains Mono", "Fira Code", Consolas, "Liberation Mono", Menlo,
  Courier, "PingFang SC", "Microsoft YaHei"
```

> **Win7 注意**：`Microsoft YaHei`（微软雅黑）为 Win7 起自带字体，中文渲染可用；等宽栈在 Win7 上退化到 `Consolas`（Vista 起自带），可用。

### 6.1 字号阶梯（实测原值）

| 名称 | 字号/行高 | 字重 | 用途 |
|---|---|---|---|
| `xxxs-11` | 11/14 | 400 | 图注、表格辅助 |
| `xxs-12` | 12/18 | 400 | 次级说明 |
| `xs-13` | 13/20 | 400 | 表格正文、表单说明 |
| `s-14` | 14/22 | 400 | **正文默认** |
| `s-strong-14` | 14/22 | 500 | 强调正文 |
| `base-16` | 16/24 | 400 | 大正文 |
| `l-20` | 20/28 | 500 | 卡片标题 |
| `xl-24` | 24/32 | 600 | 页面标题 |
| `md-code` | 12/19 | 400（等宽） | 设备编号 |

> 范例的「大字号 + 紧行高」节奏：Dashboard Hero 数字使用 `xl-24` 的**放大版**（Phase 3 定义 32/40 字重 600），列表页不放大，保持 13–14px 高密度。

### 6.2 编号等宽规则

设备编号（`display_no`/`equipment_no`）一律使用等宽字族 + `font-variant-numeric: tabular-nums`，便于纵向比对与查错。新增工具类 `.num`（Phase 1 落地于 `global.css`）。

---

## 7. 间距 / 圆角 / 描边

- **间距刻度（4px 基数）**：4 / 8 / 12 / 16 / 24 / 32 / 48。卡片内边距 16，卡片间距 16，页面区块间距 24，Dashboard 大区块 32。
- **圆角**：卡片/块 `12px`（系统值）；控件 `8px`（明示取值）；标签胶囊 `4px`（antd 基线）。
- **描边**：`1px`，颜色用 `--border-l1/l2/l3`。
- **阴影**：仅浮层使用 `--shadow-lv1/lv2`；卡片不用阴影，用描边（对齐范例的分层方式）。

---

## 8. antd 令牌映射（Phase 1 实现依据）

`ConfigProvider theme={{ algorithm, token, components }}` 映射表：

| antd token | 浅色 | 深色 | 来源 |
|---|---|---|---|
| `algorithm` | `defaultAlgorithm` | `darkAlgorithm` | 模式 |
| `colorPrimary` | `#4176e6` | `#679efe` | `--brand` |
| `colorLink` | `#4868b2` | `#679efe` | `--link`（浅色换深一档以满足 AA） |
| `colorSuccess` | `#22c55e` | `#4ed17e` | `--status-in-stock` |
| `colorWarning` | `#f59e0b` | `#f7ad31` | `--status-borrowed` |
| `colorError` | `#ef4444` | `#f25a5a` | `--status-maintenance` |
| `colorInfo` | `#4176e6` | `#679efe` | `--brand` |
| `colorBgBase` | `#ffffff` | `#151517` | `--bg-base` |
| `colorBgLayout` | `#f9fafb` | `#151517` | 页面底 |
| `colorBgContainer` | `#ffffff` | `#232324` | `--bg-layer-1` |
| `colorBgElevated` | `#ffffff` | `#2c2c2e` | `--bg-layer-2` |
| `colorText` | `#0f1115` | `#f9fafb` | `--text-primary` |
| `colorTextSecondary` | `#61666b` | `#cfd3d6` | `--text-secondary` |
| `colorTextTertiary` | `#81858c` | `#adb2b8` | `--text-tertiary` |
| `colorTextQuaternary` | `#81858c` | `#81858c` | `--text-dimmed` |
| `colorBorder` | `rgba(0,0,0,0.12)` | `rgba(255,255,255,0.16)` | `--border-l3` |
| `colorBorderSecondary` | `rgba(0,0,0,0.06)` | `rgba(255,255,255,0.08)` | `--border-l1/l2` |
| `borderRadius` | `8` | `8` | `--radius-control` |
| `borderRadiusLG` | `12` | `12` | `--radius-card` |
| `fontFamily` | 系统字族（§6） | 同 | 实测 |
| `fontSize` | `14` | `14` | `s-14` |
| `controlHeight` | `32` | `32` | antd 基线 |
| `wireframe` | `false` | `false` | — |

**组件级覆盖**（保持克制，只覆盖与本系统布局相关的项）：

| 组件 | 覆盖要点 |
|---|---|
| `Layout` | **实测修正**：antd 5.8.6 的 Layout 组件令牌为 `colorBgHeader` / `colorBgBody` / `colorBgTrigger`（**不存在** `headerBg`/`bodyBg`/`siderBg`/`headerHeight`）；侧栏底色由 Sider 内联样式取 `var(--bg-layer-1)` |
| `Menu` | 项圆角 `--radius-control`；选中态用 `--bg-hover` + `--link` 文字；**不再使用 `theme="dark"`**（改为跟随模式） |
| `Card` | `headerBg: transparent`；`paddingLG: 16` |
| `Table` | **实测修正**：antd 5.8.6 的 Table 组件令牌接口为**空**（`ComponentToken {}`），无法用主题定制表头 → 表头底色/文字色/描边改由 `global.css` 的 CSS 变量规则接管（`.ant-table-thead > tr > th`、`.ant-table-tbody > tr > td`） |
| `Descriptions` | `labelBg` 取 `--bg-sunken` |
| `Tag` | 沿用 antd 预设色（`green`/`blue`/`orange`/`red`/`default`/`purple`），由 algorithm 自动适配两模式，**不手改** |

---

## 9. 组件规范

### 9.1 页面头（PageHeader，Phase 2 落地）
结构：`标题（xl-24 600）` + `一句说明（s-14 --text-secondary）` + 右侧主操作按钮。区块之间用 24px 间距，不用分隔线堆叠。

### 9.2 卡片
`--bg-layer-1` 底 + `1px --border-l2` 描边 + `12px` 圆角 + `16px` 内边距；标题 20/28 500；卡片不使用阴影。

### 9.3 状态胶囊
沿用 antd `<Tag>` 预设色（六状态映射见 §4）。**禁止**用自定义 `style={{color}}` 表达状态。

### 9.4 表格
表头 `--bg-sunken` + `--text-secondary` 13px；正文 13px；行 hover `--bg-hover`；纵向描边 `--border-l1`；编号列等宽（`.num`）；分页口径不变（台账 20 条/页，其他查看类 10 条/页，见 `web/src/pagination.ts`）。

### 9.5 空状态与加载态
- 空状态：居中，主文案 14px `--text-secondary`，附一句下一步操作指引（如「可先在班组管理/设备流转中分配」）；**保留各页既有文案**。
- 加载态：骨架屏优先（`Card loading` / `Table loading`），不用转圈遮罩。

### 9.6 危险操作
标题红色强调 + 后果说明 + 二次确认；沿用既有确认机制（清空重导、恢复备份、报废），**不改变确认语义**。

---

## 10. 使用规则（强制）

1. **禁止新增硬编码颜色**：页面与组件只能使用 §3 的语义变量（CSS `var(--...)`）或 antd 令牌。Phase 1 完成后由 `grep` 检查（见阶段报告）。
2. **令牌单一来源**：色值只在 `web/src/theme.ts` 定义一次，运行时写入 `document.documentElement` 的 CSS 变量；`global.css` **只引用** `var(--...)`。
3. **新增颜色必须先过对比度校验**：文字 ≥4.5:1，非文本图形 ≥3:1，结果记入本文档。
4. **状态表达统一**：状态一律走 `web/src/status.ts` + antd `Tag`；新增状态需先改状态机与本文档。

---

## 11. 已知偏差（诚实记录，不隐藏）

| # | 偏差 | 影响 | 处置 |
|---|---|---|---|
| 1 | ~~浅色模式「外借」文字色 `#dd8629` 对比度 2.79（未达 AA 4.5）~~ | ~~仅浅色模式下"外借 N 台"这类彩色数字文本~~ | ✅ **已于 Phase 4 解决**：不再用纯彩色文字承载该数值，改为 antd `<Tag color="orange">` 胶囊（自带底色，两模式均可读）。色板中确实不存在可过 AA 的更深琥珀色文字色，故改用容器承载而非发明新色值 |
| 2 | **浅色模式三级文字 `#81858c` 对比度 3.71** | 次要说明文字（≥14px） | 该值为设计系统原值（`alias-label-tertiary` 浅色），保持与系统一致；不擅自发明新灰阶 |
| 3 | 浅色模式状态填色对白底对比度 2.15–4.10 | 仅用于**非文本**图形（圆点/色块/图表），按 WCAG 1.4.11 需 3:1；「在库」2.28、「报废」2.73、「外借」2.15 略低于 3:1 | 深色模式为主用模式（决策 2③），浅色模式这些填色仅出现在 antd `Tag`（由 antd 自行保证可读性）与图表；图表将叠加描边以增强边界辨识 |

## 12. Phase 0 验收待确认项

1. 本令牌表（§2–§3）与状态映射（§4）是否确认？
2. 已知偏差 §11 的处置方式（其中第 1 项推迟到 Phase 4 修复）是否接受？
3. 确认后进入 Phase 1（按 `docs/ui-redesign-plan.md` §6 Phase 1 执行）。

---

## 13. Phase 1 实施记录（2026-09-12）

### 13.1 落地文件

| 文件 | 职责 |
|---|---|
| `web/src/theme.ts` | **全站唯一色值来源**：色板 + 深浅两套语义别名 + `applyCssVariables()` + `buildAntdTheme()` + 字族 |
| `web/src/themeMode.tsx` | 模式状态（Context + `useThemeMode`）、localStorage 记忆、启动前预写入避免闪烁 |
| `web/src/status.ts` | 六状态文案/配色的唯一来源；CSS 变量版（DOM）与实色版（ECharts 画布）双出口 |
| `web/src/echartsTheme.ts` | 深浅两套 ECharts 主题（模块加载即 `registerTheme`） |
| `web/src/global.css` | 基础层：只引用 `var(--...)`；等宽 `.num`、`.text-muted`、`.text-danger`；滚动条；表格表头/描边接管 |
| `web/src/main.tsx` | 首屏前 `applyCssVariables(readStoredMode())`；`ThemeModeProvider` + `ConfigProvider` 动态主题 |
| `web/src/App.tsx` | 顶栏深浅切换按钮；清理 `#001529`/`#fff`/`#aaa`；Menu/Sider 不再强制 dark |
| `web/src/DashboardPage.tsx` | 图表接入主题（`echarts.init(el, ECHARTS_THEME[mode])`）；状态色改由 `status.ts` 提供；清理 `#1677ff`/`#cf1322`/`#999` |
| `web/src/BorrowsPage.tsx`、`ImportPage.tsx`、`ImportDetailPage.tsx` | 清理 `#cf1322`/`#722ed1`/`#fa8c16`/`#666`/`#888` |

### 13.2 实施中发现的 antd 5.8.6 约束（已按实测修正）

1. **Layout 组件令牌名**：实为 `colorBgHeader` / `colorBgBody` / `colorBgTrigger`；§8 原拟名（`headerBg` 等）不存在，TS 编译期即报错拦截。
2. **Table 组件令牌接口为空**（`ComponentToken {}`），无法通过 `ConfigProvider` 定制表头 → 表头色改由 `global.css` 用语义变量接管。
3. 侧栏底色：Layout 令牌无可用的 Sider 项，改由 Sider 内联样式取 `var(--bg-layer-1)`。

### 13.3 验证结果（本次）

| 验证项 | 方式 | 结果 |
|---|---|---|
| 类型检查 | `tsc --noEmit -p tsconfig.json` | ✅ 通过 |
| 前端构建 | `vite build`（`target: chrome109` 不变） | ✅ 通过（3331 模块，产物 2.26 MB / gzip 731 kB） |
| 后端内嵌编译 | `go build ./...` | ✅ 通过（新 dist 正常嵌入） |
| 硬编码色清理 | 全仓 `web/src` 扫描 `#hex`（排除 `theme.ts`） | ✅ 0 处；55 个色值全部集中于 `theme.ts` |
| 旧主题残留 | 扫描 `#001529` / `theme="dark"` | ✅ 0 处 |
| 产物标记 | 构建产物包含 `--bg-base`/`--text-primary`/`#151517`/`#679efe`/`equipment-theme-mode`/`data-theme` | ✅ 全部命中 |
| 深浅切换与对比度目视 | 浏览器实机走查（Chrome 109 目标） | ⏳ **待用户验收**（本环境无浏览器，不做无法验证的断言） |

### 13.4 留待后续 Phase 的事项

- 浅色模式「外借」文字色偏差（§11 第 1 项）→ Phase 4。
- 其余页面（设备台账/班组设备/班组管理/备份/设置）的状态映射仍为页面内联定义 → Phase 4 统一收敛到 `status.ts`。
- `docs/user-guide.md` 的界面说明 → 按 `ui-redesign-plan.md` §6，由 Phase 6 在发布时统一补写（避免手册描述未发布的行为）。
