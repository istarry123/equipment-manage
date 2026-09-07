# 设备资产与流转管理系统

## 1. 项目目标

开发一个运行在 **Windows 单机环境**中的设备资产与流转管理系统。

系统主要用于管理公司内部设备，核心目标是：

> 能够随时查询某一个具体设备编号当前在哪里、属于哪个班组、什么时候交给该班组；如果设备外借，则能够知道外借日期、外借方、预计归还日期和实际归还日期；如果没有使用，则显示为“在库”。

系统第一版基于现有 Excel：

`设备借出总账.xlsx`

进行初始化数据导入。

Excel 只作为系统第一次初始化的数据来源。

系统正式运行以后：

**禁止继续直接使用 Excel 作为数据库。**

所有设备状态、班组交接、外借、归还等操作都通过系统完成，并保存完整历史记录。

---

# 2. 使用场景

这是一个：

- Windows 本地单机应用
- 只需要一台电脑使用
- 不需要 Linux
- 不需要服务器
- 不需要 Docker
- 不需要 PostgreSQL
- 不需要 Redis
- 不需要互联网
- 不需要云服务

系统运行在：

```text
Windows 10 / Windows 11
```

启动方式：

```text
双击 equipment.exe
```

启动后自动打开：

```text
http://localhost:8080
```

所有数据存储在本地 SQLite 数据库：

```text
equipment.db
```

---

# 3. 技术栈

## 后端

使用：

- Go
- Gin
- SQLite
- GORM
- Go embed

后端负责：

- REST API
- 业务逻辑
- SQLite 数据访问
- 设备状态管理
- 流转记录
- Excel 导入
- 数据备份
- 前端静态文件服务

---

## 前端

使用：

- React
- TypeScript
- Vite
- Ant Design
- ECharts

前端主要负责：

- Dashboard
- 设备台账
- 设备搜索
- 设备详情
- 设备流转
- 外借管理
- 流转历史
- Excel 导入界面
- 数据统计
- 系统设置

---

# 4. 总体架构

采用单体应用架构。

不要设计微服务。

总体结构：

```text
Windows
│
└── equipment.exe
      │
      ├── Go Backend
      │     ├── REST API
      │     ├── Business Logic
      │     └── SQLite
      │
      └── React Frontend
            │
            ├── Dashboard
            ├── Equipment
            ├── Transactions
            ├── Borrowing
            └── Settings
```

数据库：

```text
equipment.db
```

备份：

```text
backup/
```

---

# 5. 核心业务模型

系统核心不是“Excel 管理”，而是：

> 设备生命周期和设备流转管理。

设备必须拥有独立身份。

例如：

```text
设备名称：环形割刀
型号：EBK-SA

设备：

6040
6057
6058
6059
6060
6061
6062
6063
6064
```

每一个设备编号都是一个独立设备实体。

不能只按照“设备名称 + 数量”管理。

---

# 6. 设备状态

第一版实现以下状态：

```text
IN_STOCK
在库
```

```text
IN_TEAM
班组使用
```

```text
BORROWED
外借
```

```text
MAINTENANCE
维修
```

```text
SCRAPPED
报废
```

必要时可以保留：

```text
OTHER
其他
```

前端必须显示中文状态。

例如：

```text
🟢 在库
🔵 班组使用
🟠 外借
🔴 维修
⚫ 报废
```

---

# 7. 核心数据库

初步设计以下核心实体：

## equipment

设备主表。

至少包含：

```text
id
equipment_no
name
model
category
status
remark
created_at
updated_at
```

---

## team

班组。

至少包含：

```text
id
name
department
remark
created_at
updated_at
```

---

## borrower

外借方。

至少包含：

```text
id
name
contact
phone
remark
created_at
updated_at
```

---

## transaction

设备流转记录。

至少包含：

```text
id
equipment_id

action

from_status
to_status

from_team_id
to_team_id

borrower_id

occurred_at

operator

remark

created_at
```

流转记录必须永久保存。

不能因为设备状态改变而删除历史记录。

---

# 8. 核心流转

必须支持：

## 入库

```text
班组 → 仓库
```

状态：

```text
IN_TEAM → IN_STOCK
```

---

## 出库给班组

