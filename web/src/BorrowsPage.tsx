import { useCallback, useEffect, useState } from 'react';
import {
  Button, Card, DatePicker, Form, Input, message, Modal, Popconfirm, Select, Space, Switch, Table, Tag, Typography,
} from 'antd';
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import dayjs from 'dayjs';

interface BorrowItem {
  id: number; equipment_id: number;
  equipment_no: string; display_no?: string; name: string; model: string; category: string;
  borrower_id: number; borrower_name: string;
  borrow_date: string; expected_return_date: string; actual_return_date: string;
  status: string; overdue_days: number;
}
interface BorrowResp { total: number; items: BorrowItem[] }
interface Borrower { id: number; name: string; contact: string; phone: string; is_active: boolean }

const OPERATOR_KEY = 'eq_last_operator';
const getOperator = () => localStorage.getItem(OPERATOR_KEY) ?? '';

async function req<T>(url: string, init?: RequestInit): Promise<T> {
  const resp = await fetch(url, {
    headers: init?.body ? { 'Content-Type': 'application/json' } : undefined,
    ...init,
  });
  const data = await resp.json().catch(() => null);
  if (!resp.ok) throw new Error(data?.error?.message ?? `HTTP ${resp.status}`);
  return data as T;
}

export default function BorrowsPage() {
  const [status, setStatus] = useState<string>('');
  const [q, setQ] = useState('');
  const [page, setPage] = useState(1);
  const [data, setData] = useState<BorrowResp | null>(null);
  const [loading, setLoading] = useState(false);

  const [borrowers, setBorrowers] = useState<Borrower[]>([]);
  const [bOpen, setBOpen] = useState(false);
  const [bEdit, setBEdit] = useState<Borrower | null>(null);
  const [bSaving, setBSaving] = useState(false);
  const [formB] = Form.useForm();

  const [ret, setRet] = useState<BorrowItem | null>(null);
  const [ext, setExt] = useState<BorrowItem | null>(null);
  const [formRet] = Form.useForm();
  const [formExt] = Form.useForm();
  const [acting, setActing] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      params.set('limit', '50');
      params.set('offset', String((page - 1) * 50));
      if (status) params.set('status', status);
      if (q) params.set('q', q);
      setData(await req<BorrowResp>(`/api/borrows?${params.toString()}`));
    } catch (e) {
      message.error(e instanceof Error ? e.message : '加载失败');
    } finally {
      setLoading(false);
    }
  }, [status, q, page]);

  const loadBorrowers = useCallback(async () => {
    try {
      setBorrowers(await req<Borrower[]>('/api/borrowers'));
    } catch {
      /* 忽略 */
    }
  }, []);

  useEffect(() => {
    void loadBorrowers();
  }, [loadBorrowers]);
  useEffect(() => {
    void load();
  }, [load]);

  const doReturn = useCallback(async () => {
    if (!ret) return;
    const v = await formRet.validateFields();
    setActing(true);
    try {
      await req(`/api/borrows/${ret.id}/return`, {
        method: 'POST',
        body: JSON.stringify({
          operator: v.operator,
          remark: v.remark ?? '',
          actual_return_date: v.actual_return_date ? v.actual_return_date.format('YYYY-MM-DD') : null,
        }),
      });
      localStorage.setItem(OPERATOR_KEY, v.operator);
      message.success('归还完成（设备回仓库）');
      setRet(null);
      void load();
    } catch (e) {
      message.error(e instanceof Error ? e.message : '归还失败');
    } finally {
      setActing(false);
    }
  }, [ret, formRet, load]);

  const doExtend = useCallback(async () => {
    if (!ext) return;
    const v = await formExt.validateFields();
    setActing(true);
    try {
      await req(`/api/borrows/${ext.id}/extend`, {
        method: 'POST',
        body: JSON.stringify({ expected_return_date: v.expected_return_date.format('YYYY-MM-DD') }),
      });
      message.success('已更新预计归还日期');
      setExt(null);
      void load();
    } catch (e) {
      message.error(e instanceof Error ? e.message : '延期失败');
    } finally {
      setActing(false);
    }
  }, [ext, formExt, load]);

  const submitBorrower = useCallback(async () => {
    const v = await formB.validateFields();
    setBSaving(true);
    try {
      if (bEdit) {
        await req(`/api/borrowers/${bEdit.id}`, {
          method: 'PUT',
          body: JSON.stringify({
            name: v.name, contact: v.contact ?? '', phone: v.phone ?? '', is_active: v.is_active ?? true,
          }),
        });
      } else {
        await req('/api/borrowers', {
          method: 'POST',
          body: JSON.stringify({ name: v.name, contact: v.contact ?? '', phone: v.phone ?? '' }),
        });
      }
      message.success('已保存');
      setBOpen(false);
      void loadBorrowers();
    } catch (e) {
      message.error(e instanceof Error ? e.message : '保存失败');
    } finally {
      setBSaving(false);
    }
  }, [bEdit, formB, loadBorrowers]);

  const toggleBorrower = useCallback(async (b: Borrower) => {
    try {
      await req(`/api/borrowers/${b.id}`, {
        method: 'PUT',
        body: JSON.stringify({ name: b.name, contact: b.contact, phone: b.phone, is_active: !b.is_active }),
      });
      void loadBorrowers();
    } catch (e) {
      message.error(e instanceof Error ? e.message : '操作失败');
    }
  }, [loadBorrowers]);

  const columns: ColumnsType<BorrowItem> = [
    {
      title: '显示编号', dataIndex: 'display_no', width: 150,
      render: (v: string, r: BorrowItem) => {
        if (v) return <Typography.Text strong>{v}</Typography.Text>;
        if (r.equipment_no) return r.equipment_no;
        return <Typography.Text type="secondary">无编号</Typography.Text>;
      },
    },
    { title: '名称', dataIndex: 'name', ellipsis: true },
    { title: '型号', dataIndex: 'model', width: 130, ellipsis: true, render: (v: string) => v || '-' },
    { title: '外借方', dataIndex: 'borrower_name', width: 180, ellipsis: true },
    { title: '借出日期', dataIndex: 'borrow_date', width: 110 },
    {
      title: '预计归还', dataIndex: 'expected_return_date', width: 110,
      render: (v: string, r) =>
        r.status === 'OUTSTANDING' && r.overdue_days > 0 ? (
          <span style={{ color: '#cf1322' }}>{v || '-'}（逾期 {r.overdue_days} 天）</span>
        ) : (v || '-'),
    },
    { title: '实际归还', dataIndex: 'actual_return_date', width: 110, render: (v: string) => v || '-' },
    {
      title: '状态', dataIndex: 'status', width: 110,
      render: (v: string, r) => {
        if (v === 'OUTSTANDING') {
          return r.overdue_days > 0 ? <Tag color="red">已逾期</Tag> : <Tag color="orange">外借中</Tag>;
        }
        return <Tag color="green">已归还</Tag>;
      },
    },
    {
      title: '操作', key: 'op', width: 150,
      render: (_: unknown, r) =>
        r.status === 'OUTSTANDING' ? (
          <Space>
            <Button type="link" size="small" onClick={() => { formRet.setFieldsValue({ operator: getOperator() }); setRet(r); }}>
              归还
            </Button>
            <Button type="link" size="small"
              onClick={() => { formExt.setFieldsValue({ expected_return_date: r.expected_return_date ? dayjs(r.expected_return_date) : undefined }); setExt(r); }}>
              延期
            </Button>
          </Space>
        ) : null,
    },
  ];

  const borrowerColumns: ColumnsType<Borrower> = [
    { title: '名称', dataIndex: 'name' },
    { title: '联系人', dataIndex: 'contact', width: 140, render: (v: string) => v || '-' },
    { title: '电话', dataIndex: 'phone', width: 160, render: (v: string) => v || '-' },
    {
      title: '状态', dataIndex: 'is_active', width: 90,
      render: (v: boolean) => (v ? <Tag color="green">启用</Tag> : <Tag color="red">停用</Tag>),
    },
    {
      title: '操作', key: 'op', width: 140,
      render: (_: unknown, b) => (
        <Space>
          <Button type="link" size="small" onClick={() => { setBEdit(b); formB.setFieldsValue({ name: b.name, contact: b.contact, phone: b.phone, is_active: b.is_active }); setBOpen(true); }}>
            编辑
          </Button>
          <Popconfirm title={b.is_active ? '停用该外借方？历史记录保留。' : '启用？'} onConfirm={() => void toggleBorrower(b)}>
            <Button type="link" size="small" danger={b.is_active}>{b.is_active ? '停用' : '启用'}</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Card
        title="外借管理（归还/延期；外借操作在设备详情中发起）"
        extra={
          <Space>
            <Button icon={<ReloadOutlined />} onClick={() => void load()}>刷新</Button>
            <Button type="primary" icon={<PlusOutlined />}
              onClick={() => { setBEdit(null); formB.resetFields(); setBOpen(true); }}>
              新增外借方
            </Button>
          </Space>
        }
      >
        <Space wrap style={{ marginBottom: 12 }}>
          <Select allowClear placeholder="状态" style={{ width: 140 }}
            options={[
              { value: '', label: '全部' },
              { value: 'OUTSTANDING', label: '外借中' },
              { value: 'OVERDUE', label: '已逾期' },
              { value: 'RETURNED', label: '已归还' },
            ]}
            value={status} onChange={(v) => { setStatus(v ?? ''); setPage(1); }} />
          <Input.Search allowClear placeholder="编号/名称/外借方" style={{ width: 240 }}
            onSearch={(v) => { setQ(v.trim()); setPage(1); }} />
        </Space>
        <Table rowKey="id" loading={loading} size="middle" columns={columns}
          dataSource={data?.items ?? []}
          pagination={{
            current: page, pageSize: 50, total: data?.total ?? 0, showSizeChanger: false,
            onChange: (p) => setPage(p), showTotal: (t) => `共 ${t} 条`,
          }} />
      </Card>

      <Card title="外借方（公司）管理" size="small">
        <Typography.Paragraph type="secondary" style={{ marginBottom: 8 }}>
          内部单位（含「双发」的本厂分厂）请在「班组管理」中维护（内部调拨），本表仅维护外部借出单位。
        </Typography.Paragraph>
        <Table rowKey="id" size="small" columns={borrowerColumns} dataSource={borrowers} pagination={false} />
      </Card>

      {/* 归还 */}
      <Modal title={`归还设备：${ret?.name ?? ''}`} open={!!ret}
        onCancel={() => setRet(null)} onOk={() => void doReturn()} confirmLoading={acting}>
        <Form form={formRet} labelCol={{ span: 6 }} wrapperCol={{ span: 17 }}>
          <Form.Item label="外借方" ><span>{ret?.borrower_name}</span></Form.Item>
          <Form.Item name="actual_return_date" label="实际归还日期"
            extra="留空=今天（历史补录可指定过去日期）">
            <DatePicker style={{ width: '100%' }} disabledDate={(d) => d.isAfter(dayjs().startOf('day'))} />
          </Form.Item>
          <Form.Item name="operator" label="操作人" rules={[{ required: true, message: '必填' }]}><Input /></Form.Item>
          <Form.Item name="remark" label="备注"><Input.TextArea rows={2} /></Form.Item>
        </Form>
      </Modal>

      {/* 延期 */}
      <Modal title={`延期归还：${ext?.name ?? ''}`} open={!!ext}
        onCancel={() => setExt(null)} onOk={() => void doExtend()} confirmLoading={acting}>
        <Form form={formExt} labelCol={{ span: 8 }} wrapperCol={{ span: 14 }}>
          <Form.Item label="外借方" ><span>{ext?.borrower_name}</span></Form.Item>
          <Form.Item name="expected_return_date" label="新预计归还日期"
            rules={[{ required: true, message: '请选择日期' }]}>
            <DatePicker style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      {/* 外借方新增/编辑 */}
      <Modal title={bEdit ? '编辑外借方' : '新增外借方'} open={bOpen}
        onCancel={() => setBOpen(false)} onOk={() => void submitBorrower()} confirmLoading={bSaving}>
        <Form form={formB} labelCol={{ span: 6 }} wrapperCol={{ span: 17 }}>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '必填' }]}><Input /></Form.Item>
          <Form.Item name="contact" label="联系人"><Input /></Form.Item>
          <Form.Item name="phone" label="电话"><Input /></Form.Item>
          {bEdit && (
            <Form.Item name="is_active" label="启用" valuePropName="checked">
              <Switch />
            </Form.Item>
          )}
        </Form>
      </Modal>
    </Space>
  );
}
