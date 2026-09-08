# 设备资产与流转管理系统 - 发布构建脚本（Windows）
# 用法：powershell -ExecutionPolicy Bypass -File scripts\build-release.ps1
# 产物：release\equipment\ 目录（可整体拷到目标 Windows 电脑）
# 工具链：Go ≤1.20（Win7 兼容，决策 11）；无 CGO。

$ErrorActionPreference = 'Stop'
$Root = Split-Path -Parent $PSScriptRoot
$OutDir = Join-Path $Root 'release\equipment'

Write-Host '== 1/4 构建前端 =='
Push-Location (Join-Path $Root 'web')
if (-not (Test-Path 'node_modules')) { npm install }
npm run build
if ($LASTEXITCODE -ne 0) { throw '前端构建失败' }
Pop-Location

Write-Host '== 2/4 用 Go 1.20 工具链编译（Win7 兼容）=='
$env:GOTOOLCHAIN = 'go1.20.14'   # 自动下载该工具链（goproxy 可访问时）
$env:CGO_ENABLED = '0'
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
if (Test-Path (Join-Path $Root '.gomod\pkg\mod')) {
  # 沙箱/离线开发环境：复用工作区内模块缓存
  $env:GOMODCACHE = Join-Path $Root '.gomod\pkg\mod'
  $env:GOPATH = Join-Path $Root '.go'
  $env:GOCACHE = Join-Path $Root '.gocache'
}
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
Push-Location $Root
go build -o (Join-Path $OutDir 'equipment.exe') ./cmd/equipment
if ($LASTEXITCODE -ne 0) { throw 'exe 构建失败' }
Pop-Location

Write-Host '== 3/4 组装交付目录 =='
foreach ($d in @('backup', 'logs')) {
  New-Item -ItemType Directory -Force -Path (Join-Path $OutDir $d) | Out-Null
}
Copy-Item (Join-Path $Root 'config.yaml') $OutDir -Force
Copy-Item (Join-Path $Root 'docs\user-guide.md') $OutDir -Force
New-Item -ItemType Directory -Force -Path (Join-Path $OutDir 'install') | Out-Null

Write-Host '== 4/4 完成 =='
Write-Host ('产物目录: ' + $OutDir)
Get-ChildItem $OutDir -Force | Select-Object Name, Length
Write-Host ''
Write-Host '提示:'
Write-Host '  1) 双击 equipment.exe 即用（首次自动建库/备份/开浏览器）。'
Write-Host '  2) Win7 需浏览器：见 用户手册 第 1 节 / install 目录说明。'
Write-Host '  3) 防火墙：本程序仅监听 127.0.0.1，通常不会触发防火墙询问。'
