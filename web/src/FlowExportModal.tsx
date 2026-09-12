import { useState } from 'react';
import { Button, Checkbox, Modal, Radio, Select, Space, Typography, message } from 'antd';
import { downloadFile } from './download';
import { STATUS_FILTER_OPTIONS } from './status';

interface Team { id: number; name: string }
interface Borrower { id: number; name: string }

export interface FlowExportModalProps {
  open: boolean;
  onClose: () => void;
  teams: Team[];
  borrowers: Borrower[];
}

type RangeValue = 'all' | 'current' | 'history';

// 导出设备流转情况 Modal：筛选条件 + 数据范围 + 含统计汇总。
export default function FlowExportModal({ open, onClose, teams, borrowers }: FlowExportModalProps) {
  const [teamId, setTeamId] = useState<number | undefined>();
  const [status, setStatus] = useState<string | undefined>();
  const [borrowerId, setBorrowerId] = useState<number | undefined>();
  const [range, setRange] = useState<RangeValue>('all');
  const [includeSummary, setIncludeSummary] = useState(true);
  const [exporting, setExporting] = useState(false);

  const reset = () => {
    setTeamId(undefined);
    setStatus(undefined);
    setBorrowerId(undefined);
    setRange('all');
    setIncludeSummary(true);
  };

  const doExport = async () => {
    setExporting(true);
    try {
      const params = new URLSearchParams();
      if (teamId) params.set('team_id', String(teamId));
      if (status) params.set('status', status);
      if (borrowerId) params.set('borrower_id', String(borrowerId));
      params.set('include_current', String(range === 'all' || range === 'current'));
      params.set('include_history', String(range === 'all' || range === 'history'));
      params.set('include_summary', String(includeSummary));
      await downloadFile(`/api/export/flow?${params.toString()}`, '设备流转情况.xlsx');
      message.success('导出成功');
      onClose();
      reset();
    } catch (e) {
      message.error(e instanceof Error ? e.message : '导出失败，请检查系统日志');
    } finally {
      setExporting(false);
    }
  };

  return (
    <Modal
      title="导出设备流转情况"
      open={open}
      onCancel={() => { onClose(); reset(); }}
      footer={[
        <Button key="cancel" onClick={() => { onClose(); reset(); }}>取消</Button>,
        <Button key="ok" type="primary" loading={exporting} disabled={exporting} onClick={() => void doExport()}>
          {exporting ? '正在导出...' : '导出 Excel'}
        </Button>,
      ]}
    >
      <Space direction="vertical" style={{ width: '100%' }} size="middle">
        <Space wrap>
          <div style={{ width: 160 }}>
            <Typography.Text>班组：</Typography.Text>
            <Select allowClear placeholder="全部" style={{ width: '100%' }} value={teamId}
              options={teams.map((t) => ({ value: t.id, label: t.name }))}
              onChange={(v) => setTeamId(v)} />
          </div>
          <div style={{ width: 140 }}>
            <Typography.Text>状态：</Typography.Text>
            <Select allowClear placeholder="全部" style={{ width: '100%' }} value={status}
              options={STATUS_FILTER_OPTIONS} onChange={(v) => setStatus(v)} />
          </div>
          <div style={{ width: 180 }}>
            <Typography.Text>外借公司：</Typography.Text>
            <Select allowClear placeholder="全部" style={{ width: '100%' }} value={borrowerId}
              options={borrowers.map((b) => ({ value: b.id, label: b.name }))}
              onChange={(v) => setBorrowerId(v)} />
          </div>
        </Space>

        <div>
          <Typography.Text strong>数据范围：</Typography.Text>
          <Radio.Group value={range} onChange={(e) => setRange(e.target.value as RangeValue)} style={{ marginLeft: 8 }}>
            <Radio value="all">当前 + 历史</Radio>
            <Radio value="current">当前设备</Radio>
            <Radio value="history">流转历史</Radio>
          </Radio.Group>
        </div>

        <Checkbox checked={includeSummary} onChange={(e) => setIncludeSummary(e.target.checked)}>
          包含统计汇总
        </Checkbox>
      </Space>
    </Modal>
  );
}
