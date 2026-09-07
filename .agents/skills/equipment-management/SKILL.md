---
name: equipment-management
description: >
  Windows 单机设备资产与流转管理系统开发 Skill。
  用于基于 Excel 设备台账构建设备编号、设备状态、班组归属、
  外借、归还、流转历史、Dashboard、Excel 导入导出和本地 SQLite
  数据管理系统。适用于 DSH、Codex 等代码 Agent。
---

# Equipment Management Skill

## 1. Skill 定位

本 Skill 用于开发：

> 设备资产与流转管理系统

系统用于公司内部设备管理。

核心目标：

> 能够随时查询某一个具体设备编号当前在哪里、属于哪个班组、
> 什么时间交给该班组；如果设备外借，则知道外借日期、
> 外借方、预计归还日期和实际归还日期；如果没有使用，
> 则显示为“在库”。

系统第一阶段的数据来源是：

`设备借出总账.xlsx`

Excel 仅用于系统初始化数据导入。

系统正式运行以后：

禁止继续把 Excel 作为数据库。

所有设备状态和流转信息必须通过系统管理。

---

# 2. 部署目标

本系统是：

- Windows 单机应用
- 单台电脑使用
- 本地运行
- SQLite 数据库
- 不依赖 Linux
- 不依赖服务器
- 不依赖 Docker
- 不依赖 PostgreSQL
- 不依赖 Redis
- 不依赖互联网
- 不依赖云服务

最终用户体验：

```text
双击 equipment.exe
        ↓
启动本地 Web 服务
        ↓
自动打开浏览器
        ↓
http://localhost:8080
```

数据：

```text
equipment.db
```

备份：

```text
backup/
```

---

# 3. 技术栈

## Backend

- Go
- Gin
- SQLite
- GORM
- Go embed

## Frontend

- React
- TypeScript
- Vite
- Ant Design
- ECharts

## Development

- Git
- DSH
- Codex

---

# 4. 架构原则

采用：

> 单体应用 + 本地 SQLite

禁止无必要地引入：

- 微服务
- Docker
- Kubernetes
- PostgreSQL
- Redis
- Kafka
- Nginx
- 云数据库
- 云服务

如果未来确实需要扩展，必须由用户明确提出。

---

# 5. 核心业务

系统核心不是 Excel 管理，而是：

> 设备生命周期与设备流转管理。

每一个有真实设备编号的设备必须作为独立设备实体。

例如：

```text
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

必须对应 9 条独立设备记录。

不能只保存：

```text
环形割刀
数量 = 9
```

---

# 6. 设备状态

第一版支持：

```text
IN_STOCK
```

在库。

```text
IN_TEAM
```

班组使用。

```text
BORROWED
```

外借。

```text
MAINTENANCE
```

维修。

```text
SCRAPPED
```

报废。

```text
OTHER
```

其他。

前端显示中文：

```text
🟢 在库
🔵 班组使用
🟠 外借
🔴 维修
⚫ 报废
🟣 其他
```

---

# 7. 核心流转

必须支持：

## 仓库 → 班组

```text
IN_STOCK → IN_TEAM
```

记录：

- 班组
- 交付时间
- 操作人
- 备注

## 班组 → 班组

```text
IN_TEAM → IN_TEAM
```

记录：

- 原班组
- 新班组
- 交接时间
- 操作人
- 备注

## 班组 → 仓库

```text
IN_TEAM → IN_STOCK
```

记录：

- 归还时间
- 操作人
- 备注

## 仓库/班组 → 外借

```text
→ BORROWED
```

记录：

- 外借方
- 外借日期
- 预计归还日期
- 操作人
- 备注

## 外借 → 仓库

```text
BORROWED → IN_STOCK
```

记录：

- 实际归还日期
- 操作人
- 备注

---

# 8. 数据模型

核心实体：

```text
equipment
team
borrower
transaction
```

## equipment

至少：

```text
id
equipment_no
internal_code
name
model
category
status
current_team_id
current_borrower_id
remark
created_at
updated_at
```

## team

```text
id
name
department
remark
created_at
updated_at
```

## borrower

```text
id
name
contact
phone
remark
created_at
updated_at
```

## transaction

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

---

# 9. 数据原则

## 当前状态

`equipment` 保存当前状态。

## 历史记录

`transaction` 保存历史过程。

任何状态变化都必须：

```text
更新 equipment
+
新增 transaction
```

不能只更新当前状态。

---

# 10. Excel 原则

Excel 是：

> 初始化数据来源。

不是数据库。

导入流程：

```text
Excel
 ↓
