import { useCallback, useState } from 'react';
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Modal,
  Space,
  Table,
  Tag,
  Typography,
  Upload,
} from 'antd';
import { UploadOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';

// ---------- 与后端约定 ----------
interface Summary {
  filename: string;
  sheet: string;
  total_d: number;
  groups: number;
  ok_groups: number;
  block_groups: number;
  ok_devices: number;
  block_devs: number;
  blocks: number;
  warnings: number;
  duplicates: number;
  unnumbered: number;
}

interface IssueItem {
  row: number;
  code: string;
  level: 'BLOCK' | 'WARN';
  group?: string;
  message: string;
}

interface GroupPreview {
  name: string;
  model: string;
  category: string;
  rows: string;
  d: number;
  numbered: number;
  unnumbered: number;
  ok: boolean;
  sample_nos: string;
}

interface ParseResp {
  parse_id: string;
  summary: Summary;
  preview: { groups: GroupPreview[]; issues: IssueItem[] };
}

interface Report {
  filename: string;
  total_source_d: number;
  groups: number;
  imported: number;
  skipped: number;
  block_groups: number;
  duplicates: number;
  warnings: number;
  unnumbered: number;
  categories: string[];
  internal_code: string;
  time: string;
}

interface EqItem {
  id: number;
  equipment_no: string | null;
  display_no?: string;
  internal_code: string;
  name: string;
  model: string;
  category: string;
  status: string;
  current_since: string | null;
}

interface EqResp {
  total: number;
  items: EqItem[];
}

const statusText: Record<string, string> = {
  IN_STOCK: '在库',
  IN_TEAM: '班组使用',
  BORROWED: '外借',
  MAINTENANCE: '维修',
  SCRAPPED: '报废',
  OTHER: '其他',
};

async function jsonOrError<T>(resp: Response): Promise<T> {
  const data = await resp.json().catch(() => null);
  if (!resp.ok) {
    const msg = data && data.error ? data.error.message : `HTTP ${resp.status}`;
    throw new Error(msg);
  }
  return data as T;
}

export default function ImportPage() {
  const [parseId, setParseId] = useState<string>('');
  const [summary, setSummary] = useState<Summary | null>(null);
  const [issues, setIssues] = useState<IssueItem[]>([]);
  const [groups, setGroups] = useState<GroupPreview[]>([]);
  const [parsing, setParsing] = useState(false);
  const [running, setRunning] = useState(false);
  const [report, setReport] = useState<Report | null>(null);
  const [eq, setEq] = useState<EqResp | null>(null);
  const [loadEq, setLoadEq] = useState(false);
  const [error, setError] = useState<string>('');

  const doParse = useCallback(async (file: File) => {
    setError('');
    setReport(null);
    setEq(null);
    setParsing(true);
    try {
      const fd = new FormData();
      fd.append('file', file);
      const resp = await fetch('/api/import/parse', { method: 'POST', body: fd });
      const data = await jsonOrError<ParseResp>(resp);
      setParseId(data.parse_id);
      setSummary(data.summary);
      setIssues(data.preview.issues);
      setGroups(data.preview.groups);
    } catch (e) {
      setError(e instanceof Error ? e.message : '解析失败');
    } finally {
      setParsing(false);
    }
  }, []);

  const doImport = useCallback(async () => {
    setRunning(true);
    setError('');
    try {
      const resp = await fetch('/api/import/run', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ parse_id: parseId }),
      });
      const data = await jsonOrError<{ report: Report }>(resp);
      setReport(data.report);
      // 读取导入结果（基础台账，完整台账在 Phase 3）
      const lr = await fetch('/api/equipment?limit=200');
      const ldata = await jsonOrError<EqResp>(lr);
      setEq(ldata);
      setLoadEq(true);
    } catch (e) {
      setError(e instanceof Error ? e.message : '导入失败');
    } finally {
      setRunning(false);
    }
  }, [parseId]);

  const confirmImport = useCallback(() => {
    if (!summary) return;
    Modal.confirm({
      title: '确认导入？',
      content: `本次将从「${summary.filename}」导入设备台账。${
        summary.block_devs > 0
          ? `有 ${summary.block_groups} 个分组（约 ${summary.block_devs} 台）存在校验问题将被【跳过】并列入报告，需人工处理后重新导入。`
          : '无阻塞项。'
      }数据写入后不可在此覆盖重导。`,
      okText: '确认导入',
      cancelText: '取消',
      onOk: () => doImport(),
    });
  }, [summary, doImport]);

  const issueColumns: ColumnsType<IssueItem> = [
    {
      title: '级别', dataIndex: 'level', width: 80,
      render: (v: string) => (v === 'BLOCK' ? <Tag color="red">阻断</Tag> : <Tag color="orange">警告</Tag>),
    },
    { title: '编码', dataIndex: 'code', width: 70 },
    { title: '行', dataIndex: 'row', width: 80 },
    { title: '分组', dataIndex: 'group', width: 220, ellipsis: true },
    { title: '说明', dataIndex: 'message' },
  ];

  const groupColumns: ColumnsType<GroupPreview> = [
    { title: '名称', dataIndex: 'name', ellipsis: true },
    { title: '型号', dataIndex: 'model', ellipsis: true },
    { title: '类别', dataIndex: 'category', width: 110 },
    { title: '行', dataIndex: 'rows', width: 90 },
    { title: '台账数量', dataIndex: 'd', width: 90 },
    { title: '编号数', dataIndex: 'numbered', width: 80 },
    { title: '无编号', dataIndex: 'unnumbered', width: 80 },
    {
      title: '状态', dataIndex: 'ok', width: 80,
      render: (v: boolean) => (v ? <Tag color="green">可导入</Tag> : <Tag color="red">阻断</Tag>),
    },
    { title: '编号样例', dataIndex: 'sample_nos', ellipsis: true },
  ];

  const eqColumns: ColumnsType<EqItem> = [
    { title: '内部码', dataIndex: 'internal_code', width: 110 },
    {
      title: '显示编号', dataIndex: 'display_no', width: 150,
      render: (v: string | undefined, r: EqItem) =>
        (v || r.equipment_no) ?? <Typography.Text type="secondary">无编号</Typography.Text>,
    },
    { title: '名称', dataIndex: 'name', ellipsis: true },
    { title: '型号', dataIndex: 'model', width: 130, ellipsis: true },
    { title: '类别', dataIndex: 'category', width: 110 },
    {
      title: '状态', dataIndex: 'status', width: 100,
      render: (v: string) => <Tag color={v === 'IN_STOCK' ? 'green' : 'blue'}>{statusText[v] ?? v}</Tag>,
    },
    { title: '到达时间', dataIndex: 'current_since', width: 170, render: (v: string | null) => v ?? '-' },
  ];

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Card title="Excel 初始化导入（Tier 1：设备台账）">
        <Space direction="vertical" style={{ width: '100%' }}>
          <Alert
            type="info"
            showIcon
            message="导入流程：选择 Excel → 解析预览 → 校验检查 → 确认导入 → 报告"
            description="仅支持首次空库全量导入；存在校验问题(BLOCK)的分组将跳过并列入报告（需人工处理后重新导入）。Excel 只作为一次性数据来源。"
          />
          <Upload
            accept=".xlsx"
            showUploadList={false}
            beforeUpload={(file) => {
              void doParse(file as File);
              return false;
            }}
          >
            <Button type="primary" icon={<UploadOutlined />} loading={parsing} disabled={running}>
              选择 Excel 文件（设备借出总账.xlsx）
            </Button>
          </Upload>
          {error && <Alert type="error" showIcon message={error} />}
        </Space>
      </Card>

      {summary && (
        <>
          <Card title="解析与校验结果">
            <Descriptions column={4} size="small" bordered>
              <Descriptions.Item label="文件">{summary.filename}</Descriptions.Item>
              <Descriptions.Item label="Sheet">{summary.sheet}</Descriptions.Item>
              <Descriptions.Item label="源台账数量">{summary.total_d}</Descriptions.Item>
              <Descriptions.Item label="分组">{summary.groups}（可导入 {summary.ok_groups}）</Descriptions.Item>
              <Descriptions.Item label="可导入台数"><b>{summary.ok_devices}</b></Descriptions.Item>
              <Descriptions.Item label="将跳过(BLOCK)"><b style={{ color: '#cf1322' }}>{summary.block_devs}</b></Descriptions.Item>
              <Descriptions.Item label="重复编号">{summary.duplicates} 次</Descriptions.Item>
              <Descriptions.Item label="无编号">{summary.unnumbered} 台</Descriptions.Item>
            </Descriptions>
            {summary.blocks > 0 && (
              <Alert
                style={{ marginTop: 12 }}
                type="warning"
                showIcon
                message={`发现 ${summary.blocks} 条阻断项、${summary.warnings} 条警告`}
                description="阻断项对应分组将被跳过。请对照下表逐条人工核对（如重复编号是两台同号机，请修正 Excel 编号后重新导入）。"
              />
            )}
            <Table
              style={{ marginTop: 12 }}
              rowKey={(r) => `${r.code}-${r.row}-${r.group ?? ''}`}
              size="small"
              columns={issueColumns}
              dataSource={issues}
              pagination={{ pageSize: 10 }}
            />
            <Button
              type="primary"
              danger
              style={{ marginTop: 12 }}
              loading={running}
              disabled={!parseId}
              onClick={confirmImport}
            >
              确认导入到系统
            </Button>
          </Card>

          <Card title="分组预览（前 30 组）" size="small">
            <Table
              rowKey={(r) => `${r.name}\u0000${r.model}\u0000${r.rows}`}
              size="small"
              columns={groupColumns}
              dataSource={groups}
              pagination={{ pageSize: 10 }}
            />
          </Card>
        </>
      )}

      {report && (
        <Card title="导入报告">
          <Descriptions column={3} size="small" bordered>
            <Descriptions.Item label="文件">{report.filename}</Descriptions.Item>
            <Descriptions.Item label="导入时间">{report.time}</Descriptions.Item>
            <Descriptions.Item label="源台账数量">{report.total_source_d}</Descriptions.Item>
            <Descriptions.Item label="成功导入"><b>{report.imported}</b> 台</Descriptions.Item>
            <Descriptions.Item label="跳过（BLOCK 分组）">{report.skipped} 台 / {report.block_groups} 组</Descriptions.Item>
            <Descriptions.Item label="无编号">{report.unnumbered} 台</Descriptions.Item>
            <Descriptions.Item label="重复编号">{report.duplicates} 次（需人工处理）</Descriptions.Item>
            <Descriptions.Item label="警告">{report.warnings} 条</Descriptions.Item>
            <Descriptions.Item label="内部码区间">{report.internal_code}</Descriptions.Item>
            <Descriptions.Item label="类别" span={3}>
              {report.categories.map((c) => <Tag key={c}>{c}</Tag>)}
            </Descriptions.Item>
          </Descriptions>
          {report.skipped > 0 && (
            <Alert
              style={{ marginTop: 12 }}
              type="warning"
              showIcon
              message={`仍有 ${report.skipped} 台（${report.block_groups} 组）因校验问题未导入`}
              description="请参照上方“校验结果”处理阻断项后，在空库状态下重新执行导入。"
            />
          )}
          {loadEq && eq && (
            <Typography.Paragraph style={{ marginTop: 12 }} type="secondary">
              当前系统共 {eq.total} 台设备（前 200 条）：
            </Typography.Paragraph>
          )}
          {loadEq && eq && (
            <Table rowKey="id" size="small" columns={eqColumns} dataSource={eq.items.slice(0, 50)} pagination={false} />
          )}
        </Card>
      )}
    </Space>
  );
}
