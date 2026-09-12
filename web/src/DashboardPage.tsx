import { useCallback, useEffect, useRef, useState } from 'react';
import { Alert, Button, Card, Col, Row, Space, Statistic, Table, Tag, Typography } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import * as echarts from 'echarts';
import { listPagination } from './pagination';

interface StatusCount { status: string; count: number }
interface NameCount { name: string; count: number }
interface FlowLine {
  equipment_id: number; equipment_no: string; display_no?: string; name: string;
  action_text: string; to_team_name: string; borrower_name: string;
  occurred_at: string; operator: string;
}
interface BorrowLine {
  borrow_record_id: number; equipment_id: number; equipment_no: string; display_no?: string; name: string;
  borrower_name: string; borrow_date: string; expected_return_date: string; overdue_days: number;
}
interface TeamCatRow { team: string; category: string; count: number }
interface Dashboard {
  total: number; by_status: StatusCount[]; by_category: NameCount[];
  by_team: NameCount[]; recent_flows: FlowLine[]; current_borrows: BorrowLine[];
  overdue_count: number; by_team_category: TeamCatRow[];
}

const STATUS_COLOR: Record<string, string> = {
  IN_STOCK: '#52c41a', IN_TEAM: '#1677ff', BORROWED: '#fa8c16',
  MAINTENANCE: '#f5222d', SCRAPPED: '#8c8c8c', OTHER: '#722ed1',
};
const STATUS_TEXT: Record<string, string> = {
  IN_STOCK: '在库', IN_TEAM: '班组使用', BORROWED: '外借',
  MAINTENANCE: '维修', SCRAPPED: '报废', OTHER: '其他',
};

function Chart({ option, height = 260 }: { option: Record<string, unknown>; height?: number }) {
  const ref = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    if (!ref.current) return;
    const chart = echarts.init(ref.current);
    chart.setOption(option as never);
    const onResize = () => chart.resize();
    window.addEventListener('resize', onResize);
    return () => {
      window.removeEventListener('resize', onResize);
      chart.dispose();
    };
  }, [option]);
  return <div ref={ref} style={{ height, width: '100%' }} />;
}

