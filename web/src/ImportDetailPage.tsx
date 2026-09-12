import { useState } from 'react';
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Descriptions,
  Modal,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  Upload,
  message,
} from 'antd';
import { UploadOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';

// 外借明细导入（v1.3 Phase 3 解析/匹配预览 + Phase 4 单事务补录）。
// 口径（决策 19 / 用户 2026-09-11）：以编号为主匹配；同号多台按候选顺序自动分配（可复核改选）；
// 无型号设备按外借方顺序打「外借N」标签（只进外借单/流转备注，不改台账名称型号）；
// 未匹配编号用其外借标签新建设备。

interface Candidate {
  equipment_id: number;
  internal_code: string;
  equipment_no: string;
  name: string;
  model: string;
  category: string;
  status: string;
  equipment_seq: number;
  label: string;
  display_no: string;
  is_current: boolean;
}

interface MatchItem {
  source_key: string;
  row: number;
  block_rows: string;
  equipment_no: string;
  company: string;
  borrow_date: string;
  borrow_raw: string;
  occurrence: number;
  occurrences: number;
  status: string;
  label?: string;
  chosen?: Candidate;
  candidates?: Candidate[];
  suggested_equipment_id: number;
  auto_order?: boolean;
  note?: string;
  evidence?: string;
}

interface BlockItem {
  rows: string;
  borrow_date: string;
  borrow_raw: string;
  company: string;
  count: number;
  numbered: number;
  unnumbered: number;
  ok: boolean;
  remark: string;
  raw_lines: string[];
}

interface BorrowerItem {
  name: string;
  exists: boolean;
  borrower_id: number;
  devices: number;
  blocks: number;
  labeled: number;
  label_from: string;
  label_to: string;
}

interface IssueItem {
  row: number;
  code: string;
  level: 'BLOCK' | 'WARN' | 'REVIEW';
  message: string;
}

interface DetailSummary {
  filename: string;
  sheet: string;
  blocks: number;
  blocked_blocks: number;
  total_count: number;
  numbered: number;
  skipped_unnumbered: number;
  total_row_raw: string;
  devices: number;
  unique: number;
  by_label: number;
  ambiguous: number;
  missing: number;
  borrowers: number;
  borrowers_new: number;
  block_issues: number;
  warnings: number;
  reviews: number;
  min_borrow_date: string;
  max_borrow_date: string;
  companies: string[];
  dup_nos: string[];
  same_no_multi_no: string[];
}

interface DetailPreview {
  blocks: BlockItem[];
  items: MatchItem[];
  item_total: number;
  ambiguous: MatchItem[];
  missing: MatchItem[];
  borrowers: BorrowerItem[];
  issues: IssueItem[];
}

interface ParseResp {
  parse_id: string;
  summary: DetailSummary;
  preview: DetailPreview;
}

interface ReportItem {
  source_key: string;
  label?: string;
  equipment_no: string;
  equipment_id: number;
  internal_code: string;
  name: string;
  model: string;
  company: string;
  borrow_date: string;
  action: string;
  reason?: string;
}

interface DetailReport {
  filename: string;
  batch_id: number;
  total: number;
  created: number;
  borrowed: number;
  history_only: number;
  skipped: number;
  blocked: number;
  labeled: number;
  companies: string[];
  items: ReportItem[];
  time: string;
}

const statusText: Record<string, string> = {
  IN_STOCK: '在库',
  IN_TEAM: '班组使用',
  BORROWED: '外借',
  MAINTENANCE: '维修',
  SCRAPPED: '报废',
  OTHER: '其他',
};

const matchText: Record<string, string> = {
  UNIQUE: '唯一命中',
  BY_LABEL: '按标签命中',
  AMBIGUOUS: '同号多台',
  MISSING: '未匹配',
};

const actionText: Record<string, string> = {
  BORROW_FULL: '置外借',
  CREATED_BORROW: '新建并外借',
  BORROW_HISTORY_ONLY: '仅补历史',
  SKIPPED: '已跳过',
  BLOCKED: '块阻断',
};

function matchTag(status: string) {
  const color = status === 'UNIQUE' ? 'green' : status === 'BY_LABEL' ? 'blue' : status === 'AMBIGUOUS' ? 'orange' : 'red';
  return <Tag color={color}>{matchText[status] ?? status}</Tag>;
}

function actionTag(action: string) {
  const color = action === 'BORROW_FULL' || action === 'CREATED_BORROW' ? 'green' : action === 'BORROW_HISTORY_ONLY' ? 'orange' : 'default';
  return <Tag color={color}>{actionText[action] ?? action}</Tag>;
}

export default function ImportDetailPage() {
  const [parseId, setParseId] = useState('');
  const [summary, setSummary] = useState<DetailSummary | null>(null);
  const [preview, setPreview] = useState<DetailPreview | null>(null);
  const [loading, setLoading] = useState(false);
  const [importing, setImporting] = useState(false);
  const [error, setError] = useState('');
  const [done, setDone] = useState(false);
  const [choices, setChoices] = useState<Record<string, number>>({});
  const [skip, setSkip] = useState<Record<string, boolean>>({});
  const [report, setReport] = useState<DetailReport | null>(null);

  const doUpload = async (file: File) => {
    setLoading(true);
    setError('');
    setDone(false);
    setSummary(null);
    setPreview(null);
    setParseId('');
    setReport(null);
    setChoices({});
    setSkip({});
    try {
      const form = new FormData();
      form.append('file', file);
      const resp = await fetch('/api/import/borrow-detail/parse', { method: 'POST', body: form });
      const data = (await resp.json().catch(() => null)) as ParseResp | { error?: { message?: string } } | null;
      if (!resp.ok) {
        const msg = data && 'error' in data && data.error ? data.error.message : `HTTP ${resp.status}`;
        throw new Error(msg ?? '解析失败');
      }
      const ok = data as ParseResp;
      setParseId(ok.parse_id);
      setSummary(ok.summary);
      setPreview(ok.preview);
      setDone(true);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  };

  const skippedCount = Object.values(skip).filter(Boolean).length;

  const doRun = async () => {
    setImporting(true);
    setError('');
    try {
      const resp = await fetch('/api/import/borrow-detail/run', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          parse_id: parseId,
          confirm: true,
          choices,
          skip: Object.keys(skip).filter((k) => skip[k]),
        }),
      });
      const data = (await resp.json().catch(() => null)) as { report?: DetailReport; error?: { message?: string } } | null;
      if (!resp.ok) {
        throw new Error(data?.error?.message ?? `HTTP ${resp.status}`);
      }
      setReport(data?.report ?? null);
      message.success('外借明细补录完成');
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setImporting(false);
    }
  };

  const confirmRun = () => {
    if (!summary) return;
    Modal.confirm({
      title: '确认补录外借明细？',
      width: 560,
      okText: '确认补录',
      okButtonProps: { danger: true },
      cancelText: '取消',
      content: (
        <div>
          <p>此操作会修改设备当前状态并新增历史记录（不可删除），请确认：</p>
          <ul>
            <li>将置为「外借」的设备：<b>{summary.devices - skippedCount}</b> 台（其中未匹配编号按标签新建设备 <b>{summary.missing}</b> 台）</li>
            <li>外借方：需新建 <b>{summary.borrowers_new}</b> 个</li>
            <li>你手动跳过的设备：<b>{skippedCount}</b> 台</li>
            <li>外借日期取 Excel 的到达时间；预计归还日期留空；操作人「系统导入」</li>
            <li>同一文件只能补录一次（幂等保护）</li>
          </ul>
        </div>
      ),
      onOk: doRun,
    });
  };

  const selectOptions = (r: MatchItem) =>
    (r.candidates ?? []).map((c) => ({
      value: c.equipment_id,
      label: `${c.label || '—'} ${c.display_no}（${c.name} / ${c.model}）${c.is_current ? ` · 当前${statusText[c.status] ?? c.status}` : ''}`,
    }));

  const ambiguousCols: ColumnsType<MatchItem> = [
    { title: '外借标签', dataIndex: 'label', width: 90, render: (v?: string) => v || '—' },
    { title: '源行', dataIndex: 'block_rows', width: 90 },
    { title: '设备编号', dataIndex: 'equipment_no', width: 100 },
    { title: '出现', width: 70, render: (_, r) => `${r.occurrence}/${r.occurrences}` },
    { title: '外借日期', dataIndex: 'borrow_date', width: 110 },
    { title: '外借方', dataIndex: 'company', width: 160 },
    {
      title: '写入目标（默认按候选顺序；可改选）',
      width: 380,
      render: (_, r) => (
        <Select
          size="small"
          style={{ width: 360 }}
          value={choices[r.source_key] ?? r.chosen?.equipment_id ?? undefined}
          options={selectOptions(r)}
          disabled={!!skip[r.source_key]}
          onChange={(v: number) => setChoices((s) => ({ ...s, [r.source_key]: v }))}
        />
      ),
    },
    {
      title: '跳过',
      width: 70,
      render: (_, r) => (
        <Checkbox
          checked={!!skip[r.source_key]}
          onChange={(e) => setSkip((s) => ({ ...s, [r.source_key]: e.target.checked }))}
        />
      ),
    },
  ];

  const missingCols: ColumnsType<MatchItem> = [
    { title: '外借标签', dataIndex: 'label', width: 90, render: (v?: string) => v || '—' },
    { title: '源行', dataIndex: 'block_rows', width: 90 },
    { title: '设备编号', dataIndex: 'equipment_no', width: 110 },
    { title: '外借日期', dataIndex: 'borrow_date', width: 110 },
    { title: '外借方', dataIndex: 'company', width: 160 },
    { title: '处理', dataIndex: 'note' },
    { title: '旁证（不自动采用）', dataIndex: 'evidence', width: 300 },
    {
      title: '跳过',
      width: 70,
      render: (_, r) => (
        <Checkbox
          checked={!!skip[r.source_key]}
          onChange={(e) => setSkip((s) => ({ ...s, [r.source_key]: e.target.checked }))}
        />
      ),
    },
  ];

  const itemCols: ColumnsType<MatchItem> = [
    { title: '外借标签', dataIndex: 'label', width: 90, render: (v?: string) => v || '—' },
    { title: '源行', dataIndex: 'block_rows', width: 90 },
    { title: '设备编号', dataIndex: 'equipment_no', width: 100 },
    {
      title: '命中设备',
      width: 250,
      render: (_, r) => (r.chosen ? `${r.chosen.display_no}（${r.chosen.name} / ${r.chosen.model}）` : '—'),
    },
    { title: '内部码', width: 110, render: (_, r) => r.chosen?.internal_code ?? '—' },
    { title: '外借日期', dataIndex: 'borrow_date', width: 110 },
    { title: '外借方', dataIndex: 'company', width: 160 },
    { title: '匹配', width: 110, render: (_, r) => matchTag(r.status) },
  ];

  const blockCols: ColumnsType<BlockItem> = [
    { title: '源行', dataIndex: 'rows', width: 100 },
    { title: '到达时间', dataIndex: 'borrow_date', width: 110 },
    { title: '外借方', dataIndex: 'company', width: 190 },
    { title: '数量(C)', dataIndex: 'count', width: 80 },
    { title: '编号台数', dataIndex: 'numbered', width: 90 },
    { title: '无编号', dataIndex: 'unnumbered', width: 80 },
    { title: '结构', width: 90, render: (_, r) => (r.ok ? <Tag color="green">可导入</Tag> : <Tag color="red">阻断</Tag>) },
    { title: '编号原文', render: (_, r) => <span style={{ color: '#666' }}>{(r.raw_lines ?? []).join(' | ')}</span> },
  ];

  const issueCols: ColumnsType<IssueItem> = [
    { title: '行', dataIndex: 'row', width: 60 },
    { title: '编码', dataIndex: 'code', width: 80 },
    {
      title: '级别',
      dataIndex: 'level',
      width: 90,
      render: (v: string) => <Tag color={v === 'BLOCK' ? 'red' : v === 'REVIEW' ? 'orange' : 'default'}>{v}</Tag>,
    },
    { title: '说明', dataIndex: 'message' },
  ];

  const borrowerCols: ColumnsType<BorrowerItem> = [
    { title: '外借方', dataIndex: 'name' },
    {
      title: '字典状态',
      width: 120,
      render: (_, r) => (r.exists ? <Tag color="green">已存在</Tag> : <Tag color="orange">需新建</Tag>),
    },
    { title: '块数', dataIndex: 'blocks', width: 80 },
    { title: '涉及台数', dataIndex: 'devices', width: 100 },
    {
      title: '外借标签区间',
      width: 160,
      render: (_, r) => (r.label_from ? `${r.label_from} ~ ${r.label_to}（${r.labeled} 台）` : '—'),
    },
  ];

  const reportCols: ColumnsType<ReportItem> = [
    { title: '外借标签', dataIndex: 'label', width: 90, render: (v?: string) => v || '—' },
    { title: '设备编号', dataIndex: 'equipment_no', width: 100 },
    { title: '内部码', dataIndex: 'internal_code', width: 110 },
    { title: '台账名称/型号', width: 200, render: (_, r) => `${r.name} / ${r.model}` },
    { title: '外借方', dataIndex: 'company', width: 160 },
    { title: '外借日期', dataIndex: 'borrow_date', width: 110 },
    { title: '结果', width: 110, render: (_, r) => actionTag(r.action) },
    { title: '说明', dataIndex: 'reason' },
  ];

  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <Card title="外借明细导入（到达时间 / 外借方 / 数量 / 设备编号）" size="small">
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Alert
            type="info"
            showIcon
            message="历史外借补录：解析 → 匹配 → 预览 → 确认补录"
            description="以编号为主匹配现有设备台账；同号多台按候选顺序自动分配（可在预览改选）；无型号设备按外借方顺序打「外借N」标签（只写入外借单/流转备注，不改台账名称型号）；未匹配编号用其外借标签新建设备；数量列中的「无编号」按既定口径跳过。"
          />
          <Upload
            accept=".xlsx"
            maxCount={1}
            showUploadList={false}
            beforeUpload={(file) => {
              void doUpload(file as unknown as File);
              return false;
            }}
          >
            <Button type="primary" icon={<UploadOutlined />} loading={loading}>
              选择并解析外借明细 .xlsx
            </Button>
          </Upload>
          {error && <Alert type="error" showIcon message={error} />}
        </Space>
      </Card>

      {done && summary && preview && (
        <>
          <Card
            title="解析与匹配摘要"
            size="small"
            extra={
              <Space>
                {skippedCount > 0 && <span style={{ color: '#888' }}>已勾选跳过 {skippedCount} 台</span>}
                <Button danger type="primary" loading={importing} onClick={confirmRun} disabled={!parseId}>
                  确认补录（{summary.devices - skippedCount} 台）
                </Button>
              </Space>
            }
          >
            <Descriptions bordered size="small" column={4}>
              <Descriptions.Item label="文件">{summary.filename}</Descriptions.Item>
              <Descriptions.Item label="工作表">{summary.sheet}</Descriptions.Item>
              <Descriptions.Item label="块数">{summary.blocks}</Descriptions.Item>
              <Descriptions.Item label="结构阻断块">{summary.blocked_blocks}</Descriptions.Item>
              <Descriptions.Item label="数量(C 列)合计">
                <b>{summary.total_count}</b>
              </Descriptions.Item>
              <Descriptions.Item label="编号台数">{summary.numbered}</Descriptions.Item>
              <Descriptions.Item label="无编号跳过">{summary.skipped_unnumbered}</Descriptions.Item>
              <Descriptions.Item label="表末合计原文">{summary.total_row_raw || '—'}</Descriptions.Item>
              <Descriptions.Item label="唯一命中">
                <Tag color="green">{summary.unique}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="按标签命中">
                <Tag color="blue">{summary.by_label}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="同号多台">
                <Tag color="orange">{summary.ambiguous}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="未匹配">
                <Tag color="red">{summary.missing}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="外借方（需新建）">
                {summary.borrowers}（{summary.borrowers_new}）
              </Descriptions.Item>
              <Descriptions.Item label="外借日期区间">
                {summary.min_borrow_date || '—'} ~ {summary.max_borrow_date || '—'}
              </Descriptions.Item>
              <Descriptions.Item label="文件内重复编号">{(summary.dup_nos ?? []).join('、') || '—'}</Descriptions.Item>
              <Descriptions.Item label="库中同号多台编号">
                {(summary.same_no_multi_no ?? []).join('、') || '—'}
              </Descriptions.Item>
            </Descriptions>
            <div style={{ marginTop: 8, color: '#888' }}>
              解析问题：阻断 {summary.block_issues} / 提示 {summary.warnings} / 需确认 {summary.reviews}；解析会话 parse_id = {parseId}
            </div>
          </Card>

          {report && (
            <Card title={`补录结果（批次 #${report.batch_id}，${report.time}）`} size="small">
              <Descriptions bordered size="small" column={4}>
                <Descriptions.Item label="总台数">{report.total}</Descriptions.Item>
                <Descriptions.Item label="置外借">
                  <Tag color="green">{report.borrowed}</Tag>
                </Descriptions.Item>
                <Descriptions.Item label="新建设备">
                  <Tag color="blue">{report.created}</Tag>
                </Descriptions.Item>
                <Descriptions.Item label="仅补历史">
                  <Tag color="orange">{report.history_only}</Tag>
                </Descriptions.Item>
                <Descriptions.Item label="跳过">{report.skipped}</Descriptions.Item>
                <Descriptions.Item label="块阻断">{report.blocked}</Descriptions.Item>
                <Descriptions.Item label="带外借标签">{report.labeled}</Descriptions.Item>
                <Descriptions.Item label="外借方">{(report.companies ?? []).join('、')}</Descriptions.Item>
              </Descriptions>
              <Table
                style={{ marginTop: 12 }}
                size="small"
                rowKey="source_key"
                columns={reportCols}
                dataSource={report.items}
                pagination={{ pageSize: 20, showSizeChanger: false }}
                scroll={{ x: 1200 }}
              />
            </Card>
          )}

          <Card title={`同号多台（按候选顺序自动分配）：${preview.ambiguous.length} 条`} size="small">
            <Table size="small" rowKey="source_key" columns={ambiguousCols} dataSource={preview.ambiguous} pagination={false} scroll={{ x: 1300 }} />
          </Card>

          <Card title={`未匹配编号：${preview.missing.length} 条`} size="small">
            <Table size="small" rowKey="source_key" columns={missingCols} dataSource={preview.missing} pagination={false} scroll={{ x: 1300 }} />
          </Card>

          <Card title={`外借方落库计划：${preview.borrowers.length} 个`} size="small">
            <Table size="small" rowKey="name" columns={borrowerCols} dataSource={preview.borrowers} pagination={false} />
          </Card>

          <Card title={`明细预览（前 ${preview.items.length} / ${preview.item_total} 台）`} size="small">
            <Table
              size="small"
              rowKey="source_key"
              columns={itemCols}
              dataSource={preview.items}
              pagination={{ pageSize: 20, showSizeChanger: false }}
              scroll={{ x: 1100 }}
            />
          </Card>

          <Card title={`块级预览：${preview.blocks.length} 块`} size="small">
            <Table
              size="small"
              rowKey="rows"
              columns={blockCols}
              dataSource={preview.blocks}
              pagination={{ pageSize: 10, showSizeChanger: false }}
              scroll={{ x: 1300 }}
            />
          </Card>

          <Card title={`问题清单：${preview.issues.length} 条`} size="small">
            <Table
              size="small"
              rowKey={(r) => `${r.code}-${r.row}-${r.message}`}
              columns={issueCols}
              dataSource={preview.issues}
              pagination={false}
            />
          </Card>
        </>
      )}

      {!done && (
        <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
          提示：台账初始化请用「数据导入」页；本页只处理「到达时间 / 外借方 / 数量 / 设备编号」版式的外借明细。
        </Typography.Paragraph>
      )}
    </Space>
  );
}