```text
仓库 → 班组
```

状态：

```text
IN_STOCK → IN_TEAM
```

必须记录：

- 班组
- 交付时间
- 操作人
- 备注

---

## 班组之间转交

```text
班组 A → 班组 B
```

状态：

```text
IN_TEAM → IN_TEAM
```

必须记录：

- 原班组
- 新班组
- 交接时间
- 操作人
- 备注

---

## 外借

```text
仓库/班组 → 外部单位
```

状态：

```text
→ BORROWED
```

必须记录：

- 外借方
- 外借日期
- 预计归还日期
- 操作人
- 备注

---

## 外借归还

```text
外借方 → 仓库
```

状态：

```text
BORROWED → IN_STOCK
```

必须记录：

- 实际归还时间
- 操作人
- 备注

---

# 9. 最重要的业务原则

## 原则一：一个设备编号对应一个设备

不能只管理数量。

---

## 原则二：当前状态和历史记录分离

设备表保存：

```text
当前状态
当前班组
当前外借方
```

流转表保存：

```text
历史过程
```

---

## 原则三：任何状态变化都产生流转记录

例如：

```text
6061

仓库
 ↓
裁剪一组
 ↓
裁剪二组
 ↓
外借给 XX 公司
 ↓
归还
 ↓
仓库
```

所有过程都必须可以查询。

---

# 10. 页面设计

## Dashboard

首页显示：

```text
设备总数
在库
班组使用
外借
维修
报废
```

并显示：

- 各班组设备数量
- 最近设备流转
- 当前外借设备
- 最近归还设备
- 状态统计图

---

# 11. 设备台账

设备列表支持：

```text
设备编号搜索
设备名称搜索
型号搜索
类别筛选
状态筛选
班组筛选
```

列表至少显示：

```text
设备编号
设备名称
型号
类别
状态
当前位置
当前班组/外借方
最后更新时间
```

---

# 12. 设备详情

点击设备，例如：

```text
6061
```

显示：

```text
设备编号：6061
设备名称：环形割刀
型号：EBK-SA
类别：裁剪设备

当前状态：班组使用

当前班组：裁剪一组

交付时间：
2026-09-05 14:30
```

提供操作：

```text
[转交]
[外借]
[归还]
[入库]
[维修]
```

下面显示完整：

```text
流转历史
```

采用时间线展示。

---

# 13. 外借管理

单独提供外借页面。

显示：

```text
设备编号
设备名称
外借方
外借日期
预计归还日期
实际归还日期
状态
```

支持：

```text
查看
归还
延期
查询
```

可以快速发现：

```text
已逾期未归还
```

---

# 14. Excel 导入

第一次运行系统时，需要支持：

```text
Excel
 ↓
解析
 ↓
数据预览
 ↓
数据检查
 ↓
用户确认
 ↓
导入 SQLite
```

必须处理：

- 重复设备编号
- 空设备编号
- 多个编号
- 数量与编号数量不一致
- 空字段
- 重复数据
- 异常数据

对于没有真实设备编号的设备：

不要伪造财务资产编号。

可以生成系统内部 ID，例如：

```text
EQ-000001
```

同时保留：

```text
equipment_no = NULL
internal_code = EQ-000001
```

---

# 15. Excel 导入原则

设备编号是核心。

例如：

```text
台账数量：9

设备编号：

6040
6057
6058
6059
6060
6061
6062
6063
6064
```

必须转换为：

```text
9 条 equipment 数据
```

而不是：

```text
1 条设备 + 数量 9
```

---

# 16. 数据备份

因为系统使用 SQLite，所以实现简单可靠的本地备份。

目录：

```text
backup/
```

例如：

```text
backup/
├── equipment-2026-09-05.db
├── equipment-2026-09-06.db
└── equipment-2026-09-07.db
```

建议：

- 程序启动自动备份
- 用户可以手动备份
- 用户可以恢复数据库
- 默认保留最近 30 份

恢复操作必须进行确认。

---

# 17. Windows 部署

最终目标：

```text
equipment/
│
├── equipment.exe
├── equipment.db
├── config.yaml
│
├── backup/
│
└── logs/
```

用户无需安装：

