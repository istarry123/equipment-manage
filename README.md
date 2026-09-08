# 设备资产与流转管理系统

Windows 单机设备资产与流转管理：以单个设备编号为唯一身份，管理设备台账、班组流转、外借归还、维修报废与完整历史（详见 `docs/`）。

- 项目治理：`AGENTS.md`（决策基线 14 项 + 内嵌双 Skill）
- 需求/数据/设计文档：`docs/`（Phase 0 产物）

## 技术栈

- 后端：Go + Gin + GORM + SQLite（**纯 Go 驱动 modernc/glebarez，无 CGO**，go:embed 内嵌前端）
- 前端：React + TypeScript + Vite + Ant Design（构建目标 chrome109）
- 存储：本地 `equipment.db`；备份 `backup/`；日志 `logs/`

## 目录结构

```text
cmd/equipment       程序入口
internal/
  api/              Gin 路由（JSON API + SPA 静态资源）
  browser/          启动自动打开浏览器（Win7 优先 Chrome）
  config/           config.yaml 读写
  database/         连接 + user_version 迁移器 + migrations/*.sql
  logger/           日志（控制台+文件）
  models/           GORM 模型
  webui/            //go:embed dist（由 web/ 构建输出）
web/                React 前端工程
docs/               阶段文档
```

## 构建与运行

前置（仅构建机需要，最终用户不需要）：Go ≥1.20、Node ≥18。

```powershell
# 1) 构建前端（输出到 internal/webui/dist）
cd web
npm install
npm run build
cd ..

# 2) 后端测试
go test ./...

# 3) 编译
go build -o equipment.exe ./cmd/equipment

# 4) 运行：双击 equipment.exe 或
.\equipment.exe          # 自动打开 http://localhost:8080
```

- 环境变量 `EQ_NO_BROWSER=1`：启动时不自动打开浏览器（测试用）
- 环境变量 `DSH_DEBUG=1`：gin 调试模式
- 首次启动自动创建 `equipment.db`（含版本化迁移）与默认 `config.yaml`

## Windows 7 交付构建（决策基线 11）

交付工具链锁定 **Go ≤ 1.20**（最后支持 Win7），且无 CGO 依赖：

```powershell
$env:GOTOOLCHAIN = 'go1.20.14'   # 自动下载该工具链
go build -o equipment.exe ./cmd/equipment
```

Win7 机器需 Chrome 109 / Firefox ESR 115（交付物随附 Chrome 109 离线安装包，Phase 6 落地）。

## 版本与阶段

| 版本 | 说明 |
|---|---|
| v0.1.0-phase0 | 需求与数据分析（docs/ 六份） |
| v0.2.0-phase1 | 项目骨架：Go 后端 + SQLite 迁移 + REST 框架 + React 骨架 + exe 可运行 |
| v0.3.0-phase2 | Excel 数据导入（真实文件 2147→1477 台入库验证） |
| v0.4.0-phase3 | 设备台账（搜索/筛选/详情/新增/编辑/受限更正） |
| v0.5.0-phase4 | 设备流转（八类动作+外借单+历史时间线+班组/外借方管理） |
| v0.6.0-phase5 | Dashboard 统计、Excel 导出、备份/恢复 |
| **v1.0.0** | **Windows 交付版**：Go1.20.14 发布构建 + 用户手册 + 发布脚本 |

## 发布构建（Windows）

```powershell
powershell -ExecutionPolicy Bypass -File scripts\build-release.ps1
# 产物：release\equipment\（equipment.exe + config.yaml + 用户手册 + backup/logs/install）
```

- Win7 兼容：发布脚本用 `GOTOOLCHAIN=go1.20.14` + `CGO_ENABLED=0` 构建（决策 11）；
- Win7 需 Chrome 109 / Firefox ESR 115（获取方式见 `docs/user-guide.md` 第 1 节）。

## 最终使用结构

```text
equipment/
├── equipment.exe     双击即用（自动建库、自动备份、自动开浏览器）
├── equipment.db      运行生成（勿手工编辑）
├── config.yaml       端口等配置（缺失自动生成）
├── user-guide.md     用户手册
├── backup/           备份（启动自动 + 手动；保留最近 30 份）
└── logs/             日志
```
