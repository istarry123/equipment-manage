import { useState } from 'react';
import { Alert, Button, Card, Descriptions, Space, Table, Tag, Typography, Upload } from 'antd';
import { UploadOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';

// 外借明细导入（v1.3 Phase 3）：上传 → 解析 → 与现有台账匹配 → 预览核对。
// 本阶段为只读预览：人工选择与写入在下一阶段（Phase 4）启用。

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
  chosen?: Candidate;
  candidates?: Candidate[];
  suggested_equipment_id: number;
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
  AMBIGUOUS: '待人工选择',
  MISSING: '未匹配',
};

function matchTag(status: string) {
  const color = status === 'UNIQUE' ? 'green' : status === 'BY_LABEL' ? 'blue' : status === 'AMBIGUOUS' ? 'orange' : 'red';
  return <Tag color={color}>{matchText[status] ?? status}</Tag>;
}

export default function ImportDetailPage() {
  const [parseId, setParseId] = useState('');
  const [summary, setSummary] = useState<DetailSummary | null>(null);
  const [preview, setPreview] = useState<DetailPreview | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [done, setDone] = useState(false);

  const doUpload = async (file: File) => {
    setLoading(true);
    setError('');
    setDone(false);
    setSummary(null);
    setPreview(null);
    setParseId('');
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

  const ambiguousCols: ColumnsType<MatchItem> = [
    { title: '源行', dataIndex: 'block_rows', width: 90 },
    { title: '设备编号', dataIndex: 'equipment_no', width: 100 },
    {
      title: '出现',
      width: 70,
      render: (_, r) => `${r.occurrence}/${r.occurrences}`,
    },
    { title: '外借日期', dataIndex: 'borrow_date', width: 110 },
    { title: '外借方', dataIndex: 'company', width: 170 },
    {
      title: '库中候选（占位标签 / 显示编号 / 名称 / 型号）',
      render: (_, r) => (
        <Space direction="vertical" size={2}>
          {(r.candidates ?? []).map((c) => (
            <span key={c.equipment_id}>
              <Tag color={c.equipment_id === r.suggested_equipment_id ? 'geekblue' : 'default'}>{c.label || '—'}</Tag>
              {c.display_no}（{c.name} / {c.model}）
              {c.is_current && <Tag color="red" style={{ marginLeft: 8 }}>{statusText[c.status] ?? c.status}</Tag>}
              <span style={{ color: '#999', marginLeft: 8 }}>{c.internal_code}</span>
            </span>
          ))}
        </Space>
      ),
    },
    { title: '说明', dataIndex: 'note', width: 320 },
  ];

  const missingCols: ColumnsType<MatchItem> = [
    { title: '源行', dataIndex: 'block_rows', width: 90 },
    { title: '设备编号', dataIndex: 'equipment_no', width: 110 },
    { title: '外借日期', dataIndex: 'borrow_date', width: 110 },
    { title: '外借方', dataIndex: 'company', width: 180 },
    { title: '处理建议', dataIndex: 'note' },
    { title: '旁证（不自动采用）', dataIndex: 'evidence', width: 320 },
  ];

  const itemCols: ColumnsType<MatchItem> = [
    { title: '源行', dataIndex: 'block_rows', width: 90 },
    { title: '设备编号', dataIndex: 'equipment_no', width: 100 },
    {
      title: '命中设备',
      width: 260,
      render: (_, r) => (r.chosen ? `${r.chosen.display_no}（${r.chosen.name} / ${r.chosen.model}）` : '—'),
    },
    { title: '内部码', width: 110, render: (_, r) => r.chosen?.internal_code ?? '—' },
    { title: '外借日期', dataIndex: 'borrow_date', width: 110 },
    { title: '外借方', dataIndex: 'company', width: 170 },
    { title: '匹配', width: 110, render: (_, r) => matchTag(r.status) },
  ];

  const blockCols: ColumnsType<BlockItem> = [
    { title: '源行', dataIndex: 'rows', width: 100 },
    { title: '到达时间', dataIndex: 'borrow_date', width: 110 },
    { title: '外借方', dataIndex: 'company', width: 190 },
    { title: '数量(C)', dataIndex: 'count', width: 80 },
    { title: '编号台数', dataIndex: 'numbered', width: 90 },
    { title: '无编号', dataIndex: 'unnumbered', width: 80 },
    {
      title: '结构',
      width: 90,
      render: (_, r) => (r.ok ? <Tag color="green">可导入</Tag> : <Tag color="red">阻断</Tag>),
    },
    { title: '编号原文', render: (_, r) => <span style={{ color: '#666' }}>{(r.raw_lines ?? []).join(' | ')}</span> },
  ];

  const issueCols: ColumnsType<IssueItem> = [
    { title: '行', dataIndex: 'row', width: 60 },
    { title: '编码', dataIndex: 'code', width: 80 },
    {
      title: '级别',
      dataIndex: 'level',
      width: 90,
      render: (v: string) => (
        <Tag color={v === 'BLOCK' ? 'red' : v === 'REVIEW' ? 'orange' : 'default'}>{v}</Tag>
      ),
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
  ];

  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <Card title="外借明细导入（到达时间 / 外借方 / 数量 / 设备编号）" size="small">
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Alert
            type="info"
            showIcon
            message="本页为「解析 + 匹配预览」（只读）"
            description="用于补录历史外借明细：系统按编号为主匹配现有设备台账，同号多台时给出「预分配1、预分配2…」候选，未匹配编号单独列出。人工选择与最终写入在下一阶段启用。"
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
          <Card title="解析与匹配摘要" size="small">
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
              <Descriptions.Item label="待人工选择">
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
              <Descriptions.Item label="文件内重复编号">
                {(summary.dup_nos ?? []).join('、') || '—'}
              </Descriptions.Item>
              <Descriptions.Item label="库中同号多台编号">
                {(summary.same_no_multi_no ?? []).join('、') || '—'}
              </Descriptions.Item>
            </Descriptions>
            <div style={{ marginTop: 8, color: '#888' }}>
              解析问题：阻断 {summary.block_issues} / 提示 {summary.warnings} / 需确认 {summary.reviews}
              ；解析会话 parse_id = {parseId}（写入阶段复用）
            </div>
          </Card>

          <Card title={`待人工选择（同号多台）：${preview.ambiguous.length} 条`} size="small">
            <Table
              size="small"
              rowKey="source_key"
              columns={ambiguousCols}
              dataSource={preview.ambiguous}
              pagination={false}
              scroll={{ x: 1200 }}
            />
          </Card>

          <Card title={`未匹配编号：${preview.missing.length} 条`} size="small">
            <Table
              size="small"
              rowKey="source_key"
              columns={missingCols}
              dataSource={preview.missing}
              pagination={false}
              scroll={{ x: 1200 }}
            />
          </Card>

          <Card title={`外借方落库计划：${preview.borrowers.length} 个`} size="small">
            <Table
              size="small"
              rowKey="name"
              columns={borrowerCols}
              dataSource={preview.borrowers}
              pagination={false}
            />
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
            <Table size="small" rowKey={(r) => `${r.code}-${r.row}-${r.message}`} columns={issueCols} dataSource={preview.issues} pagination={false} />
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
