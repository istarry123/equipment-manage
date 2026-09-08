import { useCallback, useEffect, useState } from 'react';
import {
  Alert,
  Badge,
  Button,
  Card,
  Descriptions,
  Layout,
  Menu,
  Space,
  Spin,
  Tag,
  Typography,
} from 'antd';
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
import { getHealth, type Health } from './api';

const { Header, Sider, Content } = Layout;

// 主功能导航：占位（后续 Phase 依次开放）
const MENU_ITEMS = [
  { key: 'dashboard', icon: <DashboardOutlined />, label: 'Dashboard' },
  { key: 'equipment', icon: <AppstoreOutlined />, label: '设备台账', disabled: true },
  { key: 'flow', icon: <SwapOutlined />, label: '设备流转', disabled: true },
  { key: 'borrow', icon: <ExportOutlined />, label: '外借管理', disabled: true },
  { key: 'team', icon: <TeamOutlined />, label: '班组管理', disabled: true },
  { key: 'import', icon: <ImportOutlined />, label: '数据导入', disabled: true },
  { key: 'backup', icon: <DatabaseOutlined />, label: '数据备份', disabled: true },
  { key: 'settings', icon: <SettingOutlined />, label: '系统设置', disabled: true },
];

function DashboardPage() {
  const [health, setHealth] = useState<Health | null>(null);
  const [error, setError] = useState<string>('');
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      setHealth(await getHealth());
    } catch (e) {
      setError(e instanceof Error ? e.message : '未知错误');
      setHealth(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
      <Card title="系统状态">
        {loading ? (
          <Spin tip="正在连接后端..." />
        ) : error ? (
          <Alert
            type="error"
            showIcon
            message="无法连接后端服务"
            description={error}
            action={<Button onClick={() => void load()}>重试</Button>}
          />
        ) : health ? (
          <Descriptions column={2} bordered size="small">
            <Descriptions.Item label="服务状态">
              <Tag color="green">{health.status === 'ok' ? '运行正常' : health.status}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label="数据库">
              <Tag color={health.db === 'up' ? 'green' : 'red'}>{health.db === 'up' ? '连接正常' : health.db}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label="后端版本">{health.version}</Descriptions.Item>
            <Descriptions.Item label="服务器时间">{health.server_time}</Descriptions.Item>
          </Descriptions>
        ) : null}
      </Card>

      <Card title="Dashboard（骨架占位）">
        <Alert
          type="info"
          showIcon
          message="Phase 1 骨架已就绪"
          description="设备总数 / 状态分布 / 班组统计 / 最近流转等图表将在 Phase 5 实现。当前页面用于验证 前后端通信（此卡片数据来自后端 /api/health）。"
        />
      </Card>
    </Space>
  );
}

export default function App() {
  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Header style={{ display: 'flex', alignItems: 'center', background: '#001529' }}>
        <Typography.Title level={4} style={{ color: '#fff', margin: 0 }}>
          设备资产与流转管理系统
        </Typography.Title>
        <Badge status="processing" text={<span style={{ color: '#aaa' }}>Phase 1 骨架</span>} style={{ marginLeft: 24 }} />
      </Header>
      <Layout>
        <Sider width={200} theme="dark">
          <Menu
            theme="dark"
            mode="inline"
            defaultSelectedKeys={['dashboard']}
            items={MENU_ITEMS}
            style={{ height: '100%', borderRight: 0 }}
          />
        </Sider>
        <Content style={{ margin: 16 }}>
          <DashboardPage />
        </Content>
      </Layout>
    </Layout>
  );
}
