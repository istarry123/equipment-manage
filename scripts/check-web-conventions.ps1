# 前端约定检查（v1.4 Phase 4 起固化）
#
# 目的：v1.4 界面改版引入了若干「唯一来源」约束，这些约束靠人工 code review 容易失守。
# 本脚本把可机器判定的部分固化为断言，随时可跑（无依赖，仅用 PowerShell）。
#
# 用法：powershell -ExecutionPolicy Bypass -File scripts\check-web-conventions.ps1
# 退出码：0 = 全部通过；1 = 有断言失败

$ErrorActionPreference = 'Stop'
$Root = Split-Path -Parent $PSScriptRoot
$Src = Join-Path $Root 'web\src'

$fail = 0
function Assert([string]$name, [bool]$ok, [string]$detail = '') {
  if ($ok) {
    Write-Host ("PASS  " + $name) -ForegroundColor Green
  } else {
    $script:fail++
    Write-Host ("FAIL  " + $name + $(if ($detail) { "  → $detail" } else { '' })) -ForegroundColor Red
  }
}

$tsx = Get-ChildItem $Src -Recurse -File -Include *.tsx,*.ts
$tsxOnly = Get-ChildItem $Src -Recurse -File -Include *.tsx

Write-Host "== 1. 色值唯一来源：除 theme.ts 外不得出现硬编码 #hex =="
$hexHits = $tsx | Select-String -Pattern '#[0-9a-fA-F]{3,8}\b' | Where-Object { $_.Filename -ne 'theme.ts' }
Assert 'web/src 无 theme.ts 之外的硬编码色值' ($null -eq $hexHits) (($hexHits | ForEach-Object { "$($_.Filename):$($_.LineNumber)" }) -join ', ')

Write-Host "`n== 2. 设备状态文案唯一来源：不得在页面里再定义状态字典 =="
# 精确匹配状态字典的键值写法（如 IN_STOCK: '在库'），避免误伤普通说明文字
$dupStatus = $tsxOnly | Select-String -Pattern "IN_STOCK:\s*'在库'"
Assert '无页面重复定义设备状态文案表' ($null -eq $dupStatus) (($dupStatus | ForEach-Object { "$($_.Filename):$($_.LineNumber)" }) -join ', ')

Write-Host "`n== 3. 分页口径：每页条数只在 pagination.ts 定义 =="
$pag = Join-Path $Src 'pagination.ts'
$pagText = Get-Content -Raw $pag
Assert 'PAGE_SIZE_LEDGER = 20' ($pagText -match 'PAGE_SIZE_LEDGER\s*=\s*20')
Assert 'PAGE_SIZE_LIST = 10'   ($pagText -match 'PAGE_SIZE_LIST\s*=\s*10')
$hardPageSize = $tsxOnly | Select-String -Pattern 'pageSize:\s*(10|20)\b'
Assert '页面内无硬编码 pageSize: 10/20' ($null -eq $hardPageSize) (($hardPageSize | ForEach-Object { "$($_.Filename):$($_.LineNumber)" }) -join ', ')
$ledger = Get-Content -Raw (Join-Path $Src 'EquipmentPage.tsx')
Assert '设备台账使用 PAGE_SIZE_LEDGER（20 条/页）' ($ledger -match 'PAGE_SIZE_LEDGER')

Write-Host "`n== 4. 编号等宽：主列表页的编号列须带 num-cell =="
$numCellFiles = @('EquipmentPage.tsx','TeamViewPage.tsx','BorrowsPage.tsx','ImportPage.tsx','ImportDetailPage.tsx')
foreach ($f in $numCellFiles) {
  $txt = Get-Content -Raw (Join-Path $Src $f)
  Assert "$f 使用 num-cell" ($txt -match "num-cell")
}

Write-Host "`n== 5. 吸顶表头：主列表页的每个 <Table 都须带 sticky =="
# 说明：<Table 多为跨行书写（属性换行），故按文件计数比较；
# 若改用「同行匹配」会把写法正确的多行 JSX 误判为失败（本脚本初版即栽在这里）。
$stickyFiles = @('EquipmentPage.tsx','TeamViewPage.tsx','BorrowsPage.tsx','TeamsPage.tsx','SettingsPage.tsx','BackupPage.tsx')
foreach ($f in $stickyFiles) {
  $txt = Get-Content -Raw (Join-Path $Src $f)
  $tableCount = ([regex]::Matches($txt, '<Table\b')).Count
  $stickyCount = ([regex]::Matches($txt, '\bsticky\b')).Count
  Assert "$f 全部 <Table> 均吸顶（Table $tableCount 个 / sticky $stickyCount 处）" ($stickyCount -ge $tableCount)
}

Write-Host "`n== 6. 后端零改动的信任边界（仅供核对，非断言） =="
Write-Host "  说明：本脚本只校验前端约定；后端是否被改动请用 git diff 核对。"

Write-Host ''
if ($fail -eq 0) {
  Write-Host "全部约定检查通过" -ForegroundColor Green
  exit 0
} else {
  Write-Host "有 $fail 项约定检查失败" -ForegroundColor Red
  exit 1
}