解析
 ↓
数据清洗
 ↓
预览
 ↓
校验
 ↓
用户确认
 ↓
SQLite
```

必须检查：

- 重复设备编号
- 空设备编号
- 多设备编号
- 数量与编号数量不一致
- 空字段
- 异常数据
- 日期异常
- 借出信息异常

不得擅自删除原始数据。

---

# 11. 无编号设备

如果 Excel 中设备没有真实设备编号：

不得伪造财务资产编号。

可以生成：

```text
internal_code
```

例如：

```text
EQ-000001
```

但：

```text
equipment_no = NULL
```

必须保留“无真实设备编号”的事实。

---

# 12. UI

整体风格：

- 简洁
- 专业
- 实用
- 公司内部管理系统
- 信息密度适中
- 表格清晰
- 状态明显

主要页面：

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

# 13. Dashboard

必须能够显示：

- 设备总数
- 在库
- 班组使用
- 外借
- 维修
- 报废
- 各班组设备数量
- 当前外借设备
- 逾期设备
- 最近流转
- 最近归还
- 设备状态统计

---

# 14. 设备查询

系统必须快速回答：

> 6061 在哪里？

例如：

```text
设备编号：6061

环形割刀
EBK-SA

状态：班组使用

当前班组：裁剪一组

交付时间：
2026-09-05 14:30
```

---

# 15. 设备详情

必须显示：

```text
设备基本信息
当前状态
当前位置
当前班组
当前外借方
最后更新时间
```

以及：

```text
完整流转时间线
```

例如：

```text
2026-09-05
仓库 → 裁剪一组

2026-08-20
裁剪二组 → 仓库

