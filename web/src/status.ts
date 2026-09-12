/**
 * 设备状态显示与配色的唯一来源（对应 equipment-management §6 的六种状态）。
 *
 * 规则（docs/ui-design-tokens.md §10）：
 * 1. 状态文案与配色只在此处定义，页面不得再写内联色值。
 * 2. Tag 一律使用 antd 预设色（两模式由 algorithm 自动适配），禁止 style={{color}} 表达状态。
 * 3. 图表用实色（fillColor/textColor）——ECharts 画布无法解析 CSS 变量。
 */
import { getAlias } from './theme';
import type { ThemeAlias, ThemeMode } from './theme';

export interface StatusMeta {
  /** 中文显示名（与状态机一致，不得随意更改） */
  text: string;
  /** antd Tag 预设色 */
  tag: string;
  /** 填充色 CSS 变量（圆点/色块/边框） */
  fillVar: string;
  /** 文字色 CSS 变量 */
  textVar: string;
  /** 令牌键（供图表取实色） */
  fillKey: keyof ThemeAlias;
  textKey: keyof ThemeAlias;
}

export const STATUS_META: Record<string, StatusMeta> = {
  IN_STOCK: {
    text: '在库',
    tag: 'green',
    fillVar: 'var(--status-in-stock)',
    textVar: 'var(--status-in-stock-text)',
    fillKey: 'statusInStock',
    textKey: 'statusInStockText',
  },
  IN_TEAM: {
    text: '班组使用',
    tag: 'blue',
    fillVar: 'var(--status-in-team)',
    textVar: 'var(--status-in-team-text)',
    fillKey: 'statusInTeam',
    textKey: 'statusInTeamText',
  },
  BORROWED: {
    text: '外借',
    tag: 'orange',
    fillVar: 'var(--status-borrowed)',
    textVar: 'var(--status-borrowed-text)',
    fillKey: 'statusBorrowed',
    textKey: 'statusBorrowedText',
  },
  MAINTENANCE: {
    text: '维修',
    tag: 'red',
    fillVar: 'var(--status-maintenance)',
    textVar: 'var(--status-maintenance-text)',
    fillKey: 'statusMaintenance',
    textKey: 'statusMaintenanceText',
  },
  SCRAPPED: {
    text: '报废',
    tag: 'default',
    fillVar: 'var(--status-scrapped)',
    textVar: 'var(--status-scrapped-text)',
    fillKey: 'statusScrapped',
    textKey: 'statusScrappedText',
  },
  OTHER: {
    text: '其他',
    tag: 'purple',
    fillVar: 'var(--status-other)',
    textVar: 'var(--status-other-text)',
    fillKey: 'statusOther',
    textKey: 'statusOtherText',
  },
};

/** 状态中文名；未知状态原样返回（不猜测、不吞掉） */
export function statusText(status: string): string {
  return STATUS_META[status]?.text ?? status;
}

/** 状态 Tag 预设色；未知状态用 default */
export function statusTagColor(status: string): string {
  return STATUS_META[status]?.tag ?? 'default';
}

/** 填充色 CSS 变量；未知状态回退中性色 */
export function statusFillVar(status: string): string {
  return STATUS_META[status]?.fillVar ?? 'var(--text-tertiary)';
}

/** 文字色 CSS 变量；未知状态回退中性色 */
export function statusTextVar(status: string): string {
  return STATUS_META[status]?.textVar ?? 'var(--text-tertiary)';
}

/** 图表填充实色（ECharts 画布用） */
export function statusFillColor(mode: ThemeMode, status: string): string {
  const meta = STATUS_META[status];
  if (!meta) return getAlias(mode).textTertiary;
  return getAlias(mode)[meta.fillKey];
}

/** 图表文字实色（ECharts 画布用） */
export function statusTextColor(mode: ThemeMode, status: string): string {
  const meta = STATUS_META[status];
  if (!meta) return getAlias(mode).textTertiary;
  return getAlias(mode)[meta.textKey];
}
