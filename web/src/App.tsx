import { useEffect, useState } from 'react';
import { Navigate, Route, Routes, useLocation, useNavigate } from 'react-router-dom';
import { Badge, Button, Layout, Menu, Tooltip, Typography } from 'antd';
import { BulbOutlined } from '@ant-design/icons';
import DashboardPage from './DashboardPage';
import EquipmentPage from './EquipmentPage';
import ImportPage from './ImportPage';
import ImportDetailPage from './ImportDetailPage';
import TeamsPage from './TeamsPage';
import BorrowsPage from './BorrowsPage';
import BackupPage from './BackupPage';
import SettingsPage from './SettingsPage';
import TeamViewPage from './TeamViewPage';
import PageHeader from './PageHeader';
import { DEFAULT_PATH, ROUTES, routeByPath } from './routes';
import { useThemeMode } from './themeMode';

const { Header, Sider, Content } = Layout;

const APP_NAME = '设备资产与流转管理系统';
// v1.4.0：界面改版（设计令牌 / 路由化 / Dashboard / 列表页 / 危险操作）随本次发布统一收口
const APP_VERSION = 'v1.4.0';

function pathOf(key: string): string {
  return ROUTES.find((r) => r.key === key)?.path ?? DEFAULT_PATH;
}

/**
 * 应用外壳（v1.4 Phase 2）：顶栏 + 可折叠侧栏 + 内容区（统一页面头）。
 * 页面组件本身不做任何改动；菜单、页面标题、document.title 均由 routes.tsx 一处驱动。
 */
export default function App() {
  const navigate = useNavigate();
  const location = useLocation();
  const { mode, toggleMode } = useThemeMode();
  const [collapsed, setCollapsed] = useState(false);

  const meta = routeByPath(location.pathname);
  const activeKey = meta?.key ?? 'dashboard';
  // 跨页打开设备详情：由班组设备页经路由 state 传递，本组件不再持有跳转状态
  const detailRequestId = (location.state as { openDeviceId?: number } | null)?.openDeviceId ?? null;

  useEffect(() => {
    document.title = meta ? `${meta.label} · ${APP_NAME}` : APP_NAME;
  }, [meta]);

  const goMenu = (key: string) => {
    navigate(pathOf(key));
  };

  const openDeviceDetail = (id: number) => {
    navigate(pathOf('equipment'), { state: { openDeviceId: id } });
  };

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Header
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 12,
          background: 'var(--bg-layer-1)',
          borderBottom: '1px solid var(--border-l1)',
        }}
      >
        <Typography.Title
          level={4}
          onClick={() => navigate(DEFAULT_PATH)}
          style={{ color: 'var(--text-primary)', margin: 0, cursor: 'pointer', whiteSpace: 'nowrap' }}
        >
          {APP_NAME}
        </Typography.Title>
        <Badge status="processing" text={<span style={{ color: 'var(--text-tertiary)' }}>{APP_VERSION}</span>} />
        <div style={{ marginLeft: 'auto' }}>
          <Tooltip title={mode === 'dark' ? '切换到浅色模式' : '切换到深色模式'}>
            <Button
              type="text"
              icon={<BulbOutlined />}
              onClick={toggleMode}
              style={{ color: 'var(--text-secondary)' }}
              aria-label="切换深浅色模式"
            >
              {mode === 'dark' ? '浅色' : '深色'}
            </Button>
          </Tooltip>
        </div>
      </Header>
      <Layout>
        <Sider
          className="app-sider"
          width={200}
          collapsedWidth={56}
          collapsible
          collapsed={collapsed}
          onCollapse={setCollapsed}
          style={{ background: 'var(--bg-layer-1)', borderRight: '1px solid var(--border-l1)' }}
        >
          <Menu
            mode="inline"
            selectedKeys={[activeKey]}
            items={ROUTES.map((r) => ({ key: r.key, icon: r.icon, label: r.label }))}
            onClick={({ key }) => goMenu(key)}
            style={{ borderRight: 0, background: 'transparent' }}
          />
        </Sider>
        <Content style={{ margin: 16, minWidth: 0 }}>
          <PageHeader title={meta?.label ?? APP_NAME} description={meta?.description} />
          <Routes>
            <Route path="/" element={<Navigate to={DEFAULT_PATH} replace />} />
            <Route path="/dashboard" element={<DashboardPage onOpenTeamView={() => goMenu('teamview')} />} />
            {/* 「设备流转」与「设备台账」共用同一页面组件（详情内按状态提供流转动作） */}
            <Route path="/equipment" element={<EquipmentPage requestOpenId={detailRequestId} />} />
            <Route path="/flow" element={<EquipmentPage requestOpenId={detailRequestId} />} />
            <Route path="/teamview" element={<TeamViewPage onOpenDevice={openDeviceDetail} />} />
            <Route path="/borrow" element={<BorrowsPage />} />
            <Route path="/team" element={<TeamsPage />} />
            <Route path="/import" element={<ImportPage />} />
            <Route path="/import-detail" element={<ImportDetailPage />} />
            <Route path="/backup" element={<BackupPage />} />
            <Route path="/settings" element={<SettingsPage />} />
            {/* 未登记路径（含后端 SPA 回退送来的任意 URL）统一回默认页 */}
            <Route path="*" element={<Navigate to={DEFAULT_PATH} replace />} />
          </Routes>
        </Content>
      </Layout>
    </Layout>
  );
}
