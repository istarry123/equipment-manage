/**
 * v1.4 界面设计令牌 —— 全站唯一色值来源
 *
 * 色值来源：本机 DSH 设计系统（@deepseek-ai/dsh-client-ui-theme）**实测取得**，非猜测。
 * 规范文档：docs/ui-design-tokens.md（含采样命令与 WCAG 对比度实测结果）。
 * 决策依据：AGENTS.md 决策 20（不引入 Next.js；默认深色 + 一键切换浅色）。
 *
 * 强制规则：
 * 1. 全站颜色只在本文件定义；页面/组件一律使用语义 CSS 变量（var(--...)）或 antd 令牌，禁止硬编码 hex。
 * 2. applyCssVariables(mode) 在启动时把令牌写入 document.documentElement；
 *    global.css 只引用 var(--...)，不写字面色值（避免双份令牌漂移）。
 */
import { theme as antdTheme } from 'antd';
import type { ThemeConfig } from 'antd';

export type ThemeMode = 'dark' | 'light';

/** 决策 20②：默认深色 + 一键切换浅色 */
export const DEFAULT_MODE: ThemeMode = 'dark';
export const MODE_STORAGE_KEY = 'equipment-theme-mode';

/** 系统字族（实测原值；Win7 自带 Microsoft YaHei / Consolas，均可用） */
export const FONT_FAMILY =
  '-apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", "Helvetica Neue", Helvetica, Arial, sans-serif';
export const FONT_FAMILY_CODE =
  '"SF Mono", "JetBrains Mono", "Fira Code", Consolas, "Liberation Mono", Menlo, Courier, "PingFang SC", "Microsoft YaHei"';

/** 语义别名：每个模式一套。字段值均取自 ui-design-tokens.md §3 实测表。 */
export interface ThemeAlias {
  bgBase: string;
  bgLayer1: string;
  bgLayer2: string;
  bgLayer3: string;
  bgSunken: string;
  bgHover: string;
  borderL1: string;
  borderL2: string;
  borderL3: string;
  textPrimary: string;
  textSecondary: string;
  textTertiary: string;
  textCaption: string;
  textDimmed: string;
  brand: string;
  link: string;
  statusInStock: string;
  statusInTeam: string;
  statusBorrowed: string;
  statusMaintenance: string;
  statusScrapped: string;
  statusOther: string;
  statusInStockText: string;
  statusInTeamText: string;
  statusBorrowedText: string;
  statusMaintenanceText: string;
  statusScrappedText: string;
  statusOtherText: string;
  dangerText: string;
  scrollbarThumb: string;
  scrollbarHover: string;
  shadowLv1: string;
  shadowLv2: string;
}

/** 深色模式：六状态色经 WCAG 实测全部达 AA（5.23–9.53） */
const DARK: ThemeAlias = {
  bgBase: '#151517',
  bgLayer1: '#232324',
  bgLayer2: '#2c2c2e',
  bgLayer3: '#353638',
  bgSunken: '#1b1b1c',
  bgHover: 'rgba(255, 255, 255, 0.08)',
  borderL1: 'rgba(255, 255, 255, 0.06)',
  borderL2: 'rgba(255, 255, 255, 0.12)',
  borderL3: 'rgba(255, 255, 255, 0.16)',
  textPrimary: '#f9fafb',
  textSecondary: '#cfd3d6',
  textTertiary: '#adb2b8',
  textCaption: '#81858c',
  textDimmed: '#43454a',
  brand: '#679efe',
  link: '#679efe',
  statusInStock: '#4ed17e',
  statusInTeam: '#679efe',
  statusBorrowed: '#f7ad31',
  statusMaintenance: '#f25a5a',
  statusScrapped: '#979da6',
  statusOther: '#8b76f6',
  statusInStockText: '#4ed17e',
  statusInTeamText: '#679efe',
  statusBorrowedText: '#f7ad31',
  statusMaintenanceText: '#f25a5a',
  statusScrappedText: '#979da6',
  statusOtherText: '#8b76f6',
  dangerText: '#f25a5a',
  scrollbarThumb: '#545557',
  scrollbarHover: '#65676b',
  shadowLv1: '0 2px 4px rgba(0, 0, 0, 0.35)',
  shadowLv2: '0 4px 12px rgba(0, 0, 0, 0.45)',
};

/** 浅色模式：文字色均经实测；已知偏差见 ui-design-tokens.md §11 */
const LIGHT: ThemeAlias = {
  bgBase: '#ffffff',
  bgLayer1: '#ffffff',
  bgLayer2: '#ffffff',
  bgLayer3: '#ffffff',
  bgSunken: '#f9fafb',
  bgHover: '#f1f3f5',
  borderL1: 'rgba(0, 0, 0, 0.04)',
  borderL2: 'rgba(0, 0, 0, 0.10)',
  borderL3: 'rgba(0, 0, 0, 0.12)',
  textPrimary: '#0f1115',
  textSecondary: '#61666b',
  textTertiary: '#81858c',
  textCaption: '#81858c',
  textDimmed: '#61666b',
  brand: '#4176e6',
  link: '#4868b2',
  statusInStock: '#22c55e',
  statusInTeam: '#4176e6',
  statusBorrowed: '#f59e0b',
  statusMaintenance: '#ef4444',
  statusScrapped: '#979da6',
  statusOther: '#722ed1',
  statusInStockText: '#233c2c',
  statusInTeamText: '#4868b2',
  statusBorrowedText: '#dd8629',
  statusMaintenanceText: '#cf1322',
  statusScrappedText: '#61666b',
  statusOtherText: '#722ed1',
  dangerText: '#cf1322',
  scrollbarThumb: '#e1e5ee',
  scrollbarHover: '#d4d4d4',
  shadowLv1: '0 2px 4px rgba(0, 0, 0, 0.05)',
  shadowLv2: '0 4px 12px rgba(0, 0, 0, 0.02)',
};

