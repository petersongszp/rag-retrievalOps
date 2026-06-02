'use client';

import { useEffect, useMemo, useState } from 'react';
import {
  Alert,
  Button,
  Card,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import {
  KeyOutlined,
  PlusOutlined,
  ReloadOutlined,
  RetweetOutlined,
  StopOutlined,
} from '@ant-design/icons';
import { API_KEY_API } from '@/config/api';
import apiClient from '@/services/api/client';
import { getErrorMessage } from '@/services/api/errors';
import { canManageAPIKey } from '@/services/auth/permissions';
import { useAuth } from '@/services/auth/store';
import type {
  APIKeyRecord,
  CreateAPIKeyPayload,
  CreateAPIKeyResponse,
  RotateAPIKeyResponse,
} from '@/types/api-key';
import { ForbiddenState } from './forbidden-state';

const { Paragraph, Text, Title } = Typography;

const permissionOptions = [
  { label: 'retrieve', value: 'retrieve' },
  { label: 'kb:read', value: 'kb:read' },
  { label: 'log:read', value: 'log:read' },
];

function getStatusColor(status: string): string {
  if (status === 'active') {
    return 'success';
  }
  if (status === 'revoked') {
    return 'default';
  }
  return 'processing';
}

function buildRetrieveCurlCommand(key: string): string {
  return [
    "curl -X POST '$RAG_BASE_URL/v1/retrieve' \\",
    "  -H 'Content-Type: application/json' \\",
    `  -H 'Authorization: Bearer ${key}' \\`,
    "  -d '{",
    '    "query": "知识库里关于 Go 并发的内容是什么？",',
    '    "kb_ids": [1],',
    '    "top_k": 5',
    "  }'",
  ].join('\n');
}

export function APIKeysPage() {
  const { user } = useAuth();
  const canManage = canManageAPIKey(user?.role);
  const [items, setItems] = useState<APIKeyRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [createSubmitting, setCreateSubmitting] = useState(false);
  const [editingItem, setEditingItem] = useState<APIKeyRecord | null>(null);
  const [updateSubmitting, setUpdateSubmitting] = useState(false);
  const [revealedKey, setRevealedKey] = useState<string | null>(null);
  const [rotateEnabled, setRotateEnabled] = useState(true);
  const [rotatingId, setRotatingId] = useState<number | null>(null);
  const [statusFilter, setStatusFilter] = useState<string>('all');
  const [appFilter, setAppFilter] = useState('');
  const [createForm] = Form.useForm<CreateAPIKeyPayload>();
  const [editForm] = Form.useForm<Pick<CreateAPIKeyPayload, 'name' | 'permissions'>>();

  const loadKeys = async () => {
    try {
      setLoading(true);
      setError(null);
      const result = (await apiClient.get(API_KEY_API.LIST)) as { items?: APIKeyRecord[] };
      setItems(result.items ?? []);
    } catch (loadError) {
      setError(getErrorMessage(loadError, '加载 API Key 列表失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadKeys();
  }, []);

  const filteredItems = useMemo(() => {
    return items.filter((item) => {
      const statusMatched = statusFilter === 'all' ? true : item.status === statusFilter;
      const appMatched = appFilter.trim()
        ? item.app_id.toLowerCase().includes(appFilter.trim().toLowerCase())
        : true;
      return statusMatched && appMatched;
    });
  }, [appFilter, items, statusFilter]);

  const handleCreate = async (values: CreateAPIKeyPayload) => {
    try {
      setCreateSubmitting(true);
      const result = (await apiClient.post(API_KEY_API.CREATE, values)) as CreateAPIKeyResponse;
      setCreateOpen(false);
      createForm.resetFields();
      setRevealedKey(result.key);
      message.success('API Key 创建成功');
      await loadKeys();
    } catch (createError) {
      message.error(getErrorMessage(createError, '创建 API Key 失败'));
    } finally {
      setCreateSubmitting(false);
    }
  };

  const handleUpdate = async () => {
    if (!editingItem) {
      return;
    }

    try {
      setUpdateSubmitting(true);
      const values = await editForm.validateFields();
      await apiClient.put(API_KEY_API.UPDATE(editingItem.id), values);
      message.success('API Key 已更新');
      setEditingItem(null);
      editForm.resetFields();
      await loadKeys();
    } catch (updateError) {
      if (updateError && typeof updateError === 'object' && 'errorFields' in updateError) {
        return;
      }
      message.error(getErrorMessage(updateError, '更新 API Key 失败'));
    } finally {
      setUpdateSubmitting(false);
    }
  };

  const handleRevoke = async (item: APIKeyRecord) => {
    try {
      await apiClient.delete(API_KEY_API.DELETE(item.id));
      message.success('API Key 已吊销');
      await loadKeys();
    } catch (revokeError) {
      message.error(getErrorMessage(revokeError, '吊销 API Key 失败'));
    }
  };

  const handleRotate = async (item: APIKeyRecord) => {
    try {
      setRotatingId(item.id);
      const result = (await apiClient.post(API_KEY_API.ROTATE(item.id), {})) as RotateAPIKeyResponse;
      setRevealedKey(result.key);
      setRotateEnabled(true);
      message.success('API Key 已轮换，请立即复制新 Key');
      await loadKeys();
    } catch (rotateError) {
      setRotateEnabled(false);
      message.warning(getErrorMessage(rotateError, '当前环境暂未启用 Key 轮换'));
    } finally {
      setRotatingId(null);
    }
  };

  const appOptions = useMemo(
    () =>
      Array.from(new Set(items.map((item) => item.app_id)))
        .filter(Boolean)
        .map((value) => ({ label: value, value })),
    [items]
  );

  if (!canManage) {
    return (
      <ForbiddenState
        title="当前角色无权管理 API Key"
        description="viewer 角色不会显示或开放 API Key 的创建、轮换、吊销入口。"
      />
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <Title level={2} style={{ marginBottom: 8 }}>
            API Keys
          </Title>
          <Paragraph type="secondary" style={{ marginBottom: 0 }}>
            为 Agent 或 SDK 创建服务端访问 Key。完整明文只会在创建或轮换后展示一次，
            关闭弹窗后无法再次查看。
          </Paragraph>
        </div>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => void loadKeys()}>
            刷新
          </Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
            创建 API Key
          </Button>
        </Space>
      </div>

      {error ? <Alert type="error" showIcon message={error} /> : null}

      <Card>
        <div className="mb-4 flex flex-wrap gap-3">
          <Select
            value={statusFilter}
            onChange={setStatusFilter}
            options={[
              { label: '全部状态', value: 'all' },
              { label: 'active', value: 'active' },
              { label: 'revoked', value: 'revoked' },
            ]}
            style={{ minWidth: 140 }}
          />
          <Select
            allowClear
            placeholder="按 app_id 过滤"
            value={appFilter || undefined}
            onChange={(value) => setAppFilter(value ?? '')}
            options={appOptions}
            style={{ minWidth: 220 }}
          />
        </div>

        <Table<APIKeyRecord>
          rowKey="id"
          loading={loading}
          dataSource={filteredItems}
          locale={{
            emptyText: loading ? '正在加载...' : <Empty description="暂无 API Key" />,
          }}
          pagination={{ pageSize: 8 }}
          columns={[
            {
              title: '名称',
              dataIndex: 'name',
              key: 'name',
              render: (_, item) => (
                <Space direction="vertical" size={2}>
                  <Text strong>{item.name}</Text>
                  <Text type="secondary">{item.app_id}</Text>
                </Space>
              ),
            },
            {
              title: 'Key Prefix',
              dataIndex: 'key_prefix',
              key: 'key_prefix',
              render: (value: string) => <Text code>{value}</Text>,
            },
            {
              title: '权限',
              dataIndex: 'permissions',
              key: 'permissions',
              render: (permissions: string[]) =>
                permissions?.length ? (
                  <Space wrap>
                    {permissions.map((permission) => (
                      <Tag key={permission}>{permission}</Tag>
                    ))}
                  </Space>
                ) : (
                  <Text type="secondary">未设置</Text>
                ),
            },
            {
              title: '状态',
              dataIndex: 'status',
              key: 'status',
              render: (value: string) => <Tag color={getStatusColor(value)}>{value}</Tag>,
            },
            {
              title: '最近使用',
              dataIndex: 'last_used_at',
              key: 'last_used_at',
              render: (value: string) => value || '-',
            },
            {
              title: '过期时间',
              dataIndex: 'expires_at',
              key: 'expires_at',
              render: (value: string) => value || '永不过期',
            },
            {
              title: '操作',
              key: 'actions',
              render: (_, item) => (
                <Space wrap>
                  <Button
                    size="small"
                    onClick={() => {
                      setEditingItem(item);
                      editForm.setFieldsValue({
                        name: item.name,
                        permissions: item.permissions,
                      });
                    }}
                    disabled={item.status === 'revoked'}
                  >
                    编辑
                  </Button>
                  <Button
                    size="small"
                    icon={<RetweetOutlined />}
                    loading={rotatingId === item.id}
                    disabled={item.status === 'revoked' || !rotateEnabled}
                    onClick={() => void handleRotate(item)}
                  >
                    轮换
                  </Button>
                  <Popconfirm
                    title="确认吊销这个 API Key 吗？"
                    description="吊销后旧 Key 会立即失效，/v1/retrieve 将不能继续使用它。"
                    okText="确认吊销"
                    cancelText="取消"
                    okButtonProps={{ danger: true }}
                    onConfirm={() => void handleRevoke(item)}
                  >
                    <Button size="small" danger icon={<StopOutlined />} disabled={item.status === 'revoked'}>
                      吊销
                    </Button>
                  </Popconfirm>
                </Space>
              ),
            },
          ]}
        />
      </Card>

      <Modal
        title="创建 API Key"
        open={createOpen}
        okText="创建"
        cancelText="取消"
        confirmLoading={createSubmitting}
        onCancel={() => {
          setCreateOpen(false);
          createForm.resetFields();
        }}
        onOk={() => {
          void createForm.validateFields().then((values) => handleCreate(values));
        }}
      >
        <Form<CreateAPIKeyPayload>
          form={createForm}
          layout="vertical"
          initialValues={{ permissions: ['retrieve'], expires_in: 0 }}
        >
          <Form.Item label="名称" name="name" rules={[{ required: true, message: '请输入 Key 名称' }]}>
            <Input placeholder="例如：production-agent" />
          </Form.Item>

          <Form.Item label="应用 ID" name="app_id" rules={[{ required: true, message: '请输入 app_id' }]}>
            <Input placeholder="例如：support-bot" />
          </Form.Item>

          <Form.Item label="权限" name="permissions">
            <Select mode="multiple" options={permissionOptions} placeholder="选择权限范围" />
          </Form.Item>

          <Form.Item
            label="过期时间（秒）"
            name="expires_in"
            extra="填 0 表示不过期；当前后端契约按 expires_in 秒数消费。"
          >
            <InputNumber min={0} className="w-full" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="一次性查看完整 API Key"
        open={Boolean(revealedKey)}
        okText="我已复制并妥善保存"
        cancelText="关闭"
        onOk={() => setRevealedKey(null)}
        onCancel={() => setRevealedKey(null)}
      >
        <Space direction="vertical" size={16} className="w-full">
          <Alert
            type="warning"
            showIcon
            message="关闭后将无法再次查看完整 Key"
            description="完整明文不会写入本地存储、URL 或全局状态，请现在就复制到安全位置。"
          />

          <div className="rounded-2xl border border-slate-200 bg-slate-50 p-4">
            <Space direction="vertical" size={8} className="w-full">
              <Text strong>完整 Key</Text>
              <Text code copyable={{ text: revealedKey || '' }}>
                {revealedKey}
              </Text>
            </Space>
          </div>

          <div className="rounded-2xl border border-slate-200 bg-slate-950 p-4">
            <Space direction="vertical" size={8} className="w-full">
              <Space>
                <KeyOutlined className="text-slate-300" />
                <Text className="text-slate-200">可直接复制的 cURL 示例</Text>
              </Space>
              <pre className="overflow-x-auto whitespace-pre-wrap text-xs leading-6 text-slate-100">
                {buildRetrieveCurlCommand(revealedKey || '')}
              </pre>
            </Space>
          </div>

          <Alert
            type="info"
            showIcon
            message="使用边界"
            description="JWT 用于 Admin UI；这个 API Key 应只由 Agent 后端或服务端 SDK 持有，不能直接交给终端用户。"
          />
        </Space>
      </Modal>

      <Modal
        title={editingItem ? `编辑 API Key：${editingItem.name}` : '编辑 API Key'}
        open={Boolean(editingItem)}
        okText="保存"
        cancelText="取消"
        confirmLoading={updateSubmitting}
        onCancel={() => {
          setEditingItem(null);
          editForm.resetFields();
        }}
        onOk={() => {
          void handleUpdate();
        }}
      >
        <Form form={editForm} layout="vertical">
          <Form.Item label="名称" name="name" rules={[{ required: true, message: '请输入 Key 名称' }]}>
            <Input />
          </Form.Item>
          <Form.Item label="权限" name="permissions">
            <Select mode="multiple" options={permissionOptions} />
          </Form.Item>
          {editingItem ? (
            <Alert
              type="info"
              showIcon
              message="不可修改字段"
              description={`key_prefix=${editingItem.key_prefix}，tenant_id 与明文 key 不允许在这里被修改。`}
            />
          ) : null}
        </Form>
      </Modal>
    </div>
  );
}
