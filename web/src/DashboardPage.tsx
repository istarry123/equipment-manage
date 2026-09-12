import { useCallback, useEffect, useRef, useState } from 'react';
import { Alert, Button, Card, Space, Table, Tag, Typography } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import * as echarts from 'echarts';
import { listPagination } from './pagination';
import { getAlias } from './theme';
import { ECHARTS_THEME } from './echartsTheme';
import { useThemeMode } from './themeMode';
import { statusFillColor, statusFillVar, statusText } from './status';
import {
  buildStatusKpis,
  countOfStatus,
  overdueSummary,
  percentOf,
} from './dashboardModel';
import type { BorrowLine, Dashboard, FlowLine, NameCount, TeamCatRow } from './dashboardModel';
import './dashboard.css';

/**
 * Dashboard（v1.4 Phase 3）。
 *
 * 数据来源：现有 `GET /api/dashboard`（**后端未做任何改动**）。
 * 本页改造仅限呈现：所有数值仍逐项取自同一响应，派生值（占比）集中在 dashboardModel.ts。
 */
function Chart({ option, height = 260 }: { option: Record<string, unknown>; height?: number }) {
  const { mode } = useThemeMode();
  const ref = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    if (!ref.current) return;
    // ECharts 画布不参与 antd CSS-in-JS，必须显式传入主题（见 echartsTheme.ts）
    const chart = echarts.init(ref.current, ECHARTS_THEME[mode]);
    chart.setOption(option as never);
    const onResize = () => chart.resize();
    window.addEventListener('resize', onResize);
    return () => {
      window.removeEventListener('resize', onResize);
      chart.dispose();
    };
  }, [option, mode]);
  return <div ref={ref} style={{ height, width: '100%' }} />;
}

/** KPI 卡片：状态圆点 + 大字号数值 + 占比 */
function KpiCard({
  label,
  value,
  dotColor,
  percent,
  danger,
}: {
  label: string;
  value: number;
  dotColor: string;
  percent?: number;
  danger?: boolean;
}) {
  return (
    <div className="dash-kpi">
      <div className="dash-kpi__label">
        <span className="dash-kpi__dot" style={{ background: dotColor }} />
        <span>{label}</span>
      </div>
      <div className={danger ? 'dash-kpi__value dash-kpi__value--danger num' : 'dash-kpi__value num'}>{value}</div>
      {typeof percent === 'number' && <div className="dash-kpi__percent">{percent}%</div>}
    </div>
  );
}

export default function DashboardPage({ onOpenTeamView }: { onOpenTeamView?: () => void } = {}) {
  const [d, setD] = useState<Dashboard | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const { mode } = useThemeMode();

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

  const total = d?.total ?? 0;
  const kpis = buildStatusKpis(d);
  const overdue = overdueSummary(d);
  const inStockPercent = percentOf(total, countOfStatus(d, 'IN_STOCK'));
  const borrowedPercent = percentOf(total, countOfStatus(d, 'BORROWED'));

  const donutOption: Record<string, unknown> = {
    tooltip: { trigger: 'item' },
    legend: { bottom: 0 },
    series: [
      {
        type: 'pie', radius: ['42%', '68%'], avoidLabelOverlap: true,
        label: { show: false }, emphasis: { label: { show: true } },
        data: (d?.by_status ?? []).map((s) => ({
          name: statusText(s.status),
          value: s.count,
          itemStyle: { color: statusFillColor(mode, s.status) },
        })),
      },
    ],
  };
  const barOption = (rows: NameCount[]): Record<string, unknown> => ({
    tooltip: { trigger: 'axis' },
    grid: { left: 8, right: 16, bottom: 8, top: 24, containLabel: true },
    xAxis: { type: 'value', minInterval: 1 },
    yAxis: { type: 'category', data: rows.map((r) => r.name) },
    series: [{ type: 'bar', barWidth: 14, data: rows.map((r) => r.count), itemStyle: { color: getAlias(mode).brand } }],
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
    <Space direction="vertical" size={24} style={{ width: '100%' }}>
      {error && <Alert type="error" showIcon message={error} action={<Button onClick={() => void load()}>重试</Button>} />}
      {d && (
        <>
          {/* Hero：设备总数（大字号 + 大留白） */}
          <section className="dash-hero">
            <div className="dash-hero__label">设备总数</div>
            <div className="dash-hero__value num">{total}</div>
            <div className="dash-hero__hint">
              在库 {inStockPercent}% · 外借 {borrowedPercent}% · 逾期 {overdue.count} 台
            </div>
          </section>

          {/* KPI 卡片网格：逐项对应 by_status，并追加逾期外借 */}
          <div className="dash-kpis">
            {kpis.map((k) => (
              <KpiCard
                key={k.status}
                label={statusText(k.status)}
                value={k.count}
                dotColor={statusFillVar(k.status)}
                percent={k.percent}
              />
            ))}
            <KpiCard
              label="逾期外借"
              value={overdue.count}
              dotColor={overdue.hasOverdue ? 'var(--danger-text)' : 'var(--text-dimmed)'}
              danger={overdue.hasOverdue}
            />
          </div>

          <div className="dash-grid-3">
            <Card title={`状态分布（共 ${total} 台）`}>
              <Chart option={donutOption} height={240} />
            </Card>
            <Card title="各班组/内部单位设备数">
              {(d.by_team?.length ?? 0) > 0
                ? <Chart option={barOption(d.by_team ?? [])} height={240} />
                : <div className="dash-empty" style={{ height: 240, lineHeight: '240px' }}>暂无数据</div>}
            </Card>
            <Card title="各类别设备数">
              {(d.by_category?.length ?? 0) > 0
                ? <Chart option={barOption(d.by_category ?? [])} height={240} />
                : <div className="dash-empty" style={{ height: 240, lineHeight: '240px' }}>暂无数据</div>}
            </Card>
          </div>

          <div className="dash-grid-2">
            <Card
              title="最近流转"
              extra={<Button size="small" icon={<ReloadOutlined />} onClick={() => void load()} loading={loading}>刷新</Button>}
            >
              <Table rowKey={(r) => `${r.equipment_id}-${r.occurred_at}-${r.action_text}`} size="small"
                columns={flowCols} dataSource={d.recent_flows ?? []} pagination={listPagination()} />
            </Card>
            <Card
              title="当前外借"
              extra={<Tag color={overdue.hasOverdue ? 'red' : 'green'}>{overdue.hasOverdue ? `${overdue.count} 台逾期` : '无逾期'}</Tag>}
            >
              <Table rowKey="borrow_record_id" size="small" columns={borrowCols}
                dataSource={d.current_borrows ?? []} pagination={listPagination()} />
            </Card>
          </div>

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
      <Card title="各班组设备使用情况" extra={
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
