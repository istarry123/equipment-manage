import { useCallback, useEffect, useState } from 'react';
import {
  Alert, Button, Card, Collapse, Col, Empty, Input, Row, Select, Space, Statistic, Table, Tag, Typography,
} from 'antd';
import { ReloadOutlined, SearchOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { listPagination } from './pagination';

interface Team { id: number; name: string; is_active: boolean }
interface Category { id: number; name: string }

interface TeamViewDevice {
  id: number; equipment_no: string | null; equipment_seq?: number; display_no?: string; internal_code: string;
  name: string; model: string; category: string; status: string; current_since: string;
}
interface CategoryGroup { category: string; count: number; devices: TeamViewDevice[] }
interface TeamGroup { id: number; name: string; total: number; categories: CategoryGroup[] }
interface Stats {
  team_count: number; total_equipment: number;
  in_team_equipment: number; unassigned_equipment: number;
}
interface TeamView {
  stats: Stats;
  teams: TeamGroup[];
  unassigned: CategoryGroup & { devices: TeamViewDevice[] };
  total_rows: number;
}

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

async function req<T>(url: string): Promise<T> {
  const resp = await fetch(url);
  const data = await resp.json().catch(() => null);
  if (!resp.ok) throw new Error(data?.error?.message ?? `HTTP ${resp.status}`);
  return data as T;
}

const displayNoOf = (d: TeamViewDevice): string => d.display_no ?? d.equipment_no ?? '无编号';

const columns = (onOpen: (id: number) => void): ColumnsType<TeamViewDevice> => [
  {
    title: '编号', dataIndex: 'display_no', width: 150,
    render: (_: unknown, r: TeamViewDevice) =>
      r.equipment_no ? (
        <Button type="link" style={{ padding: 0, fontWeight: 600 }} onClick={() => onOpen(r.id)}>
          {displayNoOf(r)}
        </Button>
      ) : (
        <Typography.Text type="secondary">{displayNoOf(r)}</Typography.Text>
      ),
  },
  { title: '名称', dataIndex: 'name', ellipsis: true },
  { title: '型号', dataIndex: 'model', width: 150, ellipsis: true, render: (v: string) => v || '-' },
  {
    title: '状态', dataIndex: 'status', width: 100,
    render: (v: string) => {
      const m = statusMeta[v];
      return m ? <Tag color={m.color}>{m.text}</Tag> : <Tag>{v}</Tag>;
    },
  },
  {
    title: '到达当前班组时间', dataIndex: 'current_since', width: 170,
    render: (v: string) => v || '-',
  },
];

export default function TeamViewPage({ onOpenDevice }: { onOpenDevice: (id: number) => void }) {
  const [view, setView] = useState<TeamView | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [teams, setTeams] = useState<Team[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);

  const [fTeam, setFTeam] = useState<string>(''); // '' 全部 | 'unassigned' | id
  const [fCategory, setFCategory] = useState<number | undefined>();
  const [fStatus, setFStatus] = useState<string>('');
  const [q, setQ] = useState('');
  const [qInput, setQInput] = useState('');

  const loadMeta = useCallback(async () => {
    try {
      const [t, c] = await Promise.all([req<Team[]>('/api/teams'), req<Category[]>('/api/categories')]);
      setTeams(t.filter((x) => x.is_active));
      setCategories(c);
    } catch {
      /* 元数据失败不阻塞 */
    }
  }, []);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const p = new URLSearchParams();
      if (fTeam) p.set('team', fTeam);
      if (fCategory) p.set('category', String(fCategory));
      if (fStatus) p.set('status', fStatus);
      if (q) p.set('q', q);
      const query = p.toString();
      setView(await req<TeamView>(`/api/teams/equipment${query ? `?${query}` : ''}`));
    } catch (e) {
      setError(e instanceof Error ? e.message : '加载失败');
      setView(null);
    } finally {
      setLoading(false);
    }
  }, [fTeam, fCategory, fStatus, q]);

  useEffect(() => {
    void loadMeta();
  }, [loadMeta]);
  useEffect(() => {
    void load();
  }, [load]);

  const reset = () => {
    setFTeam('');
    setFCategory(undefined);
    setFStatus('');
    setQ('');
    setQInput('');
    void load();
  };

  const onlyUnassigned = fTeam === 'unassigned';
  const shownTeams = onlyUnassigned ? [] : view?.teams ?? [];
  const unassigned = view?.unassigned;

  const renderTeamCard = (g: TeamGroup) => (
    <Card
      key={g.id}
      size="small"
      title={<span>{g.name} <Typography.Text type="secondary">（班组使用中）</Typography.Text></span>}
      extra={<span>共 <b>{g.total}</b> 台</span>}
      style={{ marginBottom: 12 }}
    >
      {g.categories.length === 0 ? (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无设备" />
      ) : (
        <Collapse
          size="small"
          items={g.categories.map((cg) => ({
            key: cg.category,
            label: `${cg.category}（${cg.count}台）`,
            children: (
              <Table
                rowKey="id" size="small" columns={columns(onOpenDevice)}
                dataSource={cg.devices} pagination={listPagination()}
              />
            ),
          }))}
        />
      )}
    </Card>
  );

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Card>
        <Typography.Title level={4} style={{ marginBottom: 0 }}>
          班组设备
        </Typography.Title>
        <Typography.Text type="secondary">查看各班组当前使用的设备及设备分类</Typography.Text>
      </Card>

      {view && (
        <Card size="small">
          <Row gutter={16}>
            <Col span={6}><Statistic title="班组数量" value={view.stats.team_count} /></Col>
            <Col span={6}><Statistic title="设备总数" value={view.stats.total_equipment} /></Col>
            <Col span={6}><Statistic title="班组使用设备" value={view.stats.in_team_equipment} /></Col>
            <Col span={6}><Statistic title="未分配设备" value={view.stats.unassigned_equipment} /></Col>
          </Row>
        </Card>
      )}

      <Card size="small">
        <Space wrap>
          <Select
            allowClear placeholder="班组" style={{ width: 170 }}
            options={[
              ...teams.map((t) => ({ value: String(t.id), label: t.name })),
              { value: 'unassigned', label: '未分配设备' },
            ]}
            value={fTeam || undefined}
            onChange={(v) => setFTeam(v ?? '')}
          />
          <Select
            allowClear placeholder="设备类别" style={{ width: 160 }}
            options={categories.map((c) => ({ value: c.id, label: c.name }))}
            value={fCategory}
            onChange={(v) => setFCategory(v)}
          />
          <Select
            allowClear placeholder="设备状态" style={{ width: 140 }}
            options={STATUS_OPTIONS.map((s) => ({ value: s.value, label: s.text }))}
            value={fStatus || undefined}
            onChange={(v) => setFStatus(v ?? '')}
          />
          <Input.Search
            allowClear placeholder="设备编号 / 名称 / 型号"
            style={{ width: 240 }}
            value={qInput}
            onChange={(e) => setQInput(e.target.value)}
            onSearch={(v) => setQ(v.trim())}
          />
          <Button type="primary" icon={<SearchOutlined />} onClick={() => setQ(qInput.trim())}>查询</Button>
          <Button onClick={reset}>重置</Button>
          <Button icon={<ReloadOutlined />} loading={loading} onClick={() => void load()}>刷新</Button>
        </Space>
      </Card>

      {error && (
        <Alert
          type="error" showIcon message={error}
          action={<Button size="small" onClick={() => void load()}>重新加载</Button>}
        />
      )}

      {!loading && view && !error && (
        <>
          {onlyUnassigned ? (
            <Card
              size="small"
              title={<span>未分配设备 <Typography.Text type="secondary">（在库/外借/维修等，current_team_id 为空）</Typography.Text></span>}
              extra={<span>共 <b>{unassigned?.count ?? 0}</b> 台</span>}
            >
              {!unassigned || unassigned.devices.length === 0 ? (
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="没有符合条件的设备" />
              ) : (
                <Table
                  rowKey="id" size="small" columns={columns(onOpenDevice)}
                  dataSource={unassigned.devices}
                  pagination={listPagination()}
                />
              )}
            </Card>
          ) : (
            <>
              {unassigned && unassigned.devices.length > 0 && (
                <Card
                  size="small"
                  title="未分配设备（异常管理视图）"
                  extra={<span>共 <b>{unassigned.count}</b> 台</span>}
                  style={{ marginBottom: 12 }}
                >
                  <Table
                    rowKey="id" size="small" columns={columns(onOpenDevice)}
                    dataSource={unassigned.devices} pagination={listPagination()}
                  />
                </Card>
              )}
              {shownTeams.length === 0 ? (
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="没有找到符合条件的设备" />
              ) : (
                shownTeams.map(renderTeamCard)
              )}
            </>
          )}
        </>
      )}
      {!loading && !error && !view && (
        <Card size="small"><Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无班组" /></Card>
      )}
    </Space>
  );
}
