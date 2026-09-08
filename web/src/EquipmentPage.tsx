import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Button,
  Card,
  Descriptions,
  Drawer,
  Form,
  Input,
  message,
  Modal,
  Select,
  Space,
  Table,
  Tag,
  Typography,
} from 'antd';
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';

// ---------- 类型 ----------
interface Category { id: number; name: string }
interface Team { id: number; name: string; is_active: boolean }

interface EqItem {
  id: number;
  equipment_no: string | null;
  internal_code: string;
  name: string;
  model: string;
  category: string;
  category_id: number | null;
  status: string;
  status_text?: string;
  current_team: string;
  current_team_id: number | null;
  current_borrower: string;
  current_borrower_id: number | null;
  current_since: string | null;
  remark: string;
  created_at: string;
  updated_at: string;
}

interface EqResp { total: number; items: EqItem[] }

const STATUS_OPTIONS = [
  { value: 'IN_STOCK', text: '在库', color: 'green' },
  { value: 'IN_TEAM', text: '班组使用', color: 'blue' },
  { value: 'BORROWED', text: '外借', color: 'orange' },
  { value: 'MAINTENANCE', text: '维修', color: 'red' },
  { value: 'SCRAPPED', text: '报废', color: 'default' },
  { value: 'OTHER', text: '其他', color: 'purple' },
];

const statusMeta: Record<string, { text: string; color: string }> = Object.fromEntries(
  STATUS_OPTIONS.map((s) => [s.value, { text: s.text, color: s.color }]),
);

const OPERATOR_KEY = 'eq_last_operator';
const getOperator = () => localStorage.getItem(OPERATOR_KEY) ?? '';
const setOperator = (v: string) => localStorage.setItem(OPERATOR_KEY, v);

async function req<T>(url: string, init?: RequestInit): Promise<T> {
  const resp = await fetch(url, {
    headers: init?.body ? { 'Content-Type': 'application/json' } : undefined,
    ...init,
  });
  const data = await resp.json().catch(() => null);
  if (!resp.ok) {
    const msg = data && data.error ? data.error.message : `HTTP ${resp.status}`;
    throw new Error(msg);
  }
  return data as T;
}

function locationText(e: EqItem): string {
  switch (e.status) {
    case 'IN_STOCK':
      return '仓库（在库）';
    case 'IN_TEAM':
      return e.current_team || '班组使用';
    case 'BORROWED':
      return e.current_borrower || '外借';
    case 'MAINTENANCE':
      return '维修中';
    case 'SCRAPPED':
      return '已报废';
    default:
      return e.status;
  }
}

