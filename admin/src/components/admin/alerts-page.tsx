'use client';

import { useEffect, useMemo, useState } from 'react';
import { ReloadOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Empty, Space, Table, Tag, Typography, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import apiClient from '@/services/api/client';
import { KB_ADMIN_API } from '@/config/api';
import type { GovernanceAlert } from '@/types/kb';

const { Paragraph, Title } = Typography;

type AlertListResponse = {
  items?: GovernanceAlert[];
  total?: number;
  page?: number;
  page_size?: number;
};

export function AlertsPage() {
  const [messageApi, contextHolder] = message.useMessage();
  const [items, setItems] = useState<GovernanceAlert[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadData = async () => {
    try {
      setLoading(true);
      setError(null);
      const data = (await apiClient.get(KB_ADMIN_API.LIST_ALERTS, { params: { page: 1, page_size: 20 } })) as AlertListResponse;
      setItems(data.items ?? []);
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : '加载告警失败');
      setItems([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadData();
  }, []);

  const mutate = async (record: GovernanceAlert, action: 'ack' | 'resolve') => {
    try {
      const endpoint = action === 'ack' ? KB_ADMIN_API.ACK_ALERT(record.id) : KB_ADMIN_API.RESOLVE_ALERT(record.id);
      await apiClient.patch(endpoint, { reason: `admin ${action}` });
      messageApi.success(action === 'ack' ? '已确认告警' : '已解决告警');
      await loadData();
    } catch (mutationError) {
      messageApi.error(mutationError instanceof Error ? mutationError.message : '更新告警状态失败');
    }
  };

  const columns = useMemo<ColumnsType<GovernanceAlert>>(
    () => [
      { title: '标题', dataIndex: 'title', key: 'title' },
      { title: '类别', dataIndex: 'category', key: 'category', render: (value: string) => <Tag>{value}</Tag> },
      { title: '级别', dataIndex: 'severity', key: 'severity', render: (value: string) => <Tag color={value === 'high' ? 'error' : value === 'medium' ? 'warning' : 'default'}>{value === 'high' ? '高' : value === 'medium' ? '中' : '低'}</Tag> },
      { title: '状态', dataIndex: 'status', key: 'status', render: (value: string) => <Tag color={value === 'resolved' ? 'success' : value === 'acknowledged' ? 'processing' : 'error'}>{value === 'resolved' ? '已解决' : value === 'acknowledged' ? '已确认' : '待处理'}</Tag> },
      { title: '指标', dataIndex: 'metric_key', key: 'metric_key', render: (value?: string) => value || '-' },
      {
        title: '操作',
        key: 'action',
        render: (_, record) => (
          <Space>
            <Button size="small" onClick={() => void mutate(record, 'ack')} disabled={record.status !== 'open'}>
              确认
            </Button>
            <Button size="small" onClick={() => void mutate(record, 'resolve')} disabled={record.status === 'resolved'}>
              解决
            </Button>
          </Space>
        ),
      },
    ],
    []
  );

  return (
    <div className="space-y-6">
      {contextHolder}
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <Title level={2} style={{ marginBottom: 8 }}>
            告警中心
          </Title>
          <Paragraph type="secondary" style={{ marginBottom: 0 }}>
            汇总质量、成本、容量与审计相关治理告警，并支持确认与解决。
          </Paragraph>
        </div>
        <Button icon={<ReloadOutlined />} onClick={() => void loadData()} loading={loading}>
          刷新
        </Button>
      </div>

      {error ? <Alert type="error" showIcon message={error} /> : null}

      <Card title="治理告警列表">
        {items.length === 0 ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description="当前没有待处理的治理告警，可以继续关注质量、成本与容量波动。"
          >
            <Button icon={<ReloadOutlined />} onClick={() => void loadData()} loading={loading}>
              重新加载
            </Button>
          </Empty>
        ) : (
          <Table rowKey="id" columns={columns} dataSource={items} pagination={false} />
        )}
      </Card>
    </div>
  );
}
