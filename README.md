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
| v0.2.0-phase1 | 项目骨架（本版）：Go 后端 + SQLite 迁移 + REST 框架 + React 骨架 + exe 可运行 |
