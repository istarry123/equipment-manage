// 与后端 REST API 交互的轻量客户端。

export interface Health {
  status: string;
  db: string;
  version: string;
  server_time: string;
}

export async function getHealth(): Promise<Health> {
  const resp = await fetch('/api/health', { headers: { Accept: 'application/json' } });
  if (!resp.ok) {
    throw new Error(`后端健康检查失败: HTTP ${resp.status}`);
  }
  return (await resp.json()) as Health;
}
