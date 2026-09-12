/**
 * ECharts 统一主题（深浅两套，模块加载时注册）。
 *
 * 为什么必须做：ECharts 画布不参与 antd 的 CSS-in-JS，默认深色文字/坐标轴在深色底上不可读。
 * 用法：echarts.init(el, ECHARTS_THEME[mode])。
 */
import * as echarts from 'echarts';
import { FONT_FAMILY, getAlias } from './theme';
import type { ThemeMode } from './theme';

export const ECHARTS_THEME: Record<ThemeMode, string> = {
  dark: 'equipment-dark',
  light: 'equipment-light',
};

function axisTheme(a: ReturnType<typeof getAlias>): Record<string, unknown> {
  return {
    axisLine: { lineStyle: { color: a.borderL3 } },
    axisTick: { lineStyle: { color: a.borderL3 } },
    axisLabel: { color: a.textSecondary },
    splitLine: { lineStyle: { color: a.borderL1 } },
    splitArea: { areaStyle: { color: ['transparent', 'transparent'] } },
  };
}

function buildTheme(mode: ThemeMode): Record<string, unknown> {
  const a = getAlias(mode);
  return {
    color: [a.statusInTeam, a.statusInStock, a.statusBorrowed, a.statusMaintenance, a.statusScrapped, a.statusOther],
    backgroundColor: 'transparent',
    textStyle: { color: a.textSecondary, fontFamily: FONT_FAMILY },
    title: { textStyle: { color: a.textPrimary, fontWeight: 600 }, subtextStyle: { color: a.textTertiary } },
    legend: { textStyle: { color: a.textSecondary }, inactiveColor: a.textDimmed, itemWidth: 12, itemHeight: 8 },
    tooltip: {
      backgroundColor: a.bgLayer2,
      borderColor: a.borderL2,
      borderWidth: 1,
      textStyle: { color: a.textPrimary },
    },
    categoryAxis: axisTheme(a),
    valueAxis: axisTheme(a),
    timeAxis: axisTheme(a),
    logAxis: axisTheme(a),
    line: { symbolSize: 6, lineStyle: { width: 2 } },
    bar: { itemStyle: { borderRadius: [3, 3, 0, 0] } },
    pie: { itemStyle: { borderColor: a.bgLayer1, borderWidth: 2 } },
  };
}

echarts.registerTheme(ECHARTS_THEME.dark, buildTheme('dark'));
echarts.registerTheme(ECHARTS_THEME.light, buildTheme('light'));
