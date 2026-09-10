# Equipment Flow Export v1.1
## 设备流转情况 Excel 导出功能——完整实现任务

你现在负责为当前设备管理系统实现一个完整的：

> **设备流转情况 Excel 导出功能（Equipment Flow Export）**

这不是简单增加一个 Excel Export 按钮。

必须将：

```text
设备当前状态
+
所属班组
+
当前外借公司
+
当前外借日期
+
历史流转记录
+
统计汇总
```

统一导出为一个结构化 Excel 文件。

---

# 一、项目现状

当前项目：

- Backend：Go
- Framework：Gin
- ORM：GORM
- Database：SQLite
- Frontend：React + TypeScript
- Build：Vite
- UI：Ant Design
- Excel：使用项目现有 Excel 库；如果当前没有统一 Excel 导出库，优先使用 `excelize`
- 前端静态资源最终通过 Go embed 打包
- 本地 Windows 单机运行

当前已经存在：

- Equipment Ledger
- Equipment Detail
- Borrowing
- Team Management
- Team Equipment
- Excel Import
- Backup / Restore
- Settings
- Flow / Transaction history

设备身份原则：

```text
equipment.id = 唯一真实设备身份
```

设备编号：

```text
equipment.equipment_no
```

不是唯一键。

重复编号通过：

```text
equipment_seq
```

和：

```text
display_no
```

进行展示。

---

# 二、本功能目标

新增：

```text
设备流转情况导出
```

最终用户可以通过系统导出：

```text
设备流转情况.xlsx
```

Excel 至少包含三个 Sheet：

```text
Sheet 1：当前流转情况
Sheet 2：流转历史
Sheet 3：统计汇总
```

---

# 三、最重要的数据来源原则

必须严格遵守：

## 当前状态

来自数据库：

```text
equipment
team
borrow_record
borrower
```

## 历史流转

来自：

```text
flow_record
```

## 不允许

导出时重新读取：

```text
设备借出总账.xlsx
```

也不允许重新解析原始 Excel。

原因：

> Excel 是初始化数据来源，而数据库才是系统运行后的真实数据源。

---

# 四、Sheet 1：当前流转情况

Sheet 名：

```text
当前流转情况
```

一行 = 一台真实设备。

绝对不能：

```text
一行 = 一个设备编号
```

因为：

```text
6041
6041
6041
6041
```

可能是四台真实设备。

---

# 五、Sheet 1 字段设计

建议字段：

| 列 | 字段 | 数据来源 |
|---|---|---|
| A | 序号 | 导出生成 |
| B | 设备名称 | equipment.name |
| C | 设备型号 | equipment.model |
| D | 设备编号 | display_no |
| E | 原始编号 | equipment.equipment_no |
| F | 所属班组 | current_team → team.name |
| G | 当前状态 | equipment.status 中文化 |
| H | 当前所在位置 | 后端计算 |
| I | 外借公司 | current borrower/company |
| J | 外借日期 | current borrow record |
| K | 备注 | equipment.remark |

---

# 六、display_no 必须统一使用

不要前端重新拼：

```text
6041（1）
6041（2）
```

必须由后端统一提供：

```text
display_no
```

规则：

### 编号唯一

```text
6041
```

### 编号重复

```text
6041（1）
6041（2）
6041（3）
6041（4）
```

### 无编号

例如：

```text
model = JUKI DDL-8700
```

显示：

```text
JUKI DDL-8700（1）
JUKI DDL-8700（2）
JUKI DDL-8700（3）
```

---

# 七、当前状态中文映射

Excel 不要直接显示：

```text
IN_STOCK
IN_TEAM
BORROWED
MAINTENANCE
SCRAPPED
```

而应该显示中文。

例如：

```text
IN_STOCK    → 仓库
IN_TEAM     → 班组使用
BORROWED    → 外借
MAINTENANCE → 维修
SCRAPPED    → 报废
```

如果项目已经有统一状态字典，必须复用现有字典。

不要在导出模块重新创建另一套状态映射。

---

# 八、当前所在位置 current_location

不要在数据库中增加：

```text
current_location
```

不要造成重复存储。

后端 DTO 中计算即可。

规则：

