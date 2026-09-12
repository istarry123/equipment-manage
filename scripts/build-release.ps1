# 设备资产与流转管理系统 - 发布构建脚本（Windows）
# 用法：powershell -ExecutionPolicy Bypass -File scripts\build-release.ps1
# 产物：release\equipment\ 目录（可整体拷到目标 Windows 电脑）
# 工具链：Go ≤1.20（Win7 兼容，决策 11）；无 CGO。

$ErrorActionPreference = 'Stop'
$Root = Split-Path -Parent $PSScriptRoot
$OutDir = Join-Path $Root 'release\equipment'

# 原生命令统一经此函数调用。
#
# 为什么需要它：PowerShell 5.1 会把**原生命令写入 stderr 的普通输出**包装成错误记录，
# 在 $ErrorActionPreference='Stop' 下直接抛终止性 RemoteException。
# 而 npm 每次构建都会向 stderr 写警告（如 chunk 体积提示），于是脚本会在第一步被
# 「构建成功但写了警告」打断（v1.4 Phase 6 实测复现并修复）。
# 因此这里临时放宽 EAP，改由 $LASTEXITCODE 判定成败——真正的失败仍然会 throw。
function Invoke-Native {
  param(
    [Parameter(Mandatory = $true)][string]$Exe,
    # 注意：参数名不能叫 $Args —— 那是 PowerShell 的自动变量，会与形参冲突
    # （表现：参数根本传不过去，被调用方打印用法帮助）。
    [Parameter(Mandatory = $true)][string[]]$ArgList,
    [Parameter(Mandatory = $true)][string]$FailMessage
  )
  $prev = $ErrorActionPreference
  $ErrorActionPreference = 'Continue'
  try {
    & $Exe @ArgList 2>&1 | ForEach-Object { Write-Host $_ }
    $code = $LASTEXITCODE
  } finally {
    $ErrorActionPreference = $prev
  }
  if ($code -ne 0) { throw "$FailMessage（exit=$code）" }
}

Write-Host '== 1/4 构建前端 =='
Push-Location (Join-Path $Root 'web')
if (-not (Test-Path 'node_modules')) {
  # 沙箱/受限环境下系统 npm 缓存目录可能不可写，统一用仓库内缓存（已在 .gitignore 中）
  Invoke-Native 'npm' @('install', '--cache', '.npm-cache') 'npm install 失败'
}
Invoke-Native 'npm' @('run', 'build') '前端构建失败'
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
Invoke-Native 'go' @('build', '-o', (Join-Path $OutDir 'equipment.exe'), './cmd/equipment') 'exe 构建失败'
Pop-Location

Write-Host '== 3/4 组装交付目录 =='
foreach ($d in @('backup', 'logs')) {
  New-Item -ItemType Directory -Force -Path (Join-Path $OutDir $d) | Out-Null
}
Copy-Item (Join-Path $Root 'config.yaml') $OutDir -Force
Copy-Item (Join-Path $Root 'docs\user-guide.md') $OutDir -Force
# 升级脚本随交付：客户已在使用时只能「只换 exe」升级（见用户手册 §9）。
# 缺了它，运维得自己去仓库里找——这是 v1.4 Phase 6 升级演练发现的交付缺口。
Copy-Item (Join-Path $Root 'scripts\update-app.ps1') $OutDir -Force
New-Item -ItemType Directory -Force -Path (Join-Path $OutDir 'install') | Out-Null

Write-Host '== 4/4 完成 =='
Write-Host ('产物目录: ' + $OutDir)
Get-ChildItem $OutDir -Force | Select-Object Name, Length
Write-Host ''
Write-Host '提示:'
Write-Host '  1) 双击 equipment.exe 即用（首次自动建库/备份/开浏览器）。'
Write-Host '  2) Win7 需浏览器：见 用户手册 第 1 节 / install 目录说明。'
Write-Host '  3) 防火墙：本程序仅监听 127.0.0.1，通常不会触发防火墙询问。'