```text
Go
Node.js
npm
pnpm
Python
Docker
Linux
PostgreSQL
```

最终只需要：

```text
equipment.exe
```

---

# 18. 开发原则

必须严格按照以下阶段开发。

禁止一次性完成整个项目。

每一个 Phase：

1. 分析
2. 实现
3. 测试
4. 修复
5. 更新文档
6. Git commit
7. 输出阶段完成报告

只有当前 Phase 验收通过后，才能进入下一阶段。

---

# Phase 0：需求与数据分析

目标：

不要急于写业务代码。

首先分析：

```text
设备借出总账.xlsx
```

输出：

```text
docs/
├── requirements.md
├── data-analysis.md
├── database-design.md
├── state-machine.md
├── import-rules.md
└── roadmap.md
```

必须分析：

- Excel Sheet
- 表头
- 字段含义
- 设备编号格式
- 重复设备编号
- 无编号设备
- 数量字段
- 借出字段
- 时间字段
- 公司字段
- 备注字段
- 数据异常

最终输出数据库 ER 设计。

### Phase 0 禁止：

- 不开发完整 UI
- 不开发完整 API
- 不修改原 Excel
- 不擅自删除原始数据
- 不猜测数据含义

如果 Excel 某个字段无法确定含义，必须标记：

```text
待确认
```

---

# Phase 1：项目基础架构

目标：

建立：

```text
Go
+
SQLite
+
React
+
TypeScript
+
Vite
+
Ant Design
```

完成：

- 项目目录
- Go 后端
- SQLite 初始化
- 数据库 migration
- REST API 基础框架
- React 基础框架
- Dashboard 空页面
- 前后端通信
- 日志
- 配置文件
- 基础测试

最终可以：

```text
go build
```

生成：

```text
equipment.exe
```

并运行：

```text
http://localhost:8080
```

---

# Phase 2：Excel 数据导入

目标：

把：

```text
设备借出总账.xlsx
```

转换到 SQLite。

实现：

```text
选择 Excel
 ↓
解析
 ↓
数据预览
 ↓
数据校验
 ↓
错误提示
 ↓
确认导入
 ↓
SQLite
```

必须提供导入报告：

```text
总记录：
成功：
跳过：
重复：
异常：
无编号：
```

导入完成后能够看到设备台账。

---

# Phase 3：设备台账

目标：

完成设备管理核心功能。

实现：

- 设备列表
- 搜索
- 筛选
- 分类
- 设备详情
- 设备新增
- 设备编辑
- 设备状态展示
- 当前班组展示
- 当前外借方展示

核心要求：

输入：

```text
6061
```

能够立即得到：

```text
6061
环形割刀
EBK-SA

当前状态：
班组使用

当前班组：
裁剪一组

交付时间：
XXXX-XX-XX XX:XX
```

---

# Phase 4：设备流转

这是整个项目最重要的阶段。

实现：

```text
在库
 ↓
交给班组
 ↓
班组 A
 ↓
班组 B
 ↓
外借
 ↓
归还
 ↓
在库
```

每次操作：

```text
更新 equipment 当前状态
+
写入 transaction
```

确保：

```text
当前状态准确
历史记录完整
```

实现：

- 入库
- 出库
- 班组交接
- 外借
- 外借归还
- 维修
- 维修完成

设备详情显示完整流转时间线。

---

# Phase 5：Dashboard、报表与备份

目标：

完成最终可用版本。

Dashboard：

```text
设备总数
在库
班组使用
外借
维修
报废
```

统计：

```text
各班组设备数量
各类别设备数量
外借设备
逾期设备
最近流转
```

实现：

- ECharts
- 流转统计
- 外借统计
- 班组统计
- Excel 导出
- 数据备份
- 数据恢复

---

# Phase 6：Windows 打包与最终验收

目标：

形成真正可以交付使用的 Windows 单机软件。

最终：

```text
equipment/
├── equipment.exe
├── equipment.db
├── config.yaml
├── backup/
└── logs/
```

实现：

- Windows 启动
- 自动打开浏览器
- SQLite 初始化
- 自动备份
- 日志
- 异常处理
- 数据恢复
- Windows 防火墙说明
- 使用说明

最终进行完整测试：

