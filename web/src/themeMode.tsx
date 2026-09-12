/**
 * 深浅模式状态管理（决策 20②：默认深色 + 一键切换浅色，localStorage 记忆）
 *
 * 令牌来源统一在 theme.ts；本文件只负责「当前模式」的读写与广播。
 */
import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { DEFAULT_MODE, MODE_STORAGE_KEY, applyCssVariables } from './theme';
import type { ThemeMode } from './theme';

interface ThemeModeValue {
  mode: ThemeMode;
  setMode: (mode: ThemeMode) => void;
  toggleMode: () => void;
}

const ThemeModeContext = createContext<ThemeModeValue | null>(null);

/** 读取本地记忆的模式；读取失败按默认模式处理并留下告警（不静默吞错） */
export function readStoredMode(): ThemeMode {
  try {
    const stored = window.localStorage.getItem(MODE_STORAGE_KEY);
    if (stored === 'dark' || stored === 'light') return stored;
  } catch (e) {
    console.warn('[theme] 读取主题记忆失败，回退默认模式', e);
  }
  return DEFAULT_MODE;
}

export function ThemeModeProvider({ children }: { children: ReactNode }) {
  const [mode, setModeState] = useState<ThemeMode>(() => readStoredMode());

  useEffect(() => {
    applyCssVariables(mode);
    try {
      window.localStorage.setItem(MODE_STORAGE_KEY, mode);
    } catch (e) {
      console.warn('[theme] 保存主题记忆失败（不影响当前会话）', e);
    }
  }, [mode]);

  const setMode = useCallback((next: ThemeMode) => setModeState(next), []);
  const toggleMode = useCallback(() => setModeState((m) => (m === 'dark' ? 'light' : 'dark')), []);

  const value = useMemo<ThemeModeValue>(() => ({ mode, setMode, toggleMode }), [mode, setMode, toggleMode]);

  return <ThemeModeContext.Provider value={value}>{children}</ThemeModeContext.Provider>;
}

export function useThemeMode(): ThemeModeValue {
  const ctx = useContext(ThemeModeContext);
  if (!ctx) throw new Error('useThemeMode 必须在 ThemeModeProvider 内使用');
  return ctx;
}
