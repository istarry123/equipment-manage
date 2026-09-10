// 文件下载工具：Blob 接收 + 解析 Content-Disposition（含 RFC 5987 中文名）。
export function filenameFrom(disposition: string | null): string | null {
  if (!disposition) return null;
  // 优先 filename*=UTF-8''percent-encoded
  const mStar = disposition.match(/filename\*=UTF-8''([^;]+)/i);
  if (mStar && mStar[1]) {
    try {
      return decodeURIComponent(mStar[1].trim());
    } catch {
      /* 忽略，回退 filename= */
    }
  }
  const m = disposition.match(/filename="?([^";]+)"?/i);
  if (m && m[1]) return m[1].trim();
  return null;
}

// downloadFile GET 下载：失败抛错（含后端统一 error.message），成功触发浏览器保存。
export async function downloadFile(url: string, fallbackName: string): Promise<void> {
  const resp = await fetch(url);
  if (!resp.ok) {
    const data = await resp.json().catch(() => null);
    const msg = data && data.error ? data.error.message : `HTTP ${resp.status}`;
    throw new Error(msg);
  }
  const blob = await resp.blob();
  const name = filenameFrom(resp.headers.get('Content-Disposition')) ?? fallbackName;
  const a = document.createElement('a');
  a.href = URL.createObjectURL(blob);
  a.download = name;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(a.href);
}
