import { useCallback, useEffect, useState } from 'react';
import { Alert, Button, Card, Descriptions, Space, Spin, Tag } from 'antd';
import { getHealth, type Health } from './api';

export default function DashboardPage() {
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
          message="Phase 2 骨架已就绪"
          description="设备总数 / 状态分布 / 班组统计 / 最近流转等图表将在 Phase 5 实现。可在左侧进入“数据导入”完成首次 Excel 初始化。"
        />
      </Card>
    </Space>
  );
}