```text
如果 status = IN_TEAM
    current_location = team.name

如果 status = BORROWED
    current_location = borrower/company.name

如果 status = IN_STOCK
    current_location = "仓库"

如果 status = MAINTENANCE
    current_location = "维修"

如果 status = SCRAPPED
    current_location = "报废"

否则：
    current_location = "未分配"
```

如果当前项目已经有更加准确的位置模型：

> 优先复用现有模型，不要重复造字段。

---

# 九、外借公司

如果设备当前：

```text
status = BORROWED
```

则：

```text
外借公司 = 当前有效 borrow_record 对应 borrower/company
```

如果当前不是外借：

```text
外借公司 = 空
```

不要把历史曾经借过的公司填入这里。

这是：

> 当前外借公司

不是：

> 历史最后一个借用公司。

---

# 十、外借日期

如果当前设备：

```text
BORROWED
```

则：

```text
外借日期 = 当前有效 borrow_record.borrow_date
```

如果没有当前借出：

```text
空
```

不要使用：

```text
flow_record 中最近一次 BORROW
```

来代替当前借出日期。

---

# 十一、当前流转情况示例

最终 Sheet 1 应类似：

| 序号 | 设备名称 | 型号 | 设备编号 | 原始编号 | 班组 | 状态 | 所在位置 | 外借公司 | 外借日期 | 备注 |
|---:|---|---|---|---|---|---|---|---|---|---|
| 1 | 缝纫机 | DDL-8700 | 6041（1） | 6041 | A班 | 班组使用 | A班 | | | |
| 2 | 缝纫机 | DDL-8700 | 6041（2） | 6041 | B班 | 班组使用 | B班 | | | |
| 3 | 缝纫机 | DDL-8700 | 6041（3） | 6041 | | 仓库 | 仓库 | | | |
| 4 | 缝纫机 | DDL-8700 | 6041（4） | 6041 | | 外借 | 莒县双发 | 莒县双发 | 2026-08-20 | |
| 5 | 熨斗 | T-3NS | T-3NS（1） | NULL | C班 | 班组使用 | C班 | | | |

---

# 十二、Sheet 2：流转历史

Sheet 名：

```text
流转历史
```

这个 Sheet 用来回答：

> 一台设备以前经历过什么？

一行 = 一条 flow_record。

---

# 十三、Sheet 2 字段

建议：

| 列 | 字段 |
|---|---|
| A | 序号 |
| B | 设备名称 |
| C | 设备型号 |
| D | 设备编号 |
| E | 流转时间 |
| F | 流转类型 |
| G | 原位置 |
| H | 新位置 |
| I | 班组 |
| J | 外借公司 |
| K | 操作备注 |

如果现有 flow_record 已经有更准确字段：

> 优先使用现有字段。

不要为了导出重复增加数据库字段。

---

# 十四、流转类型中文化

例如：

```text
BORROW
RETURN
TEAM_TRANSFER
WAREHOUSE_IN
WAREHOUSE_OUT
MAINTENANCE
SCRAP
```

映射：

```text
BORROW        → 外借
RETURN        → 归还
TEAM_TRANSFER → 班组调拨
WAREHOUSE_IN  → 入库
WAREHOUSE_OUT → 出库
MAINTENANCE   → 维修
SCRAP         → 报废
```

必须复用项目已有 Transaction / Flow 类型定义。

---

# 十五、流转历史必须保持时间顺序

Sheet 2 默认：

```text
最新 → 最旧
```

或者：

```text
最旧 → 最新
```

二选一。

我推荐：

```text
最新 → 最旧
```

方便管理人员查看最近发生的流转。

但必须保证：

> 同一设备的历史顺序准确。

---

# 十六、历史数据必须使用快照

如果 flow_record 已经保存：

```text
equipment_name_snapshot
equipment_model_snapshot
equipment_no_snapshot
```

优先使用 snapshot。

不要因为设备后来改名，导致十年前的历史记录名称全部被修改。

---

# 十七、Sheet 3：统计汇总

Sheet 名：

```text
统计汇总
```

至少包含三个统计区域。

---

# 十八、按状态统计

例如：

| 当前状态 | 设备数量 |
|---|---:|
| 班组使用 | 1050 |
| 仓库 | 530 |
| 外借 | 87 |
| 维修 | 21 |
| 报废 | 15 |

统计必须按照：

```text
equipment
```

实际记录统计。

不能按照：