```text
Excel 导入
设备搜索
设备新增
设备编辑
入库
出库
班组转交
外借
归还
维修
历史查询
Dashboard
Excel 导出
备份
恢复
```

---

# 19. Git 管理

每个阶段完成以后必须提交 Git。

建议：

```text
v0.1.0-phase0
v0.2.0-phase1
v0.3.0-phase2
v0.4.0-phase3
v0.5.0-phase4
v0.6.0-phase5
v1.0.0
```

Commit 信息必须清晰。

例如：

```text
feat: initialize equipment management project
feat: implement excel equipment import
feat: implement equipment ledger
feat: implement equipment transactions
feat: implement dashboard and backup
```

---

# 20. AI 开发规则

你是本项目的主要开发 Agent。

使用 DSH/Codex 开发时：

### 第一原则

不要一次性生成整个项目。

严格执行：

```text
Phase 0
↓
验收
↓
Phase 1
↓
验收
↓
Phase 2
↓
验收
↓
Phase 3
↓
验收
↓
Phase 4
↓
验收
↓
Phase 5
↓
验收
↓
Phase 6
```

### 第二原则

修改代码之前：

先阅读现有代码。

不要覆盖已有实现。

### 第三原则

不要为了实现功能而引入不必要的技术。

禁止擅自引入：

```text
Docker
PostgreSQL
Redis
Kubernetes
微服务
云服务
```

除非用户明确要求。

### 第四原则

保持 Windows 单机部署简单。

最终目标始终是：

```text
双击 equipment.exe
```

即可运行。

### 第五原则

数据安全优先。

任何涉及：

- 删除设备
- 删除记录
- 清空数据库
- 导入覆盖
- 恢复数据库

的操作，都必须有确认机制。

流转历史原则上禁止删除。

---

# 21. UI 设计原则

整体风格：

> 简洁、专业、现代、适合公司内部管理系统。

不要做花哨的营销网站。

重点：

- 信息密度适中
- 搜索方便
- 表格清晰
- 状态颜色明显
- 操作按钮明确
- 设备状态一眼可见
- 支持中文
- 适合 Windows 浏览器

核心导航：

```text
Dashboard
设备台账
设备流转
外借管理
班组管理
数据导入
数据备份
系统设置
```

---

# 22. 最终用户最关心的问题

系统必须能够快速回答以下问题：

### 问题 1

> 6061 在哪里？

回答：

```text
裁剪一组
```

### 问题 2

> 6061 什么时候给裁剪一组的？

回答：

```text
2026-09-05 14:30
```

### 问题 3

> 6061 以前在哪？

回答：

显示完整流转历史。

### 问题 4

> 哪些设备现在在库？

可以筛选：

```text
状态 = 在库
```

### 问题 5

> 哪些设备在裁剪一组？

可以筛选：

```text
班组 = 裁剪一组
```

### 问题 6

> 哪些设备借出去了？

筛选：

```text
状态 = 外借
```

### 问题 7

> 借给谁了？

显示：

```text
外借方
```

### 问题 8

> 借了多久？

显示：

```text
外借日期
预计归还日期
实际归还日期
```

---

# 23. 开发完成标准

最终系统必须满足：

```text
✓ Windows 单机运行
✓ SQLite 本地存储
✓ Excel 初始化导入
✓ 独立设备编号
✓ 设备台账
✓ 在库状态
✓ 班组状态
✓ 外借状态
✓ 维修状态
✓ 班组交接
✓ 外借
✓ 归还
✓ 完整流转历史
✓ Dashboard
✓ 搜索
✓ 筛选
✓ Excel 导出
✓ 自动备份
✓ 数据恢复
✓ Windows EXE
```

最终用户只需要：

```text
双击 equipment.exe
```

即可使用。

---

# 24. 现在开始执行

当前只执行：

## Phase 0

第一步：

完整读取并分析：

```text
设备借出总账.xlsx
```

然后输出：

```text
1. Excel 数据结构分析
2. 字段含义分析
3. 设备编号识别规则
4. 数据异常报告
5. 数据库设计
6. 设备状态机
7. Excel 导入规则
8. Phase 1 开发计划
```

**不要提前进入 Phase 1。**

完成 Phase 0 后暂停，等待用户确认。