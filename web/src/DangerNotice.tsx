/**
 * 危险操作后果提示（v1.4 Phase 5）。
 *
 * 统一的文案层级：**做什么**（message）→ **后果与可回退性**（description）。
 * 用于「覆盖 / 清空 / 恢复 / 报废」这类不可撤销操作，避免同一类风险在不同弹窗里各写一套说辞、
 * 有的写全、有的只提一句。
 *
 * 重要边界：本组件**只统一呈现文案层级，不改变任何确认门槛**
 * （不新增/不取消二次确认、确认词、必填原因等机制）——安全门槛的调整必须单独提出并由用户确认。
 */
import { Alert } from 'antd';

export default function DangerNotice({ message, description }: { message: string; description?: string }) {
  return <Alert type="error" showIcon message={message} description={description} style={{ marginBottom: 12 }} />;
}