```text
equipment_no
```

统计。

---

# 十九、按班组统计

例如：

| 班组 | 设备数量 |
|---|---:|
| A班 | 215 |
| B班 | 187 |
| C班 | 203 |
| D班 | 165 |

只统计当前属于班组的设备。

---

# 二十、按外借公司统计

例如：

| 外借公司 | 当前设备数量 |
|---|---:|
| 莒县双发 | 32 |
| XX公司 | 18 |
| XX贸易 | 12 |

只统计：

```text
status = BORROWED
```

的设备。

---

# 二十一、增加导出时间

Sheet 3 顶部显示：

```text
设备流转情况统计

导出时间：2026-09-09 14:30:00
```

时间使用：

```text
time.Now()
```

只用于：

> Excel 文件生成时间。

不要把它当成设备流转时间。

---

# 二十二、前端增加入口

在现有：

```text
设备台账
```

页面增加：

```text
[ 导出设备台账 ] [ 导出流转情况 ]
```

不要删除原来的设备台账导出。

---

# 二十三、导出按钮交互

点击：

```text
导出流转情况
```

打开 Ant Design Modal。

---

# 二十四、导出筛选条件

Modal：

```text
┌───────────────────────────────┐
│       导出设备流转情况        │
│                               │
│ 班组：   [ 全部 ▼ ]           │
│ 状态：   [ 全部 ▼ ]           │
│ 外借公司：[ 全部 ▼ ]          │
│                               │
│ 数据范围：                    │
│ ● 当前设备                    │
│ ○ 流转历史                    │
│ ○ 当前 + 历史                 │
│                               │
│ ☑ 包含统计汇总                │
│                               │
│       [取消] [导出 Excel]     │
└───────────────────────────────┘
```

默认：

```text
班组 = 全部
状态 = 全部
外借公司 = 全部
数据范围 = 当前 + 历史
包含统计汇总 = true
```

---

# 二十五、筛选必须作用于当前 Sheet

例如：

```text
班组 = A班
```

Sheet 1：

> 只导出 A班当前设备。

Sheet 2：

> 只导出这些设备的历史流转。

Sheet 3：

> 统计范围也必须与当前筛选一致。

---

# 二十六、外借公司筛选

例如：

```text
外借公司 = 莒县双发
```

Sheet 1：

只显示：

```text
当前外借给莒县双发
```

Sheet 2：

只显示：

```text
这些设备的历史流转
```

而不是把所有历史上曾经去过莒县双发的设备都算进去。

---

# 二十七、建议增加一个快速导出

在设备台账页面可以提供：

```text
导出全部
```

也可以提供：

```text
导出当前筛选结果
```

如果页面本身已经存在筛选器：

```text
设备名称
型号
编号
状态
班组
```

可以支持：

```text
导出当前筛选结果
```

---

# 二十八、API 设计

新增：

```http
GET /api/equipment/export/flow
```

或者根据项目已有 API 命名规范选择：

```http
GET /api/equipment/flow-export
```

不要与已有设备台账导出 API 冲突。

---

# 二十九、Query 参数

建议：

```text
team_id
status
borrower_id
include_current
include_history
include_summary
```

例如：

```text
GET /api/equipment/export/flow
    ?team_id=3
    &status=IN_TEAM
    &include_current=true
    &include_history=true
    &include_summary=true
```

如果项目已有统一 Filter DTO：

> 复用现有 Filter DTO。

---

# 三十、API 不要返回 JSON 文件数据

这是文件下载 API。

返回：

```http
Content-Type:
application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
```

设置：

```http
Content-Disposition:
attachment; filename="设备流转情况.xlsx"
```

如果项目已有文件名编码工具：

> 复用。

注意 Windows / Chrome / Edge 下载时中文文件名必须正常。

---

# 三十一、Backend Service

建议新增：

```text
service/equipment_flow_export.go
```

或按照当前项目 service 目录规范。

职责：

```text
Filter
 ↓
Query equipment
 ↓
Load team
 ↓
Load borrower
 ↓
Load current borrow record
 ↓
Calculate display_no
 ↓
Build current sheet
 ↓
Query flow_record
 ↓
Build history sheet
 ↓
Build summary
 ↓
Generate XLSX
```

不要把业务查询全部写进 Gin Handler。

---

# 三十二、DTO

建议：

```go
type EquipmentFlowExportRow struct {
    ID           uint
    Name         string
    Model        string
    DisplayNo    string
    EquipmentNo  *string
    TeamName     string
    Status       string
    Location     string
    BorrowerName string
    BorrowDate   *time.Time
    Remark       string
}
```

历史：

```go
type EquipmentFlowHistoryExportRow struct {
    EquipmentID   uint
    Name          string
    Model         string
    DisplayNo     string
    OccurredAt    time.Time
    FlowType      string
    FromLocation  string
    ToLocation    string
    TeamName      string
    BorrowerName  string
    Remark        string
}
```

根据当前项目实际模型调整。

---

# 三十三、避免 N+1 查询

禁止：

```go
for _, equipment := range equipments {
    db.First(&team, equipment.TeamID)
}
```

必须预加载或批量查询：

```text
equipment
team
borrower
borrow_record
```

历史记录同样避免 N+1。

---

# 三十四、Excel 生成

如果当前项目已经使用：

```text
excelize
```

直接复用。

如果没有：

```text
excelize
```

作为优先方案。

不要引入重量级 Excel 框架。

---

# 三十五、Excel 样式

需要基本的专业 Excel 样式。

## 标题

Sheet 1：

```text
设备当前流转情况
```

Sheet 2：

```text
设备流转历史
```

Sheet 3：

```text
设备流转统计汇总
```

标题加粗、合并。

---

# 三十六、表头样式

表头：

- 加粗
- 居中
- 自动换行
- 设置合理列宽

不要指定非常花哨的颜色。

保持企业内部管理系统的简洁风格。

---

# 三十七、冻结表头

Sheet 1：

冻结正式数据表头。

Sheet 2：

冻结正式数据表头。

Sheet 3：

根据统计区域结构决定。

---

# 三十八、自动筛选

Sheet 1：

开启 AutoFilter。

Sheet 2：

开启 AutoFilter。

方便用户在 Excel 中：

```text
筛选 A班
筛选 外借
筛选 莒县双发
```

---

# 三十九、日期格式

统一：

```text
yyyy-mm-dd
```

如果包含时间：

```text
yyyy-mm-dd hh:mm:ss
```

不要输出：

```text
2026/9/9
```

和：

```text
2026-9-9 14:3
```

这种混乱格式。

---

# 四十、数字与文本

设备编号：

```text
TEXT
```

非常重要。

例如：

```text
001
002
003
```

绝对不能因为 Excel 自动类型转换变成：

```text
1
2
3
```

所以：

> 设备编号单元格必须作为文本写入。

---

# 四十一、重复编号导出

例如：

```text
equipment_no = 6041
equipment_seq = 1
```

导出：

```text
设备编号 = 6041（1）
原始编号 = 6041
```

第二台：

```text
设备编号 = 6041（2）
原始编号 = 6041
```

这样既方便人工查看，又保留原始数据。

---

# 四十二、无编号导出

例如：

```text
model = JUKI DDL-8700
equipment_no = NULL
equipment_seq = 1
```

导出：

```text
设备编号 = JUKI DDL-8700（1）
原始编号 = 空
```

不能写：

```text
设备编号 = 无编号
```

---

# 四十三、导出排序

Sheet 1 推荐排序：

```text
设备名称
→ 型号
→ equipment_no
→ equipment_seq
```

如果存在班组筛选：

```text
班组
→ 设备名称
→ 型号
→ equipment_no
→ equipment_seq
```

Sheet 2：

```text
设备编号
→ 流转时间 DESC
```

或者：

```text
流转时间 DESC
→ 设备编号
```

保持稳定排序。

---

# 四十四、隐藏技术字段

可以在 Sheet 1 最后增加：

```text
equipment_id
```

但默认隐藏。

建议：

```text
L = equipment_id
```

设置：

```text
hidden = true
```

用途：

> 数据核对和后续技术处理。

如果当前项目不需要，可以不导出。

---

# 四十五、不要把数据库 ID 当成业务编号

Excel 用户看到：

```text
设备编号 = 6041（1）
```

而不是：

```text
设备编号 = 1023
```

`equipment.id` 只能作为隐藏技术字段。

---

# 四十六、导出文件名

推荐：

```text
设备流转情况_2026-09-09.xlsx
```

如果包含时间：

```text
设备流转情况_2026-09-09_143000.xlsx
```

如果有筛选：

```text
设备流转情况_A班_2026-09-09.xlsx
```

但不要让文件名逻辑过度复杂。

优先：

```text
设备流转情况_YYYY-MM-DD.xlsx
```

---

# 四十七、空数据处理

如果筛选：

```text
莒县双发
```

但当前没有设备：

不要返回 500。

应该正常生成 Excel：

```text
当前流转情况
```

并写：

```text
当前筛选条件下没有设备。
```

统计 Sheet：

```text
设备数量 = 0
```

---

# 四十八、权限

当前系统没有复杂 RBAC。

因此：

> 不新增权限体系。

如果已有权限中间件：

> 遵循现有权限机制。

---

# 四十九、审计

如果系统已经有 audit：

导出动作建议记录：

```text
action = EXPORT_EQUIPMENT_FLOW
```

包括：

```text
时间
筛选条件
导出数量
```

不要记录完整 Excel 二进制内容。

---

# 五十、性能要求

当前设备数量约 2000 级别。

导出必须能够一次处理：

```text
2000+
```

设备。

历史记录如果达到：

```text
数千 / 数万
```

也应该正常。

避免：

```text
一台设备一次 SQL
```

这种 N+1。

---

# 五十一、前端下载处理

前端使用：

```text
Blob
```

接收后端文件。

正确处理：

```text
Content-Disposition
```

并触发浏览器下载。

不要：

```text
window.open(JSON API)
```

---

# 五十二、前端 Loading

点击导出：

```text
loading = true
```

按钮显示：

```text
正在导出...
```

成功：

```text
导出成功
```

失败：

```text
导出失败，请检查系统日志。
```

最后：

```text
loading = false
```

---

# 五十三、前端禁止重复点击

导出过程中：

```text
disabled = true
```

防止用户连续点击生成多个 Excel。

---

# 五十四、错误处理

后端错误统一返回项目现有格式。

例如：

```json
{
  "code": "...",
  "message": "..."
}
```

不要创建另一套错误响应结构。

---

# 五十五、测试

必须增加 Backend Test。

---

# 五十六、测试 1：当前班组设备

创建：

```text
equipment A
team = A班
status = IN_TEAM
```

导出。

期待：

```text
所属班组 = A班
当前状态 = 班组使用
当前位置 = A班
```

---

# 五十七、测试 2：当前外借设备

创建：

```text
equipment = 6041
status = BORROWED
borrower = 莒县双发
borrow_date = 2026-08-20
```

导出：

期待：

```text
状态 = 外借
当前位置 = 莒县双发
外借公司 = 莒县双发
外借日期 = 2026-08-20
```

---

# 五十八、测试 3：已归还设备

历史：

```text
BORROW
RETURN
```

当前：

```text
IN_STOCK
```

导出：

期待：

```text
外借公司 = 空
外借日期 = 空
当前位置 = 仓库
```

但 Sheet 2：

```text
仍然可以看到 BORROW
仍然可以看到 RETURN
```

---

# 五十九、测试 4：重复编号

创建：

```text
6041 × 4
```

导出：

必须出现：

```text
6041（1）
6041（2）
6041（3）
6041（4）
```

不能出现：

```text
6041
6041
6041
6041
```

---

# 六十、测试 5：无编号

创建：

```text
JUKI DDL-8700
equipment_no = NULL
seq = 1
2
3
```

导出：

```text
JUKI DDL-8700（1）
JUKI DDL-8700（2）
JUKI DDL-8700（3）
```

---

# 六十一、测试 6：历史流转

设备：

```text
6041（1）
```

创建：

```text
2024-01-01 BORROW
2024-02-01 RETURN
2025-03-01 TEAM_TRANSFER
2026-08-20 BORROW
```

Sheet 2 必须有 4 条。

不能只保留最后一条。

---

# 六十二、测试 7：筛选

筛选：

```text
status = BORROWED
```

Sheet 1：

只能出现：

```text
BORROWED
```

设备。

Sheet 3：

状态统计也必须匹配筛选结果。

---

# 六十三、测试 8：班组筛选

筛选：

```text
A班
```

Sheet 1：

只出现：

```text
A班
```

设备。

---

# 六十四、测试 9：外借公司筛选

筛选：

```text
莒县双发
```

Sheet 1：

只出现当前外借给：

```text
莒县双发
```

的设备。

---

# 六十五、测试 10：中文编号

创建：

```text
equipment_no = "车间A-01"
```

导出：

必须保持：

```text
车间A-01
```

不能变成：

```text
车间A1
```

或者被 Excel 转换。

---

# 六十六、测试 11：001 编号

创建：

```text
equipment_no = "001"
```

导出后必须仍然：

```text
001
```

而不是：

```text
1
```

---

# 六十七、测试 12：空筛选

使用：

```text
一个不存在的班组
```

导出仍然成功。

Excel 正常打开。

---

# 六十八、测试 13：2000 台设备

构造约：

```text
2000
```

设备。

测试导出：

```text
<合理时间
```

并检查：

```text
Excel 行数 = equipment 数量
```

---

# 六十九、集成测试

至少验证：

```text
创建设备
→ 分配班组
→ 借出
→ 归还
→ 再借出
→ 导出
```

最终：

Sheet 1：

显示当前状态。

Sheet 2：

保留完整历史。

Sheet 3：

统计当前状态。

---

# 七十、与现有功能回归

导出功能不能破坏：

- 设备台账
- 设备详情
- 借出
- 归还
- 班组调拨
- Team Equipment
- Excel Import
- Backup
- Restore
- Dashboard
- Search

执行：

```bash
go test ./...
```

以及前端实际测试命令。

执行：

```bash
npm run build
```

执行：

```bash
go build ./...
```

---

# 七十一、代码结构建议

推荐：

```text
backend/
├── handler/
│   └── equipment_export.go
│
├── service/
│   └── equipment_flow_export.go
│
├── dto/
│   └── equipment_export.go
│
└── ...
```

前端：

```text
src/
├── pages/
│   └── Equipment/
│       └── FlowExportModal.tsx
│
├── services/
│   └── equipment.ts
│
└── ...
```

但必须遵循当前项目真实目录结构。

不要为了符合上述示例而强行移动文件。

---

# 七十二、实施前必须检查现有代码

首先执行：

```bash
git status
git log -5
```

搜索：

```text
equipment
equipment_no
equipment_seq
display_no
flow_record
borrow_record
current_team_id
current_borrow_record_id
export
excel
excelize
```

确认：

1. 当前 Equipment Model
2. 当前 Flow Model
3. 当前 Borrow Model
4. 当前 Team Model
5. 当前导出功能
6. 当前 API 路由
7. 当前前端设备页面
8. 当前状态枚举

然后再开始修改。

---

# 七十三、不要重复造已有能力

如果项目已经存在：

```text
ExcelExportService
EquipmentFilter
FlowService
StatusLabel
DisplayNo
```

必须复用。

不要创建：

```text
第二套 Excel 工具
第二套状态转换
第二套 display_no
```

---

# 七十四、与之前 v1.1 数据模型修改保持兼容

本功能必须兼容之前确定的数据模型：

```text
equipment.id
equipment.equipment_no
equipment.equipment_seq
equipment.internal_code
equipment.current_team_id
equipment.current_borrower_id
equipment.current_borrow_record_id
equipment.current_since
equipment.status
```

以及：

```text
flow_record
borrow_record
team
borrower
```

---

# 七十五、绝对不要通过 equipment_no 关联历史

例如：

```text
6041
```

存在：

```text
6041（1）
6041（2）
6041（3）
6041（4）
```

历史记录必须：

```text
equipment_id
```

关联。

不能：

```text
WHERE equipment_no = "6041"
```

否则会把四台设备的历史混在一起。

---

# 七十六、导出历史时必须以 equipment_id 为核心

正确：

```text
flow_record.equipment_id
        ↓
equipment.id
        ↓
display_no
```

错误：

```text
flow_record.equipment_no
        ↓
equipment_no
```

---

# 七十七、最终 Excel 验收

完成后必须实际打开生成的：

```text
设备流转情况_YYYY-MM-DD.xlsx
```

验证：

### Sheet 1

- [ ] 每台设备一行
- [ ] 设备名称正确
- [ ] 型号正确
- [ ] display_no 正确
- [ ] 原始编号正确
- [ ] 班组正确
- [ ] 状态正确
- [ ] 当前所在位置正确
- [ ] 外借公司正确
- [ ] 外借日期正确
- [ ] 备注正确

