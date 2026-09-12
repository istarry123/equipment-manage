import { useState } from 'react';
import { Badge, Layout, Menu, Typography } from 'antd';
import {
  AppstoreOutlined,
  DatabaseOutlined,
  DashboardOutlined,
  ExportOutlined,
  FileAddOutlined,
  ImportOutlined,
  ProfileOutlined,
  SettingOutlined,
  SwapOutlined,
  TeamOutlined,
} from '@ant-design/icons';
import DashboardPage from './DashboardPage';
import EquipmentPage from './EquipmentPage';
import ImportPage from './ImportPage';
import ImportDetailPage from './ImportDetailPage';
import TeamsPage from './TeamsPage';
import BorrowsPage from './BorrowsPage';
import BackupPage from './BackupPage';
import SettingsPage from './SettingsPage';
import TeamViewPage from './TeamViewPage';

const { Header, Sider, Content } = Layout;

const MENU_ITEMS = [
  { key: 'dashboard', icon: <DashboardOutlined />, label: 'Dashboard' },
  { key: 'equipment', icon: <AppstoreOutlined />, label: '设备台账' },
  { key: 'teamview', icon: <ProfileOutlined />, label: '班组设备' },
  { key: 'flow', icon: <SwapOutlined />, label: '设备流转' },
  { key: 'borrow', icon: <ExportOutlined />, label: '外借管理' },
  { key: 'team', icon: <TeamOutlined />, label: '班组管理' },
  { key: 'import', icon: <ImportOutlined />, label: '数据导入' },
  { key: 'importDetail', icon: <FileAddOutlined />, label: '外借明细导入' },
  { key: 'backup', icon: <DatabaseOutlined />, label: '数据备份' },
  { key: 'settings', icon: <SettingOutlined />, label: '系统设置' },
];

export default function App() {
  const [active, setActive] = useState('dashboard');
  // 跨页打开设备详情：由班组设备页点击编号触发，复用设备台账页既有详情抽屉
  const [detailRequestId, setDetailRequestId] = useState<number | null>(null);

  const goMenu = (key: string) => {
    setActive(key);
    if (key !== 'equipment' && key !== 'flow') {
      setDetailRequestId(null);
    }
  };

  const openDeviceDetail = (id: number) => {
    setDetailRequestId(id);
    setActive('equipment');
  };

  const renderPage = () => {
    switch (active) {
      case 'equipment':
      case 'flow': // 流转操作入口：设备详情中按状态提供可用动作
        return <EquipmentPage requestOpenId={detailRequestId} />;
      case 'teamview':
        return <TeamViewPage onOpenDevice={openDeviceDetail} />;
      case 'borrow':
        return <BorrowsPage />;
      case 'team':
        return <TeamsPage />;
      case 'import':
        return <ImportPage />;
      case 'importDetail':
        return <ImportDetailPage />;
      case 'backup':
        return <BackupPage />;
      case 'settings':
        return <SettingsPage />;
      case 'dashboard':
      default:
        return <DashboardPage onOpenTeamView={() => goMenu('teamview')} />;
    }
  };

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Header style={{ display: 'flex', alignItems: 'center', background: '#001529' }}>
        <Typography.Title level={4} style={{ color: '#fff', margin: 0 }}>
          设备资产与流转管理系统
        </Typography.Title>
        <Badge
          status="processing"
          text={<span style={{ color: '#aaa' }}>v1.2.0</span>}
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
            onClick={({ key }) => goMenu(key)}
            style={{ height: '100%', borderRight: 0 }}
          />
        </Sider>
        <Content style={{ margin: 16 }}>{renderPage()}</Content>
      </Layout>
    </Layout>
  );
}
