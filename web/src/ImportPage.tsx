import { useCallback, useState } from 'react';
import {
  Alert,
  Button,
  Card,
  Checkbox,
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
  reviews: number;
  duplicates: number;
  unnumbered: number;
  borrow_events: number;
  internal_events: number;
  suspected: number;
  review_items: number;
}

interface IssueItem {
  row: number;
  code: string;
  level: 'BLOCK' | 'WARN' | 'REVIEW';
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

interface PreviewDevice {
  source_key: string;
  display_no: string;
  equipment_no: string;
  name: string;
  model: string;
  category: string;
  unnumbered: boolean;
  ok: boolean;
  group_rows: string;
}

interface SuspectItem {
  source_key: string;
  display_no: string;
  name: string;
  model: string;
  company: string;
  borrow_date: string;
  row: number;
  remark: string;
}

interface ReviewItem {
  key: string;
  level: string;
  group: string;
  row: number;
  message: string;
  raw: string;
  suggestion: string;
}

interface ParseResp {
  parse_id: string;
  summary: Summary;
  preview: {
    groups: GroupPreview[];
    issues: IssueItem[];
    devices: PreviewDevice[];
    device_total: number;
    suspected: SuspectItem[];
    reviews: ReviewItem[];
  };
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
  borrowed_now: number;
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
  const [devices, setDevices] = useState<PreviewDevice[]>([]);
  const [suspects, setSuspects] = useState<SuspectItem[]>([]);
  const [reviews, setReviews] = useState<ReviewItem[]>([]);
  const [suspectPicked, setSuspectPicked] = useState<Record<string, boolean>>({});
  const [reviewAck, setReviewAck] = useState<Record<string, boolean>>({});
  const [parsing, setParsing] = useState(false);
  const [running, setRunning] = useState(false);
  const [report, setReport] = useState<Report | null>(null);
  const [eq, setEq] = useState<EqResp | null>(null);
  const [loadEq, setLoadEq] = useState(false);
  const [error, setError] = useState<string>('');

  const resetReviewState = useCallback(() => {
    setSuspectPicked({});
    setReviewAck({});
  }, []);

  const doParse = useCallback(async (file: File) => {
    setError('');
    setReport(null);
    setEq(null);
    resetReviewState();
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
      setDevices(data.preview.devices);
      setSuspects(data.preview.suspected);
      setReviews(data.preview.reviews);
    } catch (e) {
      setError(e instanceof Error ? e.message : '解析失败');
    } finally {
      setParsing(false);
    }
  }, [resetReviewState]);

  // REVIEW 未全部确认前禁止导入（§三十三）
  const reviewPending = reviews.filter((r) => !reviewAck[r.key]).length;
  const importDisabled = !parseId || reviewPending > 0;

