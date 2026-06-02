'use client';

import { useEffect, useMemo, useState } from 'react';
import { Alert, Card, Progress, Skeleton, Space, Statistic, Typography } from 'antd';
import { TENANT_API } from '@/config/api';
import apiClient from '@/services/api/client';
import { getErrorMessage } from '@/services/api/errors';
import type { TenantUsage } from '@/types/tenant';

const { Paragraph, Text, Title } = Typography;

function calcPercent(current: number, limit?: number): number | undefined {
  if (!limit || limit <= 0) {
    return undefined;
  }
  return Math.min(100, Math.round((current / limit) * 100));
}

function getProgressStatus(percent?: number): 'normal' | 'exception' | 'success' | 'active' {
  if (percent === undefined) {
    return 'normal';
  }
  if (percent >= 100) {
    return 'exception';
  }
  if (percent >= 80) {
    return 'active';
  }
  return 'normal';
}

export function TenantUsagePage() {
  const [usage, setUsage] = useState<TenantUsage | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const loadUsage = async () => {
      try {
        setLoading(true);
        setError(null);
        const result = (await apiClient.get(TENANT_API.USAGE)) as TenantUsage;
        setUsage(result);
      } catch (loadError) {
        setError(getErrorMessage(loadError, '后端用量接口待补齐或暂不可用'));
      } finally {
        setLoading(false);
      }
    };

    void loadUsage();
  }, []);

  const cards = useMemo(() => {
    if (!usage) {
      return [];
    }

    return [
      {
        key: 'api_calls_today',
        label: '今日 API 调用',
        value: usage.api_calls_today,
        limit: usage.limits.max_api_calls_per_day,
        suffix: 'calls',
      },
      {
        key: 'kb_count',
        label: '知识库数',
        value: usage.kb_count,
        limit: usage.limits.max_kb_count,
        suffix: 'KBs',
      },
      {
        key: 'doc_count',
        label: '文档数',
        value: usage.doc_count,
        limit: usage.limits.max_doc_count,
        suffix: 'docs',
      },
      {
        key: 'storage_mb',
        label: '存储用量',
        value: usage.storage_mb,
        limit: usage.limits.max_storage_mb,
        suffix: 'MB',
      },
    ];
  }, [usage]);

  return (
    <div className="space-y-6">
      <div>
        <Title level={2} style={{ marginBottom: 8 }}>
          租户用量
        </Title>
        <Paragraph type="secondary" style={{ marginBottom: 0 }}>
          用量页只展示后端返回的可信数据，不会由前端自行推算 tenant_id 或伪造配额值。
        </Paragraph>
      </div>

      {error ? <Alert type="warning" showIcon message="契约缺口" description={error} /> : null}

      {loading ? (
        <Card>
          <Skeleton active paragraph={{ rows: 8 }} />
        </Card>
      ) : usage ? (
        <div className="grid gap-4 md:grid-cols-2">
          {cards.map((item) => {
            const percent = calcPercent(item.value, item.limit);
            return (
              <Card key={item.key}>
                <Space direction="vertical" size={16} className="w-full">
                  <Statistic title={item.label} value={item.value} suffix={item.suffix} />
                  <div>
                    <div className="mb-2 flex items-center justify-between">
                      <Text type="secondary">当前 / 上限</Text>
                      <Text type="secondary">
                        {item.value} / {item.limit || '无限制'}
                      </Text>
                    </div>
                    <Progress
                      percent={percent}
                      status={getProgressStatus(percent)}
                      showInfo={percent !== undefined}
                    />
                  </div>
                  {percent !== undefined && percent >= 80 ? (
                    <Alert
                      type={percent >= 100 ? 'error' : 'warning'}
                      showIcon
                      message={percent >= 100 ? '已达到或超过上限' : '接近配额上限'}
                      description={
                        item.key === 'api_calls_today'
                          ? '如果收到 quota_exceeded，请等待每日配额重置后再继续请求。'
                          : '如果后续操作返回 quota_exceeded，请释放资源或调整使用量后再重试。'
                      }
                    />
                  ) : null}
                </Space>
              </Card>
            );
          })}
        </div>
      ) : (
        <Card>
          <Text type="secondary">暂无用量数据</Text>
        </Card>
      )}

      <Alert
        type="info"
        showIcon
        message="配额错误解释"
        description="当后端返回 quota_exceeded 或 rate_limited 时，前端会优先提示对应的配额项。若需要更细的 quota_type/current/limit/reset_at 字段，可在后端继续增强。"
      />
    </div>
  );
}
