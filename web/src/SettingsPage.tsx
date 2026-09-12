import { useCallback, useEffect, useState } from 'react';
import { Alert, Button, Card, Form, Input, List, message, Popconfirm, Space, Table, Typography } from 'antd';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { listPagination } from './pagination';

interface Category { id: number; name: string; sort: number }
interface Health { version: string }

async function req<T>(url: string, init?: RequestInit): Promise<T> {
  const resp = await fetch(url, {
    headers: init?.body ? { 'Content-Type': 'application/json' } : undefined,
    ...init,
  });
  const data = await resp.json().catch(() => null);
  if (!resp.ok) throw new Error(data?.error?.message ?? `HTTP ${resp.status}`);
  return data as T;
}

export default function SettingsPage() {
  const [categories, setCategories] = useState<Category[]>([]);
  const [version, setVersion] = useState('');
  const [adding, setAdding] = useState(false);
  const [form] = Form.useForm();

  const load = useCallback(async () => {
    try {
      setCategories(await req<Category[]>('/api/categories'));
      const h = await req<Health>('/api/health');
      setVersion(h.version);
    } catch {
      /* 忽略 */
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const addCategory = useCallback(async () => {
    const v = await form.validateFields();
    setAdding(true);
    try {
      await req('/api/categories', { method: 'POST', body: JSON.stringify({ name: v.name }) });
      message.success('类别已新增');
      form.resetFields();
      void load();
    } catch (e) {
      message.error(e instanceof Error ? e.message : '新增失败');
    } finally {
      setAdding(false);
    }
  }, [form, load]);

  const removeCategory = useCallback(async (c: Category) => {
    try {
      await req(`/api/categories/${c.id}`, { method: 'DELETE' });
      message.success('类别已删除');
      void load();
    } catch (e) {
      message.error(e instanceof Error ? e.message : '删除失败');
    }
  }, [load]);

  const columns: ColumnsType<Category> = [
    { title: '类别', dataIndex: 'name' },
    { title: '排序', dataIndex: 'sort', width: 100 },
    { title: 'ID', dataIndex: 'id', width: 80 },
    {
      title: '操作', key: 'op', width: 100,
      render: (_: unknown, r: Category) => (
        <Popconfirm
          title="删除该类别？"
          description="仅当没有任何设备引用该类别时才可删除；被引用时请先调整设备类别。"
          okText="删除" cancelText="取消" okButtonProps={{ danger: true }}
          onConfirm={() => void removeCategory(r)}
        >
          <Button type="link" size="small" danger icon={<DeleteOutlined />}>删除</Button>
        </Popconfirm>
      ),
    },
  ];

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Card title="系统信息" size="small">
        <List size="small" bordered dataSource={[
          `后端版本：${version}`,
          '数据存储：本地 SQLite（equipment.db）',
          '备份：backup/（启动自动 + 手动，保留最近 30 份，见「数据备份」）',
          '运行环境：Windows 7 SP1(x64)/10/11，浏览器 Chrome 109+ 或 Firefox ESR 115+（Win7 需安装随附 Chrome 109）',
        ]} renderItem={(t) => <List.Item>{t}</List.Item>} />
      </Card>

      <Card title="设备类别字典（决策 12：页面可维护）" size="small">
        <Space direction="vertical" style={{ width: '100%' }}>
          <Typography.Paragraph type="secondary" style={{ marginBottom: 4 }}>
            新增类别后即可在设备录入/筛选中选择；删除仅限未被任何设备引用的类别（被引用时需先调整设备类别）。
          </Typography.Paragraph>
          <Table rowKey="id" sticky size="small" columns={columns} dataSource={categories} pagination={listPagination()} />
          <Form form={form} layout="inline">
            <Form.Item name="name" rules={[{ required: true, message: '请输入类别名称' }]}>
              <Input placeholder="新类别名称，如：裁剪设备" style={{ width: 240 }} />
            </Form.Item>
            <Form.Item>
              <Button type="primary" icon={<PlusOutlined />} loading={adding} onClick={() => void addCategory()}>
                新增类别
              </Button>
            </Form.Item>
          </Form>
        </Space>
      </Card>

      <Alert type="info" showIcon message="边界提示"
        description="页面可修改业务数据；新增状态/流程规则/界面结构需代码升级（详见 AGENTS.md 决策基线）。" />
    </Space>
  );
}
