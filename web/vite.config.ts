import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { fileURLToPath, URL } from 'node:url';

// 构建产物直接输出到 internal/webui/dist，由 Go go:embed 内嵌。
export default defineConfig({
  plugins: [react()],
  base: './',
  build: {
    outDir: fileURLToPath(new URL('../internal/webui/dist', import.meta.url)),
    emptyOutDir: true,
    // Win7 浏览器上限 Chrome 109（决策基线 11）
    target: 'chrome109',
    sourcemap: false,
    chunkSizeWarningLimit: 1500,
  },
  server: {
    port: 5173,
    // 开发模式下代理后端 API
    proxy: { '/api': 'http://localhost:8080' },
  },
});
