'use client';

import { useEffect, useMemo, useState } from 'react';
import { ReloadOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Drawer, Empty, Space, Table, Tag, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import apiClient from '@/services/api/client';
import { KB_ADMIN_API } from '@/config/api';
import type { AuditEventDetail } from '@/types/kb';

const { Paragraph, Text, Title } = Typography;

type AuditListResponse = {
  items?: AuditEventDetail[];
  total?: number;
  page?: number;
  page_size?: number;
};

export function AuditPage() {
  const [items, setItems] = useState<AuditEventDetail[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [detail, setDetail] = useState<AuditEventDetail | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);

  const loadData = async () => {
    try {
      setLoading(true);
      setError(null);
      const data = (await apiClient.get(KB_ADMIN_API.LIST_AUDIT_EVENTS, {
        params: { page: 1, page_size: 20 },
      })) as AuditListResponse;
      setItems(data.items ?? []);
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : '加载审计事件失败');
      setItems([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadData();
  }, []);

  const openDetail = async (item: AuditEventDetail) => {
    try {
      const data = (await apiClient.get(`${KB_ADMIN_API.LIST_AUDIT_EVENTS}/${item.id}`)) as AuditEventDetail;
      setDetail(data);
      setDetailOpen(true);
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : '加载审计详情失败');
    }
  };

  const columns = useMemo<ColumnsType<AuditEventDetail>>(
    () => [
      { title: '操作类型', dataIndex: 'action', key: 'action', render: (value: string) => <Tag>{value}</Tag> },
      { title: '资源类型', dataIndex: 'resource_type', key: 'resource_type', render: (value: string) => value || '-' },
      { title: '资源 ID', dataIndex: 'resource_id', key: 'resource_id', render: (value?: string) => value ? <Text code>{value}</Text> : '-' },
      { title: '原因', dataIndex: 'reason', key: 'reason', render: (value?: string) => value || '-' },
      { title: '发生时间', dataIndex: 'created_at', key: 'created_at', render: (value?: string) => value ? new Date(value).toLocaleString() : '-' },
      { title: '操作', key: 'operation', render: (_, record) => <Button size="small" onClick={() => void openDetail(record)}>详情</Button> },
    ],
    []
  );

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <Title level={2} style={{ marginBottom: 8 }}>
            审计中心
          </Title>
          <Paragraph type="secondary" style={{ marginBottom: 0 }}>
            查询关键治理事件，查看脱敏后的变更前后摘要，并跟踪字段缺口。
          </Paragraph>
        </div>
        <Button icon={<ReloadOutlined />} onClick={() => void loadData()} loading={loading}>
          刷新
        </Button>
      </div>

      {error ? <Alert type="error" showIcon message={error} /> : null}

      <Card title="审计事件">
        {items.length === 0 ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description="当前没有审计事件。待策略变更、告警处理或人工操作发生后，会在这里沉淀记录。"
          >
            <Button icon={<ReloadOutlined />} onClick={() => void loadData()} loading={loading}>
              重新加载
            </Button>
          </Empty>
        ) : (
          <Table rowKey="id" columns={columns} dataSource={items} pagination={false} />
        )}
      </Card>

      <Drawer title="审计详情" open={detailOpen} onClose={() => setDetailOpen(false)} width={520}>
        {detail ? (
          <div className="space-y-4">
            <div><Text type="secondary">操作类型</Text><div>{detail.action}</div></div>
            <div><Text type="secondary">变更前</Text><pre className="overflow-auto rounded bg-slate-100 p-3 text-xs">{detail.before || '-'}</pre></div>
            <div><Text type="secondary">变更后</Text><pre className="overflow-auto rounded bg-slate-100 p-3 text-xs">{detail.after || '-'}</pre></div>
            <div><Text type="secondary">字段缺口</Text><div>{detail.contract_gaps?.join(', ') || '-'}</div></div>
          </div>
        ) : null}
      </Drawer>
    </div>
  );
}
