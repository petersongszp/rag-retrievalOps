'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  CheckCircleOutlined,
  ReloadOutlined,
  RollbackOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import { Alert, Button, Card, Descriptions, Empty, Modal, Space, Table, Tag, Typography, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { KB_ADMIN_API } from '@/config/api';
import apiClient from '@/services/api/client';
import type { IndexHealthReport, IndexOperationLog, IndexRegistryRecord } from '@/types/kb';

const { Title, Paragraph, Text } = Typography;

function roleColor(role?: string) {
  switch (role) {
    case 'active':
      return 'success';
    case 'candidate':
      return 'processing';
    case 'rollback':
      return 'warning';
    case 'deprecated':
      return 'default';
    default:
      return 'blue';
  }
}

function statusColor(status?: string) {
  switch (status) {
    case 'ready':
    case 'switched':
      return 'success';
    case 'building':
      return 'processing';
    case 'failed':
      return 'error';
    case 'rolled_back':
      return 'warning';
    default:
      return 'default';
  }
}

function normalizeError(error: unknown, fallback: string): string {
  if (
    error &&
    typeof error === 'object' &&
    'message' in error &&
    typeof error.message === 'string'
  ) {
    return error.message;
  }
  return fallback;
}

export function VectorOpsPage() {
  const [messageApi, contextHolder] = message.useMessage();
  const [registry, setRegistry] = useState<IndexRegistryRecord[]>([]);
  const [operations, setOperations] = useState<IndexOperationLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<IndexRegistryRecord | null>(null);
  const [health, setHealth] = useState<IndexHealthReport | null>(null);
  const [healthLoading, setHealthLoading] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);

  const loadData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const [registryResp, opsResp] = await Promise.all([
        apiClient.get(KB_ADMIN_API.LIST_INDEX_REGISTRY) as Promise<{ items?: IndexRegistryRecord[] }>,
        apiClient.get(KB_ADMIN_API.LIST_INDEX_OPERATIONS) as Promise<{ items?: IndexOperationLog[] }>,
      ]);
      setRegistry(registryResp.items ?? []);
      setOperations(opsResp.items ?? []);
    } catch (loadError) {
      setError(normalizeError(loadError, '加载向量库运维数据失败'));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  const activeRecord = useMemo(
    () => registry.find((item) => item.collection_role === 'active') ?? null,
    [registry]
  );

  const registryColumns = useMemo<ColumnsType<IndexRegistryRecord>>(
    () => [
      {
        title: 'Index Version',
        dataIndex: 'index_version',
        key: 'index_version',
        render: (value: string) => <Text code>{value}</Text>,
      },
      {
        title: 'Collection',
        dataIndex: 'collection_name',
        key: 'collection_name',
      },
      {
        title: 'Role',
        dataIndex: 'collection_role',
        key: 'collection_role',
        render: (value?: string) => <Tag color={roleColor(value)}>{value || 'unknown'}</Tag>,
      },
      {
        title: 'Build',
        dataIndex: 'build_status',
        key: 'build_status',
        render: (value?: string) => <Tag color={statusColor(value)}>{value || 'unknown'}</Tag>,
      },
      {
        title: 'Metric',
        dataIndex: 'metric_type',
        key: 'metric_type',
      },
      {
        title: 'Action',
        key: 'action',
        render: (_, record) => (
          <Space>
            <Button size="small" onClick={() => void inspectHealth(record)}>
              Health
            </Button>
            <Button
              size="small"
              type="primary"
              disabled={record.collection_role === 'active'}
              onClick={() => void switchActive(record)}
            >
              Activate
            </Button>
          </Space>
        ),
      },
    ],
    []
  );

  const inspectHealth = useCallback(async (record: IndexRegistryRecord) => {
    setSelected(record);
    try {
      setHealthLoading(true);
      const report = (await apiClient.get(
        KB_ADMIN_API.GET_INDEX_HEALTH(record.index_version)
      )) as IndexHealthReport;
      setHealth(report);
    } catch (loadError) {
      messageApi.error(normalizeError(loadError, '加载健康检查失败'));
      setHealth(null);
    } finally {
      setHealthLoading(false);
    }
  }, [messageApi]);

  const switchActive = useCallback(
    async (record: IndexRegistryRecord) => {
      try {
        setActionLoading(true);
        await apiClient.post(KB_ADMIN_API.SWITCH_ACTIVE_INDEX(record.index_version), {
          operation_reason: 'admin activate',
        });
        messageApi.success(`已切换 active：${record.index_version}`);
        await loadData();
      } catch (switchError) {
        messageApi.error(normalizeError(switchError, '切换 active 失败'));
      } finally {
        setActionLoading(false);
      }
    },
    [loadData, messageApi]
  );

  const rollbackActive = useCallback(async () => {
    try {
      setActionLoading(true);
      await apiClient.post(KB_ADMIN_API.ROLLBACK_ACTIVE_INDEX, {
        operation_reason: 'admin rollback',
      });
      messageApi.success('已回滚到 rollback collection');
      await loadData();
    } catch (rollbackError) {
      messageApi.error(normalizeError(rollbackError, '回滚 active 失败'));
    } finally {
      setActionLoading(false);
    }
  }, [loadData, messageApi]);

  return (
    <div className="space-y-6">
      {contextHolder}
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <Title level={2} style={{ marginBottom: 8 }}>
            向量库运维
          </Title>
          <Paragraph type="secondary" style={{ marginBottom: 0 }}>
            查看 collection/index registry、health、切换记录和回滚轨迹。
          </Paragraph>
        </div>
        <Space>
          <Button icon={<RollbackOutlined />} onClick={() => void rollbackActive()} loading={actionLoading}>
            回滚 Active
          </Button>
          <Button icon={<ReloadOutlined />} onClick={() => void loadData()} loading={loading}>
            刷新
          </Button>
        </Space>
      </div>

      {error ? <Alert type="error" showIcon message={error} /> : null}

      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <Descriptions column={1} size="small" title="Active Collection">
            <Descriptions.Item label="Version">
              {activeRecord ? <Text code>{activeRecord.index_version}</Text> : '-'}
            </Descriptions.Item>
            <Descriptions.Item label="Collection">
              {activeRecord?.collection_name || '-'}
            </Descriptions.Item>
            <Descriptions.Item label="Metric">
              {activeRecord?.metric_type || '-'}
            </Descriptions.Item>
          </Descriptions>
        </Card>
        <Card>
          <Space direction="vertical" size={8}>
            <Text type="secondary">Registry Count</Text>
            <div className="text-2xl font-semibold">{registry.length}</div>
          </Space>
        </Card>
        <Card>
          <Space direction="vertical" size={8}>
            <Text type="secondary">Operation Count</Text>
            <div className="text-2xl font-semibold">{operations.length}</div>
          </Space>
        </Card>
      </div>

      <Card title="Index Registry">
        {registry.length === 0 ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前还没有 index registry 记录" />
        ) : (
          <Table
            rowKey="index_version"
            columns={registryColumns}
            dataSource={registry}
            pagination={false}
          />
        )}
      </Card>

      <Card title="Recent Operations">
        {operations.length === 0 ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前还没有 index operation 记录" />
        ) : (
          <Table
            rowKey="id"
            pagination={false}
            dataSource={operations}
            columns={[
              { title: 'Operation', dataIndex: 'operation', key: 'operation' },
              { title: 'Index Version', dataIndex: 'index_version', key: 'index_version', render: (value: string) => <Text code>{value}</Text> },
              { title: 'Collection', dataIndex: 'collection_name', key: 'collection_name' },
              { title: 'From', dataIndex: 'from_role', key: 'from_role', render: (value?: string) => value ? <Tag>{value}</Tag> : '-' },
              { title: 'To', dataIndex: 'to_role', key: 'to_role', render: (value?: string) => value ? <Tag color={roleColor(value)}>{value}</Tag> : '-' },
              { title: 'Reason', dataIndex: 'operation_reason', key: 'operation_reason' },
            ]}
          />
        )}
      </Card>

      <Modal
        title={selected ? `Health Check: ${selected.index_version}` : 'Health Check'}
        open={Boolean(selected)}
        onCancel={() => {
          setSelected(null);
          setHealth(null);
        }}
        footer={null}
      >
        {healthLoading ? (
          <div className="py-8 text-center text-slate-500">加载健康检查中...</div>
        ) : !health ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前没有健康检查结果" />
        ) : (
          <Descriptions column={1} bordered size="small">
            <Descriptions.Item label="Collection Exists">
              {health.collection_exists ? <Tag color="success" icon={<CheckCircleOutlined />}>Yes</Tag> : <Tag color="error" icon={<WarningOutlined />}>No</Tag>}
            </Descriptions.Item>
            <Descriptions.Item label="Dimension Match">
              {String(health.dimension_match)}
            </Descriptions.Item>
            <Descriptions.Item label="Metric Match">
              {String(health.metric_type_match)}
            </Descriptions.Item>
            <Descriptions.Item label="Load Healthy">
              {String(health.load_healthy)}
            </Descriptions.Item>
            <Descriptions.Item label="Query Smoke">
              {String(health.query_smoke_healthy)}
            </Descriptions.Item>
            <Descriptions.Item label="Message">
              {health.message || '-'}
            </Descriptions.Item>
          </Descriptions>
        )}
      </Modal>
    </div>
  );
}