2026-08-01
仓库 → 裁剪二组
```

---

# 16. 外借

外借页面必须显示：

```text
设备编号
设备名称
外借方
外借日期
预计归还日期
实际归还日期
当前状态
```

必须能够发现：

```text
逾期未归还设备
```

---

# 17. 数据安全

禁止：

- 无确认删除设备
- 无确认删除历史记录
- 无确认覆盖数据库
- 无确认恢复数据库
- 删除 transaction 历史

流转历史原则上不可删除。

---

# 18. 自动备份

SQLite 数据库：

```text
equipment.db
```

备份：

```text
backup/
```

建议：

```text
程序启动自动备份
用户手动备份
保留最近 30 份
支持恢复
```

恢复必须要求二次确认。

---

# 19. Phase 开发制度

必须严格按照：

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

执行。

禁止跨 Phase。

---

# 20. Phase 0

目标：

分析：

```text
设备借出总账.xlsx
```

输出：

```text
docs/requirements.md
docs/data-analysis.md
docs/database-design.md
docs/state-machine.md
docs/import-rules.md
docs/roadmap.md
```

不得提前开发完整业务代码。

---

# 21. Phase 1

目标：

建立：

```text
Go
SQLite
React
TypeScript
Vite
Ant Design
```

实现：

- 项目骨架
- SQLite 初始化
- migration
- REST API
- React
- 前后端通信
- 日志
- 配置
- 基础测试

---

# 22. Phase 2

目标：

完成 Excel 导入。

必须：

```text
选择 Excel
→
解析
→
预览
→
校验
→
确认
→
导入 SQLite
```

导入报告至少包括：

- 总记录
- 成功
- 跳过
- 重复
- 异常
- 无编号

---

# 23. Phase 3

目标：

完成设备台账。

包括：

- 列表
- 搜索
- 筛选
- 分类
- 设备详情
- 新增
- 编辑
- 当前状态
- 当前班组
- 当前外借方

核心验收：

> 搜索 `6061` 后，能够显示当前状态、当前位置、当前班组和交付/流转时间。

---

# 24. Phase 4

目标：

完成设备流转。

包括：

- 入库
- 出库
- 班组转交
- 外借
- 归还
- 维修
- 维修完成
- 流转历史

每次状态变化必须同时：

```text
更新当前状态
+
写入 transaction
```

---

# 25. Phase 5

目标：

完成：

- Dashboard
- 状态统计
- 班组统计
- 外借统计
- 逾期统计
- Excel 导出
- 自动备份
- 数据恢复

---

# 26. Phase 6

目标：

完成 Windows 最终交付。

最终：

```text
equipment/
├── equipment.exe
├── equipment.db
├── config.yaml
├── backup/
└── logs/
```

用户无需安装：

- Go
- Node
- Python
- Docker
- PostgreSQL
- Linux

---

# 27. 每个 Phase 的执行规则

开始 Phase 前：

1. 阅读当前代码
2. 阅读 docs
3. 阅读本 Phase 文件
4. 检查上一 Phase 是否完成
5. 明确本 Phase 范围

开发：

1. 小步修改
2. 保持已有功能
3. 不跨 Phase
4. 不引入无关技术
5. 编写测试

结束：

1. 运行测试
2. 修复错误
3. 更新文档
4. 输出 Phase 报告
5. Git commit
6. Git tag
7. 停止

---

# 28. AI 行为规范

不要：

```text
看到需求就直接写代码
```

必须：

```text
理解
→
检查
→
设计
→
实现
→
测试
→
验证
```

不要猜测 Excel 字段含义。

无法确定时：

```text
标记为待确认
```

不要擅自改变业务规则。

---

# 29. 当前任务

如果用户没有明确指定 Phase：

不要自行开始开发。

询问：

```text
当前准备执行哪个 Phase？
```

如果用户说：

```text
执行 Phase 0
```

只执行 Phase 0。

如果用户说：

```text
执行 Phase 1
```

检查 Phase 0 是否存在并已完成。

然后执行 Phase 1。

---

# 30. 最终成功标准

系统必须能够：

```text
查询 6061
↓
知道在哪里
↓
知道当前状态
↓
知道哪个班组
↓
知道什么时候交付
↓
知道历史流转
```

如果外借：

```text
知道借给谁
知道什么时候借
知道预计什么时候还
知道是否逾期
知道什么时候实际归还
```

如果在库：

```text
🟢 在库
```

最终用户只需要：

```text
双击 equipment.exe
```

即可使用。

---

# 31. 推荐 Skill 使用方式

在项目根目录加载：

```text
.agents/skills/equipment-management/SKILL.md
```

执行任务时：

```text
加载 equipment-management skill。

检查当前项目状态。

当前执行 Phase 0。

严格按照 equipment-management skill 执行。

读取并分析项目中的：

设备借出总账.xlsx

只执行 Phase 0。

完成后输出 Phase 0 验收报告。

不要进入 Phase 1。
```

后续阶段同理：

```text
加载 equipment-management skill。

检查上一阶段是否完成。

执行 Phase X。

严格按照对应 Phase 规范执行。

完成测试、文档和 Git 提交后停止。

不要自动进入下一 Phase。
```

---

# 32. 强制约束

以下规则优先级最高：

1. 不跨 Phase。
2. 不猜测业务含义。
3. 不擅自改变原始数据语义。
4. 不引入未要求的基础设施。
5. 不删除流转历史。
6. 状态变化必须写 transaction。
7. Excel 只负责初始化导入。
8. 正式数据必须存 SQLite。
9. 危险操作必须确认。
10. 每个 Phase 完成后必须停止等待用户指令。

如果当前任务与上述规则冲突：

> 优先遵守本 Skill。
