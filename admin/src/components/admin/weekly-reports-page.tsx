'use client';

import { useEffect, useMemo, useState } from 'react';
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Drawer, Empty, Space, Table, Typography, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import apiClient from '@/services/api/client';
import { KB_ADMIN_API } from '@/config/api';
import type { WeeklyReport } from '@/types/kb';

const { Paragraph, Title } = Typography;

type WeeklyReportRecord = {
  id: string;
  generated_at: string;
  window_start: string;
  window_end: string;
  report: WeeklyReport;
};

type WeeklyReportListResponse = {
  items?: WeeklyReportRecord[];
  total?: number;
  page?: number;
  page_size?: number;
};

export function WeeklyReportsPage() {
  const [messageApi, contextHolder] = message.useMessage();
  const [items, setItems] = useState<WeeklyReportRecord[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [detail, setDetail] = useState<WeeklyReportRecord | null>(null);

  const loadData = async () => {
    try {
      setLoading(true);
      setError(null);
      const data = (await apiClient.get(KB_ADMIN_API.LIST_WEEKLY_REPORTS, {
        params: { page: 1, page_size: 20 },
      })) as WeeklyReportListResponse;
      setItems(data.items ?? []);
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : '加载周报失败');
      setItems([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadData();
  }, []);

  const createReport = async () => {
    try {
      await apiClient.post(KB_ADMIN_API.LIST_WEEKLY_REPORTS, {});
      messageApi.success('已生成周报');
      await loadData();
    } catch (createError) {
      messageApi.error(createError instanceof Error ? createError.message : '生成周报失败');
    }
  };

  const columns = useMemo<ColumnsType<WeeklyReportRecord>>(
    () => [
      { title: 'Report ID', dataIndex: 'id', key: 'id' },
      { title: 'Generated', dataIndex: 'generated_at', key: 'generated_at', render: (value: string) => new Date(value).toLocaleString() },
      { title: 'Window', key: 'window', render: (_, record) => `${new Date(record.window_start).toLocaleDateString()} - ${new Date(record.window_end).toLocaleDateString()}` },
      { title: 'Risks', key: 'risks', render: (_, record) => record.report.risks?.length ?? 0 },
      { title: '操作', key: 'action', render: (_, record) => <Button size="small" onClick={() => setDetail(record)}>详情</Button> },
    ],
    []
  );

  return (
    <div className="space-y-6">
      {contextHolder}
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <Title level={2} style={{ marginBottom: 8 }}>
            周报
          </Title>
          <Paragraph type="secondary" style={{ marginBottom: 0 }}>
            查看自动化治理周报，跟踪风险摘要、质量趋势和下一步动作。
          </Paragraph>
        </div>
        <Space>
          <Button icon={<PlusOutlined />} type="primary" onClick={() => void createReport()}>
            生成周报
          </Button>
          <Button icon={<ReloadOutlined />} onClick={() => void loadData()} loading={loading}>
            刷新
          </Button>
        </Space>
      </div>

      {error ? <Alert type="error" showIcon message={error} /> : null}

      <Card title="周报列表">
        {items.length === 0 ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description="当前没有周报记录。可以先生成一份周报，汇总本周风险、质量变化和后续动作。"
          >
            <Button icon={<PlusOutlined />} type="primary" onClick={() => void createReport()}>
              生成周报
            </Button>
          </Empty>
        ) : (
          <Table rowKey="id" columns={columns} dataSource={items} pagination={false} />
        )}
      </Card>

      <Drawer title="周报详情" open={Boolean(detail)} onClose={() => setDetail(null)} width={560}>
        {detail ? (
          <div className="space-y-4">
            <div><strong>风险</strong><div>{detail.report.risks?.join(' / ') || '-'}</div></div>
            <div><strong>后续动作</strong><div>{detail.report.next_actions?.join(' / ') || '-'}</div></div>
            <div><strong>质量摘要</strong><pre className="overflow-auto rounded bg-slate-100 p-3 text-xs">{JSON.stringify(detail.report.quality_summary, null, 2)}</pre></div>
          </div>
        ) : null}
      </Drawer>
    </div>
  );
}
