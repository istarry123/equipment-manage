# 设备资产与流转管理系统 - 程序更新脚本（只替换程序，保留数据）
#
# 用途：把新版 equipment.exe 更新到已在使用中的程序目录，
#       自动跳过 equipment.db / config.yaml / backup / logs，绝不动数据。
#
# 用法（在目标电脑上，把本脚本与新版 equipment.exe 放同一目录）：
#   powershell -ExecutionPolicy Bypass -File update-app.ps1 -TargetDir "D:\equipment"
#
# 说明：
#   - 更新前自动把目标目录的 equipment.db 备份到 backup\ 下（带时间戳）；
#   - 目标程序须先关闭（否则 exe 被占用无法替换）；
#   - 不覆盖 config.yaml（保留原端口设置）、不覆盖 equipment.db、不清理 backup。

param(
    [Parameter(Mandatory = $true)][string]$TargetDir,
    [string]$SourceDir = $PSScriptRoot,
    [switch]$SkipBackup
)

$ErrorActionPreference = 'Stop'

function Fail($msg) {
    Write-Host "[错误] $msg" -ForegroundColor Red
    exit 1
}

Write-Host '== 设备管理系统 程序更新（保留数据）=='

# 1) 校验目标目录
if (-not (Test-Path $TargetDir)) { Fail "目标目录不存在：$TargetDir" }
$TargetDir = (Resolve-Path $TargetDir).Path
if (-not (Test-Path (Join-Path $TargetDir 'equipment.db'))) {
    Fail "目标目录未见 equipment.db，可能选错了目录：$TargetDir`n（请指向实际运行程序所在目录）"
}

# 2) 校验新程序
$srcExe = Join-Path $SourceDir 'equipment.exe'
if (-not (Test-Path $srcExe)) { Fail "未找到新版 equipment.exe：$srcExe" }

# 3) 目标程序须已关闭
$running = Get-Process -Name 'equipment' -ErrorAction SilentlyContinue
if ($running) {
    Fail ("检测到程序正在运行（PID: " + (($running | ForEach-Object { $_.Id }) -join ', ') + "）。`n请先关闭程序窗口（或任务管理器结束 equipment.exe）后重试。")
}

# 4) 备份数据库与旧程序（安全快照 / 回退用）
$backupDir = Join-Path $TargetDir 'backup'
if (-not (Test-Path $backupDir)) { New-Item -ItemType Directory -Force -Path $backupDir | Out-Null }
$stamp = Get-Date -Format 'yyyy-MM-dd_HHmmss'

if (-not $SkipBackup) {
    $snapshot = Join-Path $backupDir ("pre-update-" + $stamp + ".db")
    Copy-Item (Join-Path $TargetDir 'equipment.db') $snapshot -Force
    Write-Host ("[1/3] 已备份数据库：" + $snapshot)
} else {
    Write-Host '[1/3] 已跳过数据库备份（-SkipBackup）'
}

# 旧程序始终备份：即使 -SkipBackup 也保留（回退只需把它改回 equipment.exe）。
# 只备份数据库是不够的——若新版异常，没有旧程序就无法真正回退。
$oldExe = Join-Path $TargetDir 'equipment.exe'
if (Test-Path $oldExe) {
    $exeSnapshot = Join-Path $backupDir ("equipment-before-update-" + $stamp + ".exe")
    Copy-Item $oldExe $exeSnapshot -Force
    Write-Host ("       已备份旧程序：" + $exeSnapshot + "（回退用）")
}

# 5) 替换程序（仅 exe；手册可选）
Copy-Item $srcExe (Join-Path $TargetDir 'equipment.exe') -Force
Write-Host '[2/3] 已更新 equipment.exe'

$srcGuide = Join-Path $SourceDir 'user-guide.md'
if (Test-Path $srcGuide) {
    Copy-Item $srcGuide (Join-Path $TargetDir 'user-guide.md') -Force
    Write-Host '       已更新 user-guide.md'
}

# 6) 结果核对（确认数据文件仍在）
Write-Host '[3/3] 核对目标目录：'
foreach ($f in @('equipment.exe', 'equipment.db', 'config.yaml')) {
    $p = Join-Path $TargetDir $f
    if (Test-Path $p) {
        $item = Get-Item $p
        Write-Host ("       " + $f.PadRight(18) + " " + $item.Length + " 字节  " + $item.LastWriteTime)
    } else {
        Write-Host ("       " + $f.PadRight(18) + " <不存在>")
    }
}
Write-Host ''
Write-Host '更新完成。请启动 equipment.exe，确认左上角版本号已更新且台账数据完整。' -ForegroundColor Green
Write-Host '提示：数据文件（equipment.db / backup / config.yaml / logs）均未被改动。'
Write-Host ('回退方法：把 backup\equipment-before-update-*.exe 改名为 equipment.exe 覆盖回来；' )
Write-Host ('          如需连数据一起回退，再用 backup\pre-update-*.db 走「数据备份」页的恢复。')
