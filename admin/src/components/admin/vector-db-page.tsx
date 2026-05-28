'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { ReloadOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Empty, Modal, Space, Table, Tag, Typography, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import apiClient from '@/services/api/client';
import { KB_ADMIN_API } from '@/config/api';

type VectorCollection = {
  collection_name: string;
  active: boolean;
  status: string;
  health_status: string;
  index_status: string;
  index_version: string;
  schema_version?: string;
  rollback_collection?: string;
  contract_gaps?: string[];
};

type VectorOperation = {
  id: number;
  index_version: string;
  collection_name: string;
  operation: string;
  operation_reason?: string;
  created_at?: string;
};

const { Paragraph, Text, Title } = Typography;

export function VectorDbPage() {
  const [messageApi, contextHolder] = message.useMessage();
  const [collections, setCollections] = useState<VectorCollection[]>([]);
  const [operations, setOperations] = useState<VectorOperation[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<VectorCollection | null>(null);
  const [health, setHealth] = useState<Record<string, unknown> | null>(null);

  const loadData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const [collectionResp, operationResp] = await Promise.all([
        apiClient.get(KB_ADMIN_API.LIST_VECTOR_COLLECTIONS) as Promise<{ items?: VectorCollection[] }>,
        apiClient.get(KB_ADMIN_API.LIST_VECTOR_OPERATIONS, { params: { page: 1, page_size: 20 } }) as Promise<{ items?: VectorOperation[] }>,
      ]);
      setCollections(collectionResp.items ?? []);
      setOperations(operationResp.items ?? []);
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : '加载 Vector DB 页面失败');
      setCollections([]);
      setOperations([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  const inspectHealth = async (item: VectorCollection) => {
    setSelected(item);
    try {
      const data = (await apiClient.get(KB_ADMIN_API.GET_VECTOR_COLLECTION_HEALTH(item.index_version))) as Record<string, unknown>;
      setHealth(data);
    } catch (loadError) {
      messageApi.error(loadError instanceof Error ? loadError.message : '加载健康详情失败');
      setHealth(null);
    }
  };

  const columns = useMemo<ColumnsType<VectorCollection>>(
    () => [
      { title: 'Collection', dataIndex: 'collection_name', key: 'collection_name' },
      { title: 'Index Version', dataIndex: 'index_version', key: 'index_version', render: (value: string) => <Text code>{value}</Text> },
      { title: 'Active', dataIndex: 'active', key: 'active', render: (value: boolean) => (value ? <Tag color="success">active</Tag> : <Tag>inactive</Tag>) },
      { title: 'Health', dataIndex: 'health_status', key: 'health_status', render: (value: string) => <Tag>{value}</Tag> },
      { title: 'Status', dataIndex: 'status', key: 'status', render: (value: string) => <Tag>{value}</Tag> },
      {
        title: '操作',
        key: 'action',
        render: (_, record) => (
          <Button size="small" onClick={() => void inspectHealth(record)}>
            健康详情
          </Button>
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
            Vector DB
          </Title>
          <Paragraph type="secondary" style={{ marginBottom: 0 }}>
            查看 Collection 列表、健康状态和最近操作记录。
          </Paragraph>
        </div>
        <Button icon={<ReloadOutlined />} onClick={() => void loadData()} loading={loading}>
          刷新
        </Button>
      </div>

      {error ? <Alert type="error" showIcon message={error} /> : null}

      <Card title="Collections">
        {collections.length === 0 ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前没有 Collection 记录" />
        ) : (
          <Table rowKey="index_version" columns={columns} dataSource={collections} pagination={false} />
        )}
      </Card>

      <Card title="Recent Operations">
        <Table
          rowKey="id"
          pagination={false}
          dataSource={operations}
          columns={[
            { title: 'Operation', dataIndex: 'operation', key: 'operation' },
            { title: 'Collection', dataIndex: 'collection_name', key: 'collection_name' },
            { title: 'Index Version', dataIndex: 'index_version', key: 'index_version' },
            { title: 'Reason', dataIndex: 'operation_reason', key: 'operation_reason', render: (value?: string) => value || '-' },
          ]}
        />
      </Card>

      <Modal open={Boolean(selected)} title={selected?.collection_name} onCancel={() => { setSelected(null); setHealth(null); }} footer={null}>
        {health ? <pre className="overflow-auto rounded bg-slate-950 p-3 text-xs text-slate-100">{JSON.stringify(health, null, 2)}</pre> : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无健康详情" />}
      </Modal>
    </div>
  );
}
