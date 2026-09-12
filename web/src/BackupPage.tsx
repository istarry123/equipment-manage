import { useCallback, useEffect, useState } from 'react';
import { Alert, Button, Card, message, Modal, Space, Table, Tag, Typography } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { listPagination } from './pagination';

interface BackupFile { name: string; size: number; mod_time: string }

async function req<T>(url: string, init?: RequestInit): Promise<T> {
  const resp = await fetch(url, {
    headers: init?.body ? { 'Content-Type': 'application/json' } : undefined,
    ...init,
  });
  const data = await resp.json().catch(() => null);
  if (!resp.ok) throw new Error(data?.error?.message ?? `HTTP ${resp.status}`);
  return data as T;
}

export default function BackupPage() {
  const [items, setItems] = useState<BackupFile[]>([]);
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [restoreTarget, setRestoreTarget] = useState<BackupFile | null>(null);
  const [confirmText, setConfirmText] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const r = await req<{ items: BackupFile[] }>('/api/backups');
      setItems(r.items);
    } catch (e) {
      message.error(e instanceof Error ? e.message : '加载失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const manualBackup = useCallback(async () => {
    setBusy(true);
    try {
      const r = await req<{ name: string }>('/api/backup', { method: 'POST', body: '{}' });
      message.success(`备份完成：${r.name}`);
      void load();
    } catch (e) {
      message.error(e instanceof Error ? e.message : '备份失败');
    } finally {
      setBusy(false);
    }
  }, [load]);

  const doRestore = useCallback(async () => {
    if (!restoreTarget) return;
    if (confirmText !== 'RESTORE') {
      message.warning('请先阅读风险提示，并输入 RESTORE 确认');
      return;
    }
    setBusy(true);
    try {
      const r = await req<{ message: string }>('/api/restore', {
        method: 'POST',
        body: JSON.stringify({ filename: restoreTarget.name, confirm: true }),
      });
      message.success('恢复完成');
      Modal.info({ title: '恢复结果', content: r.message });
      setRestoreTarget(null);
      setConfirmText('');
      void load();
    } catch (e) {
      message.error(e instanceof Error ? e.message : '恢复失败');
    } finally {
      setBusy(false);
    }
  }, [restoreTarget, confirmText]);

  const columns: ColumnsType<BackupFile> = [
    { title: '备份文件', dataIndex: 'name' },
    { title: '大小', dataIndex: 'size', width: 120, render: (v: number) => `${(v / 1024).toFixed(1)} KB` },
    { title: '时间', dataIndex: 'mod_time', width: 170 },
    {
      title: '操作', key: 'op', width: 100,
      render: (_: unknown, r: BackupFile) =>
        r.name.startsWith('pre-restore-') ? (
          <Tag>恢复前快照</Tag>
        ) : (
          <Button type="link" size="small" danger onClick={() => { setRestoreTarget(r); setConfirmText(''); }}>
            恢复
          </Button>
        ),
    },
  ];

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Card
        title="数据备份与恢复"
        extra={
          <Space>
            <Button icon={<ReloadOutlined />} onClick={() => void load()} disabled={busy}>刷新</Button>
            <Button type="primary" loading={busy} onClick={() => void manualBackup()}>立即备份</Button>
          </Space>
        }
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <Alert
            type="info" showIcon
            message="程序每次启动自动备份；手动备份即时执行；默认保留最近 30 份。"
            description="恢复为危险操作：执行前系统会自动备份当前库（pre-restore-*）以可回退；恢复完成后当前页面数据即为备份时的状态，请刷新查看。"
          />
          <Table rowKey="name" loading={loading} size="small" columns={columns} dataSource={items} pagination={listPagination()} />
        </Space>
      </Card>

      <Modal
        title={`恢复备份：${restoreTarget?.name ?? ''}`}
        open={!!restoreTarget}
        onCancel={() => setRestoreTarget(null)}
        onOk={() => void doRestore()}
        confirmLoading={busy}
        okButtonProps={{ danger: true }}
        okText="确认恢复"
      >
        <Space direction="vertical">
          <Alert type="warning" showIcon
            message="恢复将以该备份覆盖当前数据库全部数据。"
            description="执行前系统会自动保留一份当前库快照；本操作不可撤销（除从快照再恢复）。" />
          <Typography.Text>请输入 <Tag>RESTORE</Tag> 以确认：</Typography.Text>
          <input
            value={confirmText}
            onChange={(e) => setConfirmText(e.target.value)}
            placeholder="RESTORE"
            style={{ width: '100%', padding: 6 }}
          />
        </Space>
      </Modal>
    </Space>
  );
}
