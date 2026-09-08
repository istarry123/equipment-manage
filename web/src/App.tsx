import { useState } from 'react';
import { Badge, Layout, Menu, Typography } from 'antd';
import {
  AppstoreOutlined,
  DatabaseOutlined,
  DashboardOutlined,
  ExportOutlined,
  ImportOutlined,
  SettingOutlined,
  SwapOutlined,
  TeamOutlined,
} from '@ant-design/icons';
import DashboardPage from './DashboardPage';
import ImportPage from './ImportPage';

const { Header, Sider, Content } = Layout;

// 主功能导航：随 Phase 推进逐步开放（禁用项=后续阶段功能）
const MENU_ITEMS = [
  { key: 'dashboard', icon: <DashboardOutlined />, label: 'Dashboard' },
  { key: 'equipment', icon: <AppstoreOutlined />, label: '设备台账', disabled: true },
  { key: 'flow', icon: <SwapOutlined />, label: '设备流转', disabled: true },
  { key: 'borrow', icon: <ExportOutlined />, label: '外借管理', disabled: true },
  { key: 'team', icon: <TeamOutlined />, label: '班组管理', disabled: true },
  { key: 'import', icon: <ImportOutlined />, label: '数据导入' },
  { key: 'backup', icon: <DatabaseOutlined />, label: '数据备份', disabled: true },
  { key: 'settings', icon: <SettingOutlined />, label: '系统设置', disabled: true },
];

function renderPage(key: string) {
  switch (key) {
    case 'import':
      return <ImportPage />;
    case 'dashboard':
    default:
      return <DashboardPage />;
  }
}

export default function App() {
  const [active, setActive] = useState('dashboard');

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Header style={{ display: 'flex', alignItems: 'center', background: '#001529' }}>
        <Typography.Title level={4} style={{ color: '#fff', margin: 0 }}>
          设备资产与流转管理系统
        </Typography.Title>
        <Badge
          status="processing"
          text={<span style={{ color: '#aaa' }}>Phase 2</span>}
          style={{ marginLeft: 24 }}
        />
      </Header>
      <Layout>
        <Sider width={200} theme="dark">
          <Menu
            theme="dark"
            mode="inline"
            selectedKeys={[active]}
            items={MENU_ITEMS}
            onClick={({ key }) => setActive(key)}
            style={{ height: '100%', borderRight: 0 }}
          />
        </Sider>
        <Content style={{ margin: 16 }}>{renderPage(active)}</Content>
      </Layout>
    </Layout>
  );
}
