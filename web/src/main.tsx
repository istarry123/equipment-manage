import React, { useMemo } from 'react';
import ReactDOM from 'react-dom/client';
import { ConfigProvider } from 'antd';
import { BrowserRouter } from 'react-router-dom';
import zhCN from 'antd/locale/zh_CN';
import dayjs from 'dayjs';
import 'dayjs/locale/zh-cn';
import App from './App';
import './global.css';
import { applyCssVariables, buildAntdTheme } from './theme';
import { ThemeModeProvider, readStoredMode, useThemeMode } from './themeMode';

dayjs.locale('zh-cn');

// 首屏渲染前先写入设计令牌，避免浅色→深色闪烁（AGENTS.md 决策 20②）
applyCssVariables(readStoredMode());

// 主题随模式切换：ConfigProvider 的 algorithm 与令牌同步变化
// 路由用 BrowserRouter：Go 后端 NoRoute 已对未知路径回退 index.html，直接访问/刷新均可用
function AppShell() {
  const { mode } = useThemeMode();
  const themeConfig = useMemo(() => buildAntdTheme(mode), [mode]);
  return (
    <ConfigProvider locale={zhCN} theme={themeConfig}>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </ConfigProvider>
  );
}

ReactDOM.createRoot(document.getElementById('root') as HTMLElement).render(
  <React.StrictMode>
    <ThemeModeProvider>
      <AppShell />
    </ThemeModeProvider>
  </React.StrictMode>,
);
