import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  Alert,
  Button,
  Card,
  DatePicker,
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
  Timeline,
  Typography,
} from 'antd';
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import dayjs from 'dayjs';
import FlowExportModal from './FlowExportModal';
import { downloadFile } from './download';
import { PAGE_SIZE_LEDGER } from './pagination';
import { STATUS_FILTER_OPTIONS, statusFillVar, statusTagColor, statusText } from './status';
import DangerNotice from './DangerNotice';

// ---------- 类型 ----------
interface Category { id: number; name: string }
interface Team { id: number; name: string; is_active: boolean }
interface Borrower { id: number; name: string; contact: string; phone: string; is_active: boolean }

interface EqItem {
  id: number;
  equipment_no: string | null;
  equipment_seq?: number;
  display_no?: string;
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
interface TxnItem {
  id: number; action: string; action_text: string;
  from_status: string; to_status: string;
  from_team_name: string; to_team_name: string;
  borrower_name: string;
  occurred_at: string; operator: string; remark: string;
}

// 状态文案与配色统一来源见 status.ts（v1.4 Phase 4 收敛；页面内不得再定义状态清单）

// 展示编号（决策18）：统一取后端 display_no；异常缺省回退 equipment_no。
const displayNoOf = (e: EqItem): string => e.display_no ?? e.equipment_no ?? '无编号';

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

// 依据状态给出可执行动作
const ACTIONS_BY_STATUS: Record<string, string[]> = {
  IN_STOCK: ['OUT_TO_TEAM', 'BORROW', 'TO_MAINTENANCE', 'SCRAP'],
  IN_TEAM: ['HANDOVER', 'BORROW', 'RETURN_FROM_TEAM', 'TO_MAINTENANCE', 'SCRAP'],
  BORROWED: ['RETURN_BORROW', 'SCRAP'],
  MAINTENANCE: ['FROM_MAINTENANCE'],
  SCRAPPED: [],
  OTHER: ['SCRAP'],
};
const ACTION_TEXT: Record<string, string> = {
  OUT_TO_TEAM: '出库给班组',
  HANDOVER: '班组转交',
  RETURN_FROM_TEAM: '班组归还入库',
  BORROW: '外借',
  RETURN_BORROW: '外借归还',
  TO_MAINTENANCE: '送修',
  FROM_MAINTENANCE: '维修完成',
  SCRAP: '报废',
};

export default function EquipmentPage({ requestOpenId }: { requestOpenId?: number | null }) {
  const [q, setQ] = useState('');
  const [fCategory, setFCategory] = useState<number | undefined>();
  const [fStatus, setFStatus] = useState<string | undefined>();
  const [fTeam, setFTeam] = useState<number | undefined>();
  const [page, setPage] = useState(1);
  const pageSize = PAGE_SIZE_LEDGER; // 设备台账总查看：每页 20 条（其他列表 10 条）

  const [data, setData] = useState<EqResp | null>(null);
  const [loading, setLoading] = useState(false);
  const [categories, setCategories] = useState<Category[]>([]);
  const [teams, setTeams] = useState<Team[]>([]);
  const [borrowers, setBorrowers] = useState<Borrower[]>([]);

  const [detail, setDetail] = useState<EqItem | null>(null);
  const [txns, setTxns] = useState<TxnItem[]>([]);
  const [createOpen, setCreateOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [correctOpen, setCorrectOpen] = useState(false);
  const [flowExportOpen, setFlowExportOpen] = useState(false);
  const [exportingLedger, setExportingLedger] = useState(false);
  const [flowAction, setFlowAction] = useState<string>('');
  const [flowSaving, setFlowSaving] = useState(false);
  const [formCreate] = Form.useForm();
  const [formEdit] = Form.useForm();
  const [formCorrect] = Form.useForm();
  const [formFlow] = Form.useForm();
  const [saving, setSaving] = useState(false);

  const loadDicts = useCallback(async () => {
    try {
      const [c, t, b] = await Promise.all([
        req<Category[]>('/api/categories'),
        req<Team[]>('/api/teams'),
        req<Borrower[]>('/api/borrowers'),
      ]);
      setCategories(c);
      setTeams(t.filter((x) => x.is_active));
      setBorrowers(b.filter((x) => x.is_active));
    } catch {
      /* 忽略 */
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
      setData(await req<EqResp>(`/api/equipment?${params.toString()}`));
    } catch (e) {
      message.error(e instanceof Error ? e.message : '加载失败');
    } finally {
      setLoading(false);
    }
  }, [q, fCategory, fStatus, fTeam, page]);

  const loadTxns = useCallback(async (id: number) => {
    try {
      const r = await req<{ total: number; items: TxnItem[] }>(`/api/equipment/${id}/transactions?limit=200`);
      setTxns(r.items);
    } catch {
      setTxns([]);
    }
  }, []);

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
      void loadTxns(id);
    } catch (e) {
      message.error(e instanceof Error ? e.message : '加载详情失败');
    }
  }, [loadTxns]);

  // 导出设备台账（沿用现有 API，带当前页面筛选条件）
  const exportLedger = useCallback(async () => {
    setExportingLedger(true);
    try {
      const params = new URLSearchParams();
      if (q) params.set('q', q);
      if (fCategory) params.set('category', String(fCategory));
      if (fStatus) params.set('status', fStatus);
      if (fTeam) params.set('team', String(fTeam));
      await downloadFile(`/api/export/equipment?${params.toString()}`, '设备台账.xlsx');
      message.success('台账导出成功');
    } catch (e) {
      message.error(e instanceof Error ? e.message : '台账导出失败');
    } finally {
      setExportingLedger(false);
    }
  }, [q, fCategory, fStatus, fTeam]);

  // 跨页请求：班组设备视图点击编号 → 复用本页详情抽屉（同 id 不重复触发）
  const lastOpenRef = useRef<number | null>(null);
  useEffect(() => {
    if (requestOpenId && requestOpenId !== lastOpenRef.current) {
      lastOpenRef.current = requestOpenId;
      void openDetail(requestOpenId);
    }
  }, [requestOpenId, openDetail]);

  const refreshDetail = useCallback(async () => {
    if (!detail) return;
    await openDetail(detail.id);
    void load();
  }, [detail, load, openDetail]);

  const submitCreate = useCallback(async () => {
    const v = await formCreate.validateFields();
    setSaving(true);
    try {
      await req('/api/equipment', {
        method: 'POST',
        body: JSON.stringify({
          equipment_no: v.equipment_no || null,
          name: v.name, model: v.model ?? '', category_id: v.category_id ?? null,
          remark: v.remark ?? '', operator: v.operator,
        }),
      });
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
        body: JSON.stringify({ name: v.name, model: v.model ?? '', category_id: v.category_id ?? null, remark: v.remark ?? '' }),
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
        body: JSON.stringify({ equipment_no: v.equipment_no ?? null, reason: v.reason, operator: v.operator }),
      });
      setOperator(v.operator);
      message.success('受限更正完成（已记审计）');
      setCorrectOpen(false);
      formCorrect.resetFields();
      await refreshDetail();
    } catch (e) {
      message.error(e instanceof Error ? e.message : '更正失败');
    } finally {
      setSaving(false);
    }
  }, [detail, formCorrect, refreshDetail]);

  const openFlow = useCallback((action: string) => {
    formFlow.resetFields();
    formFlow.setFieldsValue({ operator: getOperator() });
    setFlowAction(action);
  }, [formFlow]);

  const submitFlow = useCallback(async () => {
    if (!detail) return;
    const v = await formFlow.validateFields();
    setFlowSaving(true);
    try {
      const body: Record<string, unknown> = {
        action: flowAction,
        operator: v.operator,
        to_team_id: v.to_team_id ?? null,
        borrower_id: v.borrower_id ?? null,
        expected_return_date: v.expected_return_date ? v.expected_return_date.format('YYYY-MM-DD') : null,
        occurred_at: v.occurred_at ? v.occurred_at.format('YYYY-MM-DD') : null,
        remark: v.remark ?? '',
      };
      await req(`/api/equipment/${detail.id}/flow`, { method: 'POST', body: JSON.stringify(body) });
      setOperator(v.operator);
      message.success(`${ACTION_TEXT[flowAction]} 完成`);
      setFlowAction('');
      await refreshDetail();
    } catch (e) {
      message.error(e instanceof Error ? e.message : '操作失败');
    } finally {
      setFlowSaving(false);
    }
  }, [detail, flowAction, formFlow, refreshDetail]);

  const flowNeedsTeam = flowAction === 'OUT_TO_TEAM' || flowAction === 'HANDOVER';
  const flowNeedsBorrower = flowAction === 'BORROW';
  const flowRemarkRequired = flowAction === 'SCRAP';
  const flowRemarkLabel = flowAction === 'SCRAP' ? '报废原因' : flowAction === 'BORROW' ? '备注' : '备注';

  const locationText = (e: EqItem): string => {
    switch (e.status) {
      case 'IN_STOCK': return '仓库（在库）';
      case 'IN_TEAM': return e.current_team || '班组使用';
      case 'BORROWED': return e.current_borrower || '外借';
      case 'MAINTENANCE': return '维修中';
      case 'SCRAPPED': return '已报废';
      default: return e.status;
    }
  };

  const txnText = (t: TxnItem): string => {
    const fromLoc = t.from_team_name || t.borrower_name || t.from_status;
    const toLoc = t.to_team_name || t.borrower_name || t.to_status;
    return `${t.action_text}：${fromLoc} → ${toLoc}`;
  };

  const columns: ColumnsType<EqItem> = useMemo(
    () => [
      {
        title: '显示编号', dataIndex: 'display_no', width: 150, className: 'num-cell',
        render: (_: unknown, r: EqItem) => displayNoOf(r),
      },
      { title: '内部码', dataIndex: 'internal_code', width: 110, className: 'num-cell' },
      { title: '名称', dataIndex: 'name', ellipsis: true },
      { title: '型号', dataIndex: 'model', width: 140, ellipsis: true, render: (v: string) => v || '-' },
      { title: '类别', dataIndex: 'category', width: 110, render: (v: string) => v || '-' },
      {
        title: '状态', dataIndex: 'status', width: 100,
        render: (v: string) => <Tag color={statusTagColor(v)}>{statusText(v)}</Tag>,
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
          <Button type="link" size="small" onClick={() => void openDetail(r.id)}>详情/流转</Button>
        ),
      },
    ],
    [openDetail],
  );

  const hasFilter = Boolean(q || fCategory || fStatus || fTeam);
  const actions = detail ? ACTIONS_BY_STATUS[detail.status] ?? [] : [];

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Card
        title="设备台账 / 流转"
        extra={
          <Space wrap>
            <Button icon={<ReloadOutlined />} onClick={() => void load()} disabled={loading}>刷新</Button>
            <Button loading={exportingLedger} disabled={exportingLedger} onClick={() => void exportLedger()}>
              导出设备台账
            </Button>
            <Button type="primary" onClick={() => setFlowExportOpen(true)}>
              导出流转情况
            </Button>
            <Button type="primary" icon={<PlusOutlined />}
              onClick={() => { formCreate.setFieldsValue({ operator: getOperator() }); setCreateOpen(true); }}>
              新增设备
            </Button>
          </Space>
        }
      >
        <Space wrap style={{ marginBottom: 12 }}>
          <Input.Search allowClear placeholder="编号 / 名称 / 型号 / 内部码" style={{ width: 260 }}
            onSearch={(v) => { setQ(v.trim()); setPage(1); }} />
          <Select allowClear placeholder="类别" style={{ width: 150 }}
            options={categories.map((c) => ({ value: c.id, label: c.name }))} value={fCategory}
            onChange={(v) => { setFCategory(v); setPage(1); }} />
          <Select allowClear placeholder="状态" style={{ width: 130 }}
            options={STATUS_FILTER_OPTIONS} value={fStatus}
            onChange={(v) => { setFStatus(v); setPage(1); }} />
          <Select allowClear placeholder="班组/内部单位" style={{ width: 170 }}
            options={teams.map((t) => ({ value: t.id, label: t.name }))} value={fTeam}
            onChange={(v) => { setFTeam(v); setPage(1); }} />
        </Space>
        <Table rowKey="id" sticky loading={loading} size="middle" columns={columns} dataSource={data?.items ?? []}
          locale={{ emptyText: hasFilter ? '无匹配结果，请调整或清空筛选条件' : '暂无设备数据' }}
          onRow={(r) => ({ onDoubleClick: () => void openDetail(r.id) })}
          pagination={{
            current: page, pageSize, total: data?.total ?? 0, showSizeChanger: false,
            showTotal: (t) => `共 ${t} 台`, onChange: (p) => setPage(p),
          }} />
      </Card>

      {/* 新增设备 */}
      <Modal title="新增设备" open={createOpen} onCancel={() => setCreateOpen(false)} onOk={() => void submitCreate()} confirmLoading={saving}>
        <Form form={formCreate} labelCol={{ span: 6 }} wrapperCol={{ span: 17 }}>
          <Form.Item name="equipment_no" label="设备编号" extra="留空 = 无编号设备">
            <Input placeholder="如 6061" />
          </Form.Item>
          <Form.Item name="name" label="设备名称" rules={[{ required: true, message: '必填' }]}><Input /></Form.Item>
          <Form.Item name="model" label="型号"><Input /></Form.Item>
          <Form.Item name="category_id" label="类别" rules={[{ required: true, message: '请选择类别' }]}>
            <Select options={categories.map((c) => ({ value: c.id, label: c.name }))} placeholder="选择类别" />
          </Form.Item>
          <Form.Item name="remark" label="备注"><Input.TextArea rows={2} /></Form.Item>
          <Form.Item name="operator" label="操作人" rules={[{ required: true, message: '必填' }]}><Input /></Form.Item>
        </Form>
      </Modal>

      {/* 详情抽屉 */}
      <Drawer
        title={detail ? `${detail.name}（${displayNoOf(detail)}）` : '详情'}
        open={!!detail}
        width={560}
        onClose={() => { setDetail(null); setFlowAction(''); }}
        extra={
          detail && (
            <Space>
              <Button onClick={() => {
                formEdit.setFieldsValue({
                  name: detail.name, model: detail.model,
                  category_id: detail.category_id, remark: detail.remark,
                });
                setEditOpen(true);
              }}>编辑</Button>
              <Button type="primary" danger onClick={() => {
                formCorrect.setFieldsValue({ equipment_no: detail.equipment_no, operator: getOperator() });
                setCorrectOpen(true);
              }}>受限更正</Button>
            </Space>
          )
        }
      >
        {detail && (
          <>
            <Descriptions column={1} bordered size="small">
              <Descriptions.Item label="内部码">{detail.internal_code}</Descriptions.Item>
              <Descriptions.Item label="显示编号">{displayNoOf(detail)}</Descriptions.Item>
              {detail.equipment_no && <Descriptions.Item label="原始编号">{detail.equipment_no}</Descriptions.Item>}
              <Descriptions.Item label="名称">{detail.name}</Descriptions.Item>
              <Descriptions.Item label="型号">{detail.model || '-'}</Descriptions.Item>
              <Descriptions.Item label="类别">{detail.category || '-'}</Descriptions.Item>
              <Descriptions.Item label="状态">
                <Tag color={statusTagColor(detail.status)}>{statusText(detail.status)}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="当前位置">{locationText(detail)}</Descriptions.Item>
              <Descriptions.Item label="到达当前状态时间">{detail.current_since ?? '-'}</Descriptions.Item>
              <Descriptions.Item label="备注">{detail.remark || '-'}</Descriptions.Item>
            </Descriptions>

            <Card size="small" title="流转操作" style={{ marginTop: 12 }}>
              {actions.length === 0 ? (
                <Typography.Text type="secondary">当前状态无可执行操作（已报废等）</Typography.Text>
              ) : (
                <Space wrap>
                  {actions.map((a) => (
                    <Button key={a} type={a === 'SCRAP' ? 'default' : 'primary'}
                      danger={a === 'SCRAP' || a === 'RETURN_BORROW'}
                      onClick={() => openFlow(a)}>
                      {ACTION_TEXT[a]}
                    </Button>
                  ))}
                </Space>
              )}
            </Card>

            <Card size="small" title={`流转历史（${txns.length} 条，最近在前）`} style={{ marginTop: 12 }}>
              {txns.length === 0 ? (
                <Typography.Text type="secondary">暂无历史</Typography.Text>
              ) : (
                <Timeline
                  items={txns.map((t) => ({
                    // 圆点取「目标状态」的语义色：一眼看出这次流转把设备带到了哪个状态；
                    // 报废单独用危险色强调（它是终态）。
                    color: t.action === 'SCRAP' ? 'var(--danger-text)' : statusFillVar(t.to_status),
                    children: (
                      <>
                        <div>
                          <b>{txnText(t)}</b>
                          <Typography.Text type="secondary" style={{ float: 'right' }}>{t.occurred_at}</Typography.Text>
                        </div>
                        <div><Typography.Text type="secondary">操作人：{t.operator}</Typography.Text></div>
                        {t.remark && <div><Typography.Text type="secondary">备注：{t.remark}</Typography.Text></div>}
                      </>
                    ),
                  }))}
                />
              )}
            </Card>
          </>
        )}
      </Drawer>

      {/* 编辑基本信息 */}
      <Modal title="编辑基本信息（编号需走受限更正）" open={editOpen}
        onCancel={() => setEditOpen(false)} onOk={() => void submitEdit()} confirmLoading={saving}>
        <Form form={formEdit} labelCol={{ span: 6 }} wrapperCol={{ span: 17 }}>
          <Form.Item name="name" label="设备名称" rules={[{ required: true, message: '必填' }]}><Input /></Form.Item>
          <Form.Item name="model" label="型号"><Input /></Form.Item>
          <Form.Item name="category_id" label="类别" rules={[{ required: true, message: '请选择类别' }]}>
            <Select options={categories.map((c) => ({ value: c.id, label: c.name }))} />
          </Form.Item>
          <Form.Item name="remark" label="备注"><Input.TextArea rows={2} /></Form.Item>
        </Form>
      </Modal>

      {/* 受限更正 */}
      <Modal title="受限更正（记录审计）" open={correctOpen}
        onCancel={() => setCorrectOpen(false)} onOk={() => void submitCorrect()} confirmLoading={saving}>
        <Typography.Paragraph type="warning">仅用于更正录入错误，须填原因并记录审计。</Typography.Paragraph>
        <Form form={formCorrect} labelCol={{ span: 6 }} wrapperCol={{ span: 17 }}>
          <Form.Item name="equipment_no" label="设备编号" extra="留空 = 清为无编号"><Input /></Form.Item>
          <Form.Item name="reason" label="更正原因" rules={[{ required: true, message: '必填' }]}>
            <Input.TextArea rows={2} placeholder="如：导入时编号录错" />
          </Form.Item>
          <Form.Item name="operator" label="操作人" rules={[{ required: true, message: '必填' }]}><Input /></Form.Item>
        </Form>
      </Modal>

      {/* 流转操作 */}
      <Modal title={ACTION_TEXT[flowAction] || '流转操作'} open={!!flowAction}
        onCancel={() => setFlowAction('')} onOk={() => void submitFlow()} confirmLoading={flowSaving}
        okButtonProps={{ danger: flowAction === 'SCRAP' }}>
        <Alert style={{ marginBottom: 12 }} type="info" showIcon
          message="每次操作将同步更新当前状态并写入流转历史（可追溯）。" />
        {flowAction === 'SCRAP' && (
          <DangerNotice
            message="报废为终态：设备报废后不可再流转。"
            description="本操作不可撤销（如需继续使用，只能重新录入设备）；报废原因必填并记入流转历史。"
          />
        )}
        <Form form={formFlow} labelCol={{ span: 6 }} wrapperCol={{ span: 17 }}>
          {flowNeedsTeam && (
            <Form.Item name="to_team_id" label={flowAction === 'OUT_TO_TEAM' ? '目标班组' : '新班组'}
              rules={[{ required: true, message: '请选择班组' }]}>
              <Select options={teams
                .filter((t) => flowAction !== 'HANDOVER' || t.id !== detail?.current_team_id)
                .map((t) => ({ value: t.id, label: t.name }))}
                placeholder="班组/内部单位" />
            </Form.Item>
          )}
          {flowNeedsBorrower && (
            <>
              <Form.Item name="borrower_id" label="外借方" rules={[{ required: true, message: '请选择外借方' }]}>
                <Select options={borrowers.map((b) => ({ value: b.id, label: b.name }))} placeholder="外部公司/单位" />
              </Form.Item>
              <Form.Item name="expected_return_date" label="预计归还日期">
                <DatePicker style={{ width: '100%' }} disabledDate={(d) => d.isBefore(dayjs().startOf('day'))} />
              </Form.Item>
            </>
          )}
          <Form.Item name="remark" label={flowRemarkLabel}
            rules={flowRemarkRequired ? [{ required: true, message: '请填写报废原因' }] : []}>
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item name="occurred_at" label="发生日期"
            extra="留空=今天（历史补录可指定过去日期）">
            <DatePicker style={{ width: '100%' }} disabledDate={(d) => d.isAfter(dayjs().startOf('day'))} />
          </Form.Item>
          <Form.Item name="operator" label="操作人" rules={[{ required: true, message: '必填' }]}><Input /></Form.Item>
        </Form>
      </Modal>

      {/* 导出流转情况 */}
      <FlowExportModal
        open={flowExportOpen}
        onClose={() => setFlowExportOpen(false)}
        teams={teams}
        borrowers={borrowers}
      />
    </Space>
  );
}