export default function DashboardPage({ onOpenTeamView }: { onOpenTeamView?: () => void } = {}) {
  const [d, setD] = useState<Dashboard | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const resp = await fetch('/api/dashboard');
      const data = await resp.json().catch(() => null);
      if (!resp.ok) throw new Error(data?.error?.message ?? `HTTP ${resp.status}`);
      setD(data as Dashboard);
    } catch (e) {
      setError(e instanceof Error ? e.message : '加载失败');
      setD(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const donutOption: Record<string, unknown> = {
    tooltip: { trigger: 'item' },
    legend: { bottom: 0 },
    series: [
      {
        type: 'pie', radius: ['42%', '68%'], avoidLabelOverlap: true,
        label: { show: false }, emphasis: { label: { show: true } },
        data: (d?.by_status ?? []).map((s) => ({
          name: STATUS_TEXT[s.status] ?? s.status,
          value: s.count,
          itemStyle: { color: STATUS_COLOR[s.status] },
        })),
      },
    ],
  };
  const barOption = (rows: NameCount[]): Record<string, unknown> => ({
    tooltip: { trigger: 'axis' },
    grid: { left: 8, right: 16, bottom: 8, top: 24, containLabel: true },
    xAxis: { type: 'value', minInterval: 1 },
    yAxis: { type: 'category', data: rows.map((r) => r.name) },
    series: [{ type: 'bar', barWidth: 14, data: rows.map((r) => r.count), itemStyle: { color: '#1677ff' } }],
  });

  const flowCols: ColumnsType<FlowLine> = [
    {
      title: '设备', key: 'eq', render: (_: unknown, r: FlowLine) => (
        <span>{r.display_no || r.equipment_no || '无编号'} <Typography.Text type="secondary">{r.name}</Typography.Text></span>
      ),
    },
    { title: '动作', dataIndex: 'action_text', width: 110 },
    { title: '去向', key: 'to', width: 150, render: (_: unknown, r: FlowLine) => r.to_team_name || r.borrower_name || '-' },
    { title: '时间', dataIndex: 'occurred_at', width: 120 },
    { title: '操作人', dataIndex: 'operator', width: 90 },
  ];
  const borrowCols: ColumnsType<BorrowLine> = [
    {
      title: '设备', key: 'eq', render: (_: unknown, r: BorrowLine) => (
        <span>{r.display_no || r.equipment_no || '无编号'} <Typography.Text type="secondary">{r.name}</Typography.Text></span>
      ),
    },
    { title: '外借方', dataIndex: 'borrower_name', width: 170, ellipsis: true },
    { title: '借出', dataIndex: 'borrow_date', width: 80 },
    {
      title: '预计归还', dataIndex: 'expected_return_date', width: 150,
      render: (v: string, r: BorrowLine) =>
        r.overdue_days > 0 ? <Tag color="red">逾期 {r.overdue_days} 天</Tag> : (v || '-'),
    },
  ];

  if (!d && !error) {
    return <Card loading />;
  }
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      {error && <Alert type="error" showIcon message={error} action={<Button onClick={() => void load()}>重试</Button>} />}
      {d && (
        <>
          <Card
            title="设备概览"
            extra={<Button icon={<ReloadOutlined />} onClick={() => void load()} loading={loading}>刷新</Button>}
          >
            <Row gutter={16}>
              <Col span={4}><Card size="small"><Statistic title="设备总数" value={d.total} /></Card></Col>
              {(d.by_status ?? []).map((s) => (
                <Col span={3} key={s.status}>
                  <Card size="small">
                    <Statistic
                      title={<span style={{ color: STATUS_COLOR[s.status] }}>{STATUS_TEXT[s.status]}</span>}
                      value={s.count}
                    />
                  </Card>
                </Col>
              ))}
              <Col span={3}>
                <Card size="small">
                  <Statistic title={<span style={{ color: '#cf1322' }}>逾期外借</span>} value={d.overdue_count} />
                </Card>
              </Col>
            </Row>
          </Card>

          <Row gutter={16}>
            <Col span={8}>
              <Card title="状态分布" size="small"><Chart option={donutOption} height={240} /></Card>
            </Col>
            <Col span={8}>
              <Card title="各班组/内部单位设备数" size="small">
                {(d.by_team?.length ?? 0) > 0
                  ? <Chart option={barOption(d.by_team ?? [])} height={240} />
                  : <div style={{ height: 240, lineHeight: '240px', textAlign: 'center', color: '#999' }}>暂无数据</div>}
              </Card>
            </Col>
            <Col span={8}>
              <Card title="各类别设备数" size="small"><Chart option={barOption(d.by_category ?? [])} height={240} /></Card>
            </Col>
          </Row>

          <Row gutter={16}>
            <Col span={12}>
              <Card title="最近流转" size="small">
                <Table rowKey={(r) => `${r.equipment_id}-${r.occurred_at}-${r.action_text}`} size="small"
                  columns={flowCols} dataSource={d.recent_flows ?? []} pagination={listPagination()} />
              </Card>
            </Col>
            <Col span={12}>
              <Card
                title="当前外借"
                size="small"
                extra={<Tag color={d.overdue_count > 0 ? 'red' : 'green'}>{d.overdue_count > 0 ? `${d.overdue_count} 台逾期` : '无逾期'}</Tag>}
              >
                <Table rowKey="borrow_record_id" size="small" columns={borrowCols}
                  dataSource={d.current_borrows ?? []} pagination={listPagination()} />
              </Card>
            </Col>
          </Row>

          <TeamCategoryMatrix rows={d.by_team_category ?? []} onOpenTeamView={onOpenTeamView} />
        </>
      )}
    </Space>
  );
}

// 各班组设备使用情况（班组 × 类别，合计列与班组设备页/台账同源）
function TeamCategoryMatrix({
  rows, onOpenTeamView,
}: {
  rows: TeamCatRow[];
  onOpenTeamView?: () => void;
}) {
  if (rows.length === 0) {
    return (
      <Card title="各班组设备使用情况" size="small" extra={
        onOpenTeamView && <Button type="link" onClick={onOpenTeamView}>查看全部班组设备 →</Button>
      }>
        <Typography.Text type="secondary">暂无班组设备（可先在班组管理/设备流转中分配）</Typography.Text>
      </Card>
    );
  }
  const cats: string[] = [];
  for (const r of rows) {
    if (!cats.includes(r.category)) cats.push(r.category);
  }
  const byTeam = new Map<string, Map<string, number>>();
  let order: string[] = [];
  for (const r of rows) {
    if (!byTeam.has(r.team)) {
      byTeam.set(r.team, new Map());
      order.push(r.team);
    }
    byTeam.get(r.team)!.set(r.category, r.count);
  }
  const colData = order.map((tm) => {
    const m = byTeam.get(tm)!;
    let sum = 0;
    const rec: Record<string, string | number> = { team: tm };
    for (const c of cats) {
      rec[c] = m.get(c) ?? 0;
      sum += m.get(c) ?? 0;
    }
    rec.total = sum;
    return rec;
  });
  return (
    <Card
      title="各班组设备使用情况"
      size="small"
      extra={onOpenTeamView && <Button type="link" onClick={onOpenTeamView}>查看全部班组设备 →</Button>}
    >
      <Table
        rowKey="team" size="small" pagination={listPagination()}
        dataSource={colData}
        columns={[
          { title: '班组', dataIndex: 'team', width: 140, fixed: 'left' as const },
          ...cats.map((c) => ({ title: c, dataIndex: c, align: 'right' as const })),
          { title: '合计', dataIndex: 'total', align: 'right' as const, render: (v: number) => <b>{v}</b> },
        ]}
      />
    </Card>
  );
}