  const doImport = useCallback(async () => {
    setRunning(true);
    setError('');
    try {
      const confirmed = suspects.filter((s) => suspectPicked[s.source_key]).map((s) => s.source_key);
      const ack = reviews.map((r) => r.key);
      const resp = await fetch('/api/import/run', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ parse_id: parseId, confirmed_suspects: confirmed, acknowledged_reviews: ack }),
      });
      const data = await jsonOrError<{ report: Report }>(resp);
      setReport(data.report);
      const lr = await fetch('/api/equipment?limit=200');
      const ldata = await jsonOrError<EqResp>(lr);
      setEq(ldata);
      setLoadEq(true);
    } catch (e) {
      setError(e instanceof Error ? e.message : '导入失败');
    } finally {
      setRunning(false);
    }
  }, [parseId, suspects, suspectPicked, reviews]);

  const confirmImport = useCallback(() => {
    if (!summary) return;
    const picked = suspects.filter((s) => suspectPicked[s.source_key]).length;
    Modal.confirm({
      title: '确认导入？',
      content: (
        <span>
          本次将从「{summary.filename}」导入设备台账。
          设备默认全部以【在库】导入；另确认 <b>{picked}</b> 台「疑似在借」为当前仍外借（将置为外借并生成外借单）。
          {summary.block_devs > 0
            ? `有 ${summary.block_groups} 个分组（约 ${summary.block_devs} 台）存在校验问题将被【跳过】并列入报告。`
            : ''}
          写入后不可在此覆盖重导。
        </span>
      ),
      okText: '确认导入',
      cancelText: '取消',
      onOk: () => doImport(),
    });
  }, [summary, suspects, suspectPicked, doImport]);

  const issueColumns: ColumnsType<IssueItem> = [
    {
      title: '级别', dataIndex: 'level', width: 90,
      render: (v: string) => (
        v === 'BLOCK' ? <Tag color="red">阻断</Tag>
          : v === 'REVIEW' ? <Tag color="purple">REVIEW</Tag>
            : <Tag color="orange">警告</Tag>
      ),
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

  const deviceColumns: ColumnsType<PreviewDevice> = [
    { title: '显示编号', dataIndex: 'display_no', width: 160 },
    { title: '名称', dataIndex: 'name', ellipsis: true },
    { title: '型号', dataIndex: 'model', width: 140, ellipsis: true, render: (v: string) => v || '-' },
    { title: '类别', dataIndex: 'category', width: 120 },
    { title: '来源行', dataIndex: 'group_rows', width: 100 },
    {
      title: '状态', dataIndex: 'ok', width: 90,
      render: (v: boolean) => (v ? <Tag color="green">可导入</Tag> : <Tag color="red">阻断</Tag>),
    },
  ];

  const suspectColumns: ColumnsType<SuspectItem> = [
    {
      title: '勾选为当前外借', key: 'pick', width: 150,
      render: (_: unknown, r: SuspectItem) => (
        <Checkbox checked={!!suspectPicked[r.source_key]}
          onChange={(e) => setSuspectPicked((prev) => ({ ...prev, [r.source_key]: e.target.checked }))}>
          确认仍借出
        </Checkbox>
      ),
    },
    { title: '显示编号', dataIndex: 'display_no', width: 160 },
    { title: '名称', dataIndex: 'name', ellipsis: true },
    { title: '型号', dataIndex: 'model', width: 130, render: (v: string) => v || '-' },
    { title: '公司', dataIndex: 'company', width: 180, ellipsis: true },
    { title: '借出日期', dataIndex: 'borrow_date', width: 110 },
    { title: '事件行', dataIndex: 'row', width: 80 },
  ];

  const reviewColumns: ColumnsType<ReviewItem> = [
    {
      title: '已确认', key: 'ack', width: 90,
      render: (_: unknown, r: ReviewItem) => (
        <Checkbox checked={!!reviewAck[r.key]}
          onChange={(e) => setReviewAck((prev) => ({ ...prev, [r.key]: e.target.checked }))} />
      ),
    },
    { title: '行', dataIndex: 'row', width: 80 },
    { title: '分组', dataIndex: 'group', width: 180, ellipsis: true },
    { title: '说明', dataIndex: 'message' },
    { title: '原始值', dataIndex: 'raw', width: 240, ellipsis: true, render: (v: string) => v || '-' },
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
      render: (v: string) => <Tag color={v === 'IN_STOCK' ? 'green' : v === 'BORROWED' ? 'orange' : 'blue'}>{statusText[v] ?? v}</Tag>,
    },
    { title: '到达时间', dataIndex: 'current_since', width: 170, render: (v: string | null) => v ?? '-' },
  ];

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Card title="Excel 导入（Preview / Review）">
        <Space direction="vertical" style={{ width: '100%' }}>
          <Alert
            type="info"
            showIcon
            message="导入流程：选择 Excel → 解析预览 → 人工审阅（REVIEW 清点 + 疑似在借勾选）→ 确认导入 → 报告"
            description="仅支持首次空库全量导入。设备默认以【在库】导入；勾选「疑似在借」为当前仍外借的设备将在导入时置为外借并生成外借单（仅外部公司，含『双发』的内部单位不作外借）。Excel 只作为一次性数据来源。"
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
              <Descriptions.Item label="Excel 台账数量">{summary.total_d}</Descriptions.Item>
              <Descriptions.Item label="预计新增设备"><b>{summary.ok_devices}</b> 台</Descriptions.Item>
              <Descriptions.Item label="历史借出事件">{summary.borrow_events}（内部调拨 {summary.internal_events}）</Descriptions.Item>
              <Descriptions.Item label="当前疑似在借">{summary.suspected}</Descriptions.Item>
              <Descriptions.Item label="无编号设备">{summary.unnumbered} 台</Descriptions.Item>
              <Descriptions.Item label="重复编号">{summary.duplicates} 次（同号多台真机）</Descriptions.Item>
              <Descriptions.Item label="需人工确认(REVIEW)"><b style={{ color: '#722ed1' }}>{summary.review_items}</b></Descriptions.Item>
              <Descriptions.Item label="错误(BLOCK)"><b style={{ color: '#cf1322' }}>{summary.blocks}</b> 条</Descriptions.Item>
              <Descriptions.Item label="分组">{summary.groups}（可导入 {summary.ok_groups}）</Descriptions.Item>
              <Descriptions.Item label="将跳过">{summary.block_devs} 台</Descriptions.Item>
            </Descriptions>
          </Card>

          {/* 设备级预览（§三十二） */}
          <Card title={`设备级预览（前 ${devices.length} / ${summary.ok_devices + summary.block_devs} 台；完整数据导入后见台账）`} size="small">
            <Table
              rowKey="source_key"
              size="small"
              columns={deviceColumns}
              dataSource={devices}
              pagination={{ pageSize: 10, showSizeChanger: false }}
            />
          </Card>

          {/* 疑似在借勾选（§三十三/决策18③） */}
          <Card
            title={
              <span>
                疑似在借（外部公司，最后借出无归还证据）
                <Typography.Text type="secondary" style={{ marginLeft: 8, fontWeight: 400 }}>
                  勾选 = 确认该设备当前仍在外借；导入时置外借并生成外借单。不勾选则默认在库。
                </Typography.Text>
              </span>
            }
            size="small"
          >
            {suspects.length === 0 ? (
              <Alert type="info" showIcon message="未发现需人工确认的疑似在借设备（外部公司）。" />
            ) : (
              <>
                <Button size="small" style={{ marginBottom: 8 }}
                  onClick={() => {
                    const all = Object.fromEntries(suspects.map((s) => [s.source_key, true]));
                    setSuspectPicked(all);
                  }}>全选</Button>
                <Button size="small" style={{ marginBottom: 8, marginLeft: 8 }}
                  onClick={() => setSuspectPicked({})}>清空</Button>
                <Table
                  rowKey="source_key"
                  size="small"
                  columns={suspectColumns}
                  dataSource={suspects}
                  pagination={{ pageSize: 10, showSizeChanger: false }}
                />
              </>
            )}
          </Card>

          {/* REVIEW 需人工确认（§三十三） */}
          <Card
            title={
              <span>
                需人工确认（REVIEW，未清点前禁止导入）
                {reviewPending > 0 && <Tag color="purple" style={{ marginLeft: 8 }}>剩 {reviewPending} 项</Tag>}
              </span>
            }
            size="small"
          >
            {reviews.length === 0 ? (
              <Alert type="success" showIcon message="无 REVIEW 项，可直接确认导入。" />
            ) : (
              <>
                <Alert style={{ marginBottom: 8 }} type="warning" showIcon
                  message={`共 ${reviews.length} 项需逐条勾选确认后才能导入（宁可保留待确认，不静默丢弃）。`} />
                <Table
                  rowKey="key"
                  size="small"
                  columns={reviewColumns}
                  dataSource={reviews}
                  pagination={{ pageSize: 10, showSizeChanger: false }}
                />
              </>
            )}
          </Card>

          {/* 问题表（BLOCK/WARN/REVIEW issue） */}
          {(issues.length > 0) && (
            <Card title="校验问题明细" size="small">
              <Table
                rowKey={(r) => `${r.level}-${r.code}-${r.row}-${r.group ?? ''}`}
                size="small"
                columns={issueColumns}
                dataSource={issues}
                pagination={{ pageSize: 10 }}
              />
            </Card>
          )}

          <Card title="分组预览" size="small">
            <Table
              rowKey={(r) => `${r.name}\u0000${r.model}\u0000${r.rows}`}
              size="small"
              columns={groupColumns}
              dataSource={groups}
              pagination={{ pageSize: 10 }}
            />
          </Card>

          <Alert
            type={importDisabled ? 'warning' : 'success'}
            showIcon
            message={
              importDisabled
                ? `还有 ${reviewPending} 项 REVIEW 未确认，暂不能导入。`
                : `REVIEW 已全部确认（${reviews.length} 项），可执行导入。`
            }
          />
          <Button
            type="primary"
            danger
            loading={running}
            disabled={importDisabled}
            onClick={confirmImport}
          >
            确认导入到系统
          </Button>
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
            <Descriptions.Item label="勾选确认当前外借"><b style={{ color: '#fa8c16' }}>{report.borrowed_now}</b> 台</Descriptions.Item>
            <Descriptions.Item label="重复编号">{report.duplicates} 次</Descriptions.Item>
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
            <>
              <Typography.Paragraph style={{ marginTop: 12 }} type="secondary">
                当前系统共 {eq.total} 台设备（前 200 条）：
              </Typography.Paragraph>
              <Table rowKey="id" size="small" columns={eqColumns} dataSource={eq.items.slice(0, 50)} pagination={false} />
            </>
          )}
        </Card>
      )}
    </Space>
  );
}