export default function EquipmentPage() {
  const [q, setQ] = useState('');
  const [fCategory, setFCategory] = useState<number | undefined>();
  const [fStatus, setFStatus] = useState<string | undefined>();
  const [fTeam, setFTeam] = useState<number | undefined>();
  const [page, setPage] = useState(1);
  const pageSize = 20;

  const [data, setData] = useState<EqResp | null>(null);
  const [loading, setLoading] = useState(false);
  const [categories, setCategories] = useState<Category[]>([]);
  const [teams, setTeams] = useState<Team[]>([]);

  const [detail, setDetail] = useState<EqItem | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [correctOpen, setCorrectOpen] = useState(false);
  const [formCreate] = Form.useForm();
  const [formEdit] = Form.useForm();
  const [formCorrect] = Form.useForm();
  const [saving, setSaving] = useState(false);

  const loadDicts = useCallback(async () => {
    try {
      const [c, t] = await Promise.all([
        req<Category[]>('/api/categories'),
        req<Team[]>('/api/teams'),
      ]);
      setCategories(c);
      setTeams(t.filter((x) => x.is_active));
    } catch {
      /* 字典失败不影响列表 */
    }
  }, []);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      params.set('limit', String(pageSize));
      params.set('offset', String((page - 1) * pageSize));
      if (q) params.set('q', q);
      if (fCategory) params.set('category', String(fCategory));
      if (fStatus) params.set('status', fStatus);
      if (fTeam) params.set('team', String(fTeam));
      const resp = await req<EqResp>(`/api/equipment?${params.toString()}`);
      setData(resp);
    } catch (e) {
      message.error(e instanceof Error ? e.message : '加载失败');
    } finally {
      setLoading(false);
    }
  }, [q, fCategory, fStatus, fTeam, page]);

  useEffect(() => {
    void loadDicts();
  }, [loadDicts]);
  useEffect(() => {
    void load();
  }, [load]);

  const openDetail = useCallback(async (id: number) => {
    try {
      const item = await req<EqItem>(`/api/equipment/${id}`);
      setDetail(item);
    } catch (e) {
      message.error(e instanceof Error ? e.message : '加载详情失败');
    }
  }, []);

  const submitCreate = useCallback(async () => {
    const v = await formCreate.validateFields();
    setSaving(true);
    try {
      const body = {
        equipment_no: v.equipment_no || null,
        name: v.name,
        model: v.model ?? '',
        category_id: v.category_id ?? null,
        remark: v.remark ?? '',
        operator: v.operator,
      };
      await req('/api/equipment', { method: 'POST', body: JSON.stringify(body) });
      setOperator(v.operator);
      message.success('设备已新增');
      setCreateOpen(false);
      formCreate.resetFields();
      void load();
    } catch (e) {
      message.error(e instanceof Error ? e.message : '新增失败');
    } finally {
      setSaving(false);
    }
  }, [formCreate, load]);

  const submitEdit = useCallback(async () => {
    if (!detail) return;
    const v = await formEdit.validateFields();
    setSaving(true);
    try {
      await req(`/api/equipment/${detail.id}`, {
        method: 'PUT',
        body: JSON.stringify({
          name: v.name,
          model: v.model ?? '',
          category_id: v.category_id ?? null,
          remark: v.remark ?? '',
        }),
      });
      message.success('已保存');
      setEditOpen(false);
      await openDetail(detail.id);
      void load();
    } catch (e) {
      message.error(e instanceof Error ? e.message : '保存失败');
    } finally {
      setSaving(false);
    }
  }, [detail, formEdit, load, openDetail]);

  const submitCorrect = useCallback(async () => {
    if (!detail) return;
    const v = await formCorrect.validateFields();
    setSaving(true);
    try {
      await req(`/api/equipment/${detail.id}/correct`, {
        method: 'POST',
        body: JSON.stringify({
          equipment_no: v.equipment_no ?? null,
          reason: v.reason,
          operator: v.operator,
        }),
      });
      setOperator(v.operator);
      message.success('受限更正完成（已记审计）');
      setCorrectOpen(false);
      formCorrect.resetFields();
      await openDetail(detail.id);
      void load();
    } catch (e) {
      message.error(e instanceof Error ? e.message : '更正失败');
    } finally {
      setSaving(false);
    }
  }, [detail, formCorrect, load, openDetail]);

  const columns: ColumnsType<EqItem> = useMemo(
    () => [
      {
        title: '设备编号', dataIndex: 'equipment_no', width: 120,
        render: (v: string | null) => v ?? <Typography.Text type="secondary">无编号</Typography.Text>,
      },
      { title: '内部码', dataIndex: 'internal_code', width: 110 },
      { title: '名称', dataIndex: 'name', ellipsis: true },
      { title: '型号', dataIndex: 'model', width: 140, ellipsis: true, render: (v: string) => v || '-' },
      { title: '类别', dataIndex: 'category', width: 110, render: (v: string) => v || '-' },
      {
        title: '状态', dataIndex: 'status', width: 100,
        render: (v: string) => {
          const m = statusMeta[v];
          return m ? <Tag color={m.color}>{m.text}</Tag> : <Tag>{v}</Tag>;
        },
      },
      {
        title: '当前位置', key: 'location', width: 150, ellipsis: true,
        render: (_: unknown, r: EqItem) => locationText(r),
      },
      {
        title: '到达时间', dataIndex: 'current_since', width: 170,
        render: (v: string | null) => v ?? '-',
      },
      {
        title: '操作', key: 'op', width: 90,
        render: (_: unknown, r: EqItem) => (
          <Button type="link" size="small" onClick={() => void openDetail(r.id)}>
            详情
          </Button>
        ),
      },
    ],
    [openDetail],
  );

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Card
        title="设备台账"
        extra={
          <Space>
            <Button icon={<ReloadOutlined />} onClick={() => void load()} disabled={loading}>
              刷新
            </Button>
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={() => {
                formCreate.setFieldsValue({ operator: getOperator() });
                setCreateOpen(true);
              }}
            >
              新增设备
            </Button>
          </Space>
        }
      >
        <Space wrap style={{ marginBottom: 12 }}>
          <Input.Search
            allowClear
            placeholder="编号 / 名称 / 型号 / 内部码"
            style={{ width: 260 }}
            onSearch={(v) => {
              setQ(v.trim());
              setPage(1);
            }}
          />
          <Select
            allowClear placeholder="类别" style={{ width: 150 }}
            options={categories.map((c) => ({ value: c.id, label: c.name }))}
            value={fCategory}
            onChange={(v) => {
              setFCategory(v);
              setPage(1);
            }}
          />
          <Select
            allowClear placeholder="状态" style={{ width: 130 }}
            options={STATUS_OPTIONS.map((s) => ({ value: s.value, label: s.text }))}
            value={fStatus}
            onChange={(v) => {
              setFStatus(v);
              setPage(1);
            }}
          />
          <Select
            allowClear placeholder="班组/内部单位" style={{ width: 170 }}
            options={teams.map((t) => ({ value: t.id, label: t.name }))}
            value={fTeam}
            onChange={(v) => {
              setFTeam(v);
              setPage(1);
            }}
          />
        </Space>
        <Table
          rowKey="id"
          loading={loading}
          size="middle"
          columns={columns}
          dataSource={data?.items ?? []}
          onRow={(r) => ({ onDoubleClick: () => void openDetail(r.id) })}
          pagination={{
            current: page,
            pageSize,
            total: data?.total ?? 0,
            showSizeChanger: false,
            showTotal: (t) => `共 ${t} 台`,
            onChange: (p) => setPage(p),
          }}
        />
      </Card>

      {/* 新增设备 */}
      <Modal title="新增设备" open={createOpen} onCancel={() => setCreateOpen(false)} onOk={() => void submitCreate()} confirmLoading={saving}>
        <Form form={formCreate} labelCol={{ span: 6 }} wrapperCol={{ span: 17 }}>
          <Form.Item name="equipment_no" label="设备编号" extra="留空 = 无编号设备（系统生成内部码）">
            <Input placeholder="如 6061" />
          </Form.Item>
          <Form.Item name="name" label="设备名称" rules={[{ required: true, message: '必填' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="model" label="型号">
            <Input />
          </Form.Item>
          <Form.Item name="category_id" label="类别" rules={[{ required: true, message: '请选择类别' }]}>
            <Select options={categories.map((c) => ({ value: c.id, label: c.name }))} placeholder="选择类别" />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item name="operator" label="操作人" rules={[{ required: true, message: '必填' }]}>
            <Input placeholder="操作人" />
          </Form.Item>
        </Form>
      </Modal>

      {/* 详情 */}
      <Drawer
        title={detail ? `${detail.name}（${detail.equipment_no ?? '无编号'}）` : '详情'}
        open={!!detail}
        width={520}
        onClose={() => setDetail(null)}
        extra={
          detail && (
            <Space>
              <Button
                onClick={() => {
                  formEdit.setFieldsValue({
                    name: detail.name, model: detail.model,
                    category_id: detail.category_id, remark: detail.remark,
                  });
                  setEditOpen(true);
                }}
              >
                编辑基本信息
              </Button>
              <Button
                type="primary" danger
                onClick={() => {
                  formCorrect.setFieldsValue({ equipment_no: detail.equipment_no, operator: getOperator() });
                  setCorrectOpen(true);
                }}
              >
                受限更正
              </Button>
            </Space>
          )
        }
      >
        {detail && (
          <Descriptions column={1} bordered size="small">
            <Descriptions.Item label="内部码">{detail.internal_code}</Descriptions.Item>
            <Descriptions.Item label="设备编号">{detail.equipment_no ?? '无编号'}</Descriptions.Item>
            <Descriptions.Item label="名称">{detail.name}</Descriptions.Item>
            <Descriptions.Item label="型号">{detail.model || '-'}</Descriptions.Item>
            <Descriptions.Item label="类别">{detail.category || '-'}</Descriptions.Item>
            <Descriptions.Item label="状态">
              {(() => {
                const m = statusMeta[detail.status];
                return m ? <Tag color={m.color}>{m.text}</Tag> : detail.status;
              })()}
            </Descriptions.Item>
            <Descriptions.Item label="当前位置">{locationText(detail)}</Descriptions.Item>
            <Descriptions.Item label="当前班组/内部单位">{detail.current_team || '-'}</Descriptions.Item>
            <Descriptions.Item label="当前外借方">{detail.current_borrower || '-'}</Descriptions.Item>
            <Descriptions.Item label="到达当前状态时间">{detail.current_since ?? '-'}</Descriptions.Item>
            <Descriptions.Item label="备注">{detail.remark || '-'}</Descriptions.Item>
            <Descriptions.Item label="创建时间">{detail.created_at}</Descriptions.Item>
            <Descriptions.Item label="更新时间">{detail.updated_at}</Descriptions.Item>
          </Descriptions>
        )}
      </Drawer>

      {/* 编辑基本信息（不含编号） */}
      <Modal title="编辑基本信息（编号不可直接修改，需走受限更正）" open={editOpen}
        onCancel={() => setEditOpen(false)} onOk={() => void submitEdit()} confirmLoading={saving}>
        <Form form={formEdit} labelCol={{ span: 6 }} wrapperCol={{ span: 17 }}>
          <Form.Item name="name" label="设备名称" rules={[{ required: true, message: '必填' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="model" label="型号">
            <Input />
          </Form.Item>
          <Form.Item name="category_id" label="类别" rules={[{ required: true, message: '请选择类别' }]}>
            <Select options={categories.map((c) => ({ value: c.id, label: c.name }))} />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={2} />
          </Form.Item>
        </Form>
      </Modal>

      {/* 受限更正（决策 13） */}
      <Modal title="受限更正（编号/字段修正将记录审计）" open={correctOpen}
        onCancel={() => setCorrectOpen(false)} onOk={() => void submitCorrect()} confirmLoading={saving}>
        <Typography.Paragraph type="warning">
          本功能仅用于更正录入错误的设备编号等字段，须填写原因并记录审计；请勿用作日常修改。
        </Typography.Paragraph>
        <Form form={formCorrect} labelCol={{ span: 6 }} wrapperCol={{ span: 17 }}>
          <Form.Item name="equipment_no" label="设备编号" extra="留空 = 清为无编号">
            <Input />
          </Form.Item>
          <Form.Item name="reason" label="更正原因" rules={[{ required: true, message: '必填' }]}>
            <Input.TextArea rows={2} placeholder="如：导入时编号录错" />
          </Form.Item>
          <Form.Item name="operator" label="操作人" rules={[{ required: true, message: '必填' }]}>
            <Input />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  );
}