### Sheet 2

- [ ] 历史记录完整
- [ ] 设备对应正确
- [ ] 日期正确
- [ ] 流转类型正确
- [ ] 原位置正确
- [ ] 新位置正确
- [ ] 外借公司正确

### Sheet 3

- [ ] 状态统计正确
- [ ] 班组统计正确
- [ ] 外借公司统计正确

---

# 七十八、最终必须做数据库与 Excel 对账

例如：

```text
数据库当前设备数量：2147
Excel Sheet1 数据设备数量：2147
```

必须一致。

如果有筛选：

```text
数据库筛选数量：87
Excel Sheet1：87
```

必须一致。

历史：

```text
数据库符合条件 flow_record：XXXX
Excel Sheet2：XXXX
```

必须一致。

---

# 七十九、最终报告

完成后输出：

## 1. 修改文件

列出：

```text
文件
修改内容
```

## 2. API

列出：

```text
GET /api/...
```

参数。

## 3. Excel 结构

说明三个 Sheet。

## 4. 数据统计

例如：

```text
设备总数：2147
当前班组：XXXX
当前外借：XXXX
仓库：XXXX
维修：XXXX
历史流转：XXXX
```

## 5. 测试

逐项：

```text
PASS / FAIL
```

## 6. 构建

```text
go test ./...
PASS

go build ./...
PASS

npm run build
PASS
```

## 7. Git

最后：

```bash
git status
git diff
```

确认没有无关修改。

---

# 八十、最终验收标准

只有以下全部满足才算完成：

- [ ] 增加“导出流转情况”
- [ ] 不影响原设备台账导出
- [ ] 当前设备一台一行
- [ ] 使用 equipment.id 作为真实身份
- [ ] 不使用 equipment_no 作为唯一键
- [ ] 重复编号正确显示
- [ ] 无编号正确显示
- [ ] 001 不被 Excel 转成 1
- [ ] 中文编号正确保留
- [ ] 当前班组正确
- [ ] 当前状态正确
- [ ] 当前所在位置正确
- [ ] 当前外借公司正确
- [ ] 当前外借日期正确
- [ ] 已归还设备不会显示当前外借公司
- [ ] 历史流转完整
- [ ] 历史流转按照 equipment_id 关联
- [ ] 历史设备名称使用 snapshot
- [ ] 支持班组筛选
- [ ] 支持状态筛选
- [ ] 支持外借公司筛选
- [ ] 支持当前 + 历史
- [ ] 支持统计汇总
- [ ] Excel 样式正常
- [ ] Excel 可筛选
- [ ] Excel 表头冻结
- [ ] 日期格式统一
- [ ] 不产生 N+1 查询
- [ ] 2000+ 设备正常导出
- [ ] 空结果正常导出
- [ ] 导出操作有 Loading
- [ ] 防止重复点击
- [ ] 错误提示正常
- [ ] 审计正常（如果已有 audit）
- [ ] 后端测试通过
- [ ] 前端构建通过
- [ ] Go 构建通过
- [ ] 数据库与 Excel 数量完成对账

---

# 八十一、最终设计原则

整个功能必须遵循：

```text
                    ┌───────────────┐
                    │   equipment   │
                    │   当前设备    │
                    └───────┬───────┘
                            │
            ┌───────────────┼────────────────┐
            ↓               ↓                ↓
          team          borrow_record      status
            │               │                │
            └───────────────┼────────────────┘
                            ↓
                     当前流转情况
                            │
                            │
                     flow_record
                            ↓
                       流转历史
                            │
                            ↓
                       统计汇总
                            │
                            ↓
                  设备流转情况.xlsx
```

最终必须实现：

> **数据库是唯一真实数据源，equipment.id 是唯一设备身份，display_no 负责人工识别，equipment 当前字段负责当前状态，flow_record 负责历史状态，Excel 只是数据库当前状态和历史流转的可视化导出。**

不要通过重新读取原始 Excel 来生成流转情况。

不要通过 equipment_no 判断设备身份。

不要因为重复编号而合并设备。

不要因为无编号而合并设备。

不要因为历史借出而错误显示当前外借。

不要丢失历史记录。