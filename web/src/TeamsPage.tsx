import { useCallback, useEffect, useState } from 'react';
import { Button, Card, Form, Input, message, Modal, Space, Switch, Table, Tag } from 'antd';
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';

interface Team { id: number; name: string; department: string; is_active: boolean }

async function req<T>(url: string, init?: RequestInit): Promise<T> {
  const resp = await fetch(url, {
    headers: init?.body ? { 'Content-Type': 'application/json' } : undefined,
    ...init,
  });
  const data = await resp.json().catch(() => null);
  if (!resp.ok) throw new Error(data?.error?.message ?? `HTTP ${resp.status}`);
  return data as T;
}

export default function TeamsPage() {
  const [rows, setRows] = useState<Team[]>([]);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<Team | null>(null);
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setRows(await req<Team[]>('/api/teams'));
    } catch (e) {
      message.error(e instanceof Error ? e.message : '加载失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const openAdd = () => {
    setEditing(null);
    form.resetFields();
    setOpen(true);
  };
  const openEdit = (t: Team) => {
    setEditing(t);
    form.setFieldsValue({ name: t.name, department: t.department, is_active: t.is_active });
    setOpen(true);
  };

  const submit = useCallback(async () => {
    const v = await form.validateFields();
    setSaving(true);
    try {
      if (editing) {
        await req(`/api/teams/${editing.id}`, {
          method: 'PUT',
          body: JSON.stringify({ name: v.name, department: v.department ?? '', is_active: v.is_active ?? true }),
        });
      } else {
        await req('/api/teams', { method: 'POST', body: JSON.stringify({ name: v.name, department: v.department ?? '' }) });
      }
      message.success('已保存');
      setOpen(false);
      void load();
    } catch (e) {
      message.error(e instanceof Error ? e.message : '保存失败');
    } finally {
      setSaving(false);
    }
  }, [editing, form, load]);

  const toggleActive = useCallback(async (t: Team) => {
    try {
      await req(`/api/teams/${t.id}`, {
        method: 'PUT',
        body: JSON.stringify({ name: t.name, department: t.department, is_active: !t.is_active }),
      });
      void load();
    } catch (e) {
      message.error(e instanceof Error ? e.message : '操作失败');
    }
  }, [load]);

  const columns: ColumnsType<Team> = [
    { title: '名称', dataIndex: 'name' },
    { title: '部门/说明', dataIndex: 'department', render: (v: string) => v || '-' },
    {
      title: '状态', dataIndex: 'is_active', width: 100,
      render: (v: boolean) => (v ? <Tag color="green">启用</Tag> : <Tag color="red">停用</Tag>),
    },
    {
      title: '操作', key: 'op', width: 180,
      render: (_: unknown, r: Team) => (
        <Space>
          <Button type="link" size="small" onClick={() => openEdit(r)}>编辑</Button>
          <Button type="link" size="small" danger={r.is_active} onClick={() => void toggleActive(r)}>
            {r.is_active ? '停用' : '启用'}
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <Card
      title="班组 / 内部单位管理"
      extra={
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => void load()}>刷新</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={openAdd}>新增班组</Button>
        </Space>
      }
    >
      <Space direction="vertical" style={{ width: '100%' }}>
        <span>
          说明：名称含「双发」的本厂内部单位（内部调拨，决策 15）与外借公司分开管理；历史记录保存流转当时的名称（快照），改名/停用不影响追溯。
        </span>
        <Table rowKey="id" loading={loading} columns={columns} dataSource={rows} pagination={false} />
      </Space>

      <Modal title={editing ? '编辑班组' : '新增班组'} open={open}
        onCancel={() => setOpen(false)} onOk={() => void submit()} confirmLoading={saving}>
        <Form form={form} labelCol={{ span: 6 }} wrapperCol={{ span: 17 }}>
          <Form.Item name="name" label="班组名称" rules={[{ required: true, message: '必填' }]}><Input /></Form.Item>
          <Form.Item name="department" label="部门/说明"><Input /></Form.Item>
          {editing && (
            <Form.Item name="is_active" label="启用" valuePropName="checked">
              <Switch />
            </Form.Item>
          )}
        </Form>
      </Modal>
    </Card>
  );
}
