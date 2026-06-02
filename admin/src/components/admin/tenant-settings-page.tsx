'use client';

import { useEffect, useState } from 'react';
import { Alert, Card, Descriptions, Skeleton, Space, Tag, Typography } from 'antd';
import { TENANT_API } from '@/config/api';
import apiClient from '@/services/api/client';
import { getErrorMessage } from '@/services/api/errors';
import { canViewTenantSettings } from '@/services/auth/permissions';
import { useAuth } from '@/services/auth/store';
import type { TenantDetail } from '@/types/tenant';
import { ForbiddenState } from './forbidden-state';

const { Paragraph, Text, Title } = Typography;

function getStatusColor(status?: string): string {
  if (status === 'active') {
    return 'success';
  }
  return status ? 'warning' : 'default';
}

export function TenantSettingsPage() {
  const { user } = useAuth();
  const [tenant, setTenant] = useState<TenantDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const loadTenant = async () => {
      try {
        setLoading(true);
        setError(null);
        const result = (await apiClient.get(TENANT_API.DETAIL)) as TenantDetail;
        setTenant(result);
      } catch (loadError) {
        setError(getErrorMessage(loadError, '后端租户详情接口待补齐或暂不可用'));
      } finally {
        setLoading(false);
      }
    };

    void loadTenant();
  }, []);

  if (!canViewTenantSettings(user?.role)) {
    return (
      <ForbiddenState
        title="当前角色无权查看租户设置"
        description="租户设置只对 owner 角色开放，避免非 owner 误读或误操作租户级配置。"
      />
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <Title level={2} style={{ marginBottom: 8 }}>
          租户设置
        </Title>
        <Paragraph type="secondary" style={{ marginBottom: 0 }}>
          这里展示当前租户的基础身份、套餐占位和资源上限。Phase 4 先以只读为主，
          后续商业化能力会在 Phase 5 继续扩展。
        </Paragraph>
      </div>

      {error ? <Alert type="warning" showIcon message="契约缺口" description={error} /> : null}

      <Card>
        {loading ? (
          <Skeleton active paragraph={{ rows: 6 }} />
        ) : tenant ? (
          <Space direction="vertical" size={20} className="w-full">
            <Descriptions column={1} bordered>
              <Descriptions.Item label="租户 ID">{tenant.tenant_id}</Descriptions.Item>
              <Descriptions.Item label="名称">{tenant.name}</Descriptions.Item>
              <Descriptions.Item label="Slug">{tenant.slug || '-'}</Descriptions.Item>
              <Descriptions.Item label="套餐">
                <Tag color="geekblue">{tenant.plan || 'free'}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="状态">
                <Tag color={getStatusColor(tenant.status)}>{tenant.status || 'unknown'}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="创建时间">{tenant.created_at || '-'}</Descriptions.Item>
              <Descriptions.Item label="更新时间">{tenant.updated_at || '-'}</Descriptions.Item>
            </Descriptions>

            <Card size="small" title="当前资源上限">
              <Descriptions column={2} size="small">
                <Descriptions.Item label="知识库上限">{tenant.max_kb_count}</Descriptions.Item>
                <Descriptions.Item label="文档上限">{tenant.max_doc_count}</Descriptions.Item>
                <Descriptions.Item label="存储上限">{tenant.max_storage_mb} MB</Descriptions.Item>
                <Descriptions.Item label="每日 API 调用">{tenant.max_api_calls_per_day}</Descriptions.Item>
              </Descriptions>
            </Card>

            <Alert
              type="info"
              showIcon
              message="当前版本说明"
              description="Phase 4 暂不开放 tenant_id、slug、plan、status 的前端修改。若后端后续开放 PUT /v1/tenant，再在这里补可编辑能力。"
            />
          </Space>
        ) : (
          <Text type="secondary">暂无租户数据</Text>
        )}
      </Card>
    </div>
  );
}