export function getAlias(mode: ThemeMode): ThemeAlias {
  return mode === 'dark' ? DARK : LIGHT;
}

/** 别名 → CSS 变量名（与 global.css 的引用一一对应） */
const CSS_VAR_NAMES: Record<keyof ThemeAlias, string> = {
  bgBase: '--bg-base',
  bgLayer1: '--bg-layer-1',
  bgLayer2: '--bg-layer-2',
  bgLayer3: '--bg-layer-3',
  bgSunken: '--bg-sunken',
  bgHover: '--bg-hover',
  borderL1: '--border-l1',
  borderL2: '--border-l2',
  borderL3: '--border-l3',
  textPrimary: '--text-primary',
  textSecondary: '--text-secondary',
  textTertiary: '--text-tertiary',
  textCaption: '--text-caption',
  textDimmed: '--text-dimmed',
  brand: '--brand',
  link: '--link',
  statusInStock: '--status-in-stock',
  statusInTeam: '--status-in-team',
  statusBorrowed: '--status-borrowed',
  statusMaintenance: '--status-maintenance',
  statusScrapped: '--status-scrapped',
  statusOther: '--status-other',
  statusInStockText: '--status-in-stock-text',
  statusInTeamText: '--status-in-team-text',
  statusBorrowedText: '--status-borrowed-text',
  statusMaintenanceText: '--status-maintenance-text',
  statusScrappedText: '--status-scrapped-text',
  statusOtherText: '--status-other-text',
  dangerText: '--danger-text',
  scrollbarThumb: '--scrollbar-thumb',
  scrollbarHover: '--scrollbar-hover',
  shadowLv1: '--shadow-lv1',
  shadowLv2: '--shadow-lv2',
};

/** 把当前模式的令牌写入根元素（启动时与切换时调用，保证首屏无闪烁） */
export function applyCssVariables(mode: ThemeMode): void {
  if (typeof document === 'undefined') return;
  const alias = getAlias(mode);
  const root = document.documentElement;
  root.setAttribute('data-theme', mode);
  root.style.colorScheme = mode;
  for (const key of Object.keys(CSS_VAR_NAMES) as (keyof ThemeAlias)[]) {
    root.style.setProperty(CSS_VAR_NAMES[key], alias[key]);
  }
  root.style.setProperty('--font-family', FONT_FAMILY);
  root.style.setProperty('--font-family-code', FONT_FAMILY_CODE);
}

/** antd 主题（映射表见 ui-design-tokens.md §8） */
export function buildAntdTheme(mode: ThemeMode): ThemeConfig {
  const a = getAlias(mode);
  return {
    algorithm: mode === 'dark' ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
    token: {
      colorPrimary: a.brand,
      colorLink: a.link,
      colorSuccess: a.statusInStock,
      colorWarning: a.statusBorrowed,
      colorError: a.statusMaintenance,
      colorInfo: a.brand,
      colorBgBase: a.bgBase,
      colorBgLayout: a.bgBase,
      colorBgContainer: a.bgLayer1,
      colorBgElevated: a.bgLayer2,
      colorText: a.textPrimary,
      colorTextSecondary: a.textSecondary,
      colorTextTertiary: a.textTertiary,
      colorTextQuaternary: a.textDimmed,
      colorBorder: a.borderL3,
      colorBorderSecondary: a.borderL2,
      borderRadius: 8,
      borderRadiusLG: 12,
      fontFamily: FONT_FAMILY,
      fontSize: 14,
      controlHeight: 32,
      wireframe: false,
    },
    components: {
      // 注意：antd 5.8.6 的 Layout 组件令牌名为 colorBgHeader / colorBgBody / colorBgTrigger
      Layout: {
        colorBgHeader: a.bgLayer1,
        colorBgBody: a.bgBase,
      },
      Menu: {
        itemBg: 'transparent',
        subMenuItemBg: 'transparent',
        itemBorderRadius: 8,
        itemHeight: 36,
        itemMarginInline: 8,
        itemSelectedBg: a.bgHover,
        itemSelectedColor: a.link,
      },
      Card: {
        headerBg: 'transparent',
        paddingLG: 16,
      },
      Descriptions: {
        labelBg: a.bgSunken,
      },
    },
    // 说明：antd 5.8.6 的 Table 组件令牌接口为空（ComponentToken {}），无法用主题定制表头，
    // 因此表头底色/文字色由 global.css 的 CSS 变量规则统一接管（见 global.css 末尾）。
  };
}
