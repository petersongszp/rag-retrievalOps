'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import {
  SyncOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import { Alert, Card, Col, Empty, Row, Statistic, Typography } from 'antd';
import { KB_ADMIN_API } from '@/config/api';
import { useKnowledgeBaseContext } from './knowledge-base-provider';

const { Paragraph, Text, Title } = Typography;

interface DashboardStats {
  kb_count: number;
  document_count: number;
  processing_job_count: number;
  failed_job_count: number;
}

const PHASE1_METRICS = [
  { title: '入库成功率趋势', api: '/api/admin/kb/metrics/ingest-success-rate' },
  { title: '检索 P50/P95 趋势', api: '/api/admin/kb/metrics/retrieve-latency' },
  { title: '空结果率趋势', api: '/api/admin/kb/metrics/empty-result-rate' },
  { title: '失败类型 TopN', api: '/api/admin/kb/metrics/failure-topn' },
];

export function DashboardPage() {
  const router = useRouter();
  const { selectedBase, isPermissionDenied } = useKnowledgeBaseContext();
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [statsError, setStatsError] = useState<string | null>(null);
  const [statsPermissionDenied, setStatsPermissionDenied] = useState(false);

  useEffect(() => {
    setLoading(true);
    setStatsError(null);
    setStatsPermissionDenied(false);

    fetch(KB_ADMIN_API.DASHBOARD_STATS)
      .then(async (r) => {
        if (r.status === 403) {
          setStatsPermissionDenied(true);
          setStatsError('权限不足，无法加载概览数据（403）');
          return;
        }
        if (!r.ok) {
          setStatsError(`加载概览数据失败（HTTP ${r.status}）`);
          return;
        }
        const data = await r.json() as { data?: DashboardStats };
        if (data?.data) {
          setStats(data.data);
        } else {
          setStatsError('概览数据结构异常，请检查后端响应格式');
        }
      })
      .catch(() => {
        setStatsError('网络错误，无法连接到后端服务');
      })
      .finally(() => setLoading(false));
  }, []);

  const jobsHref = selectedBase
    ? `/knowledge-bases/${selectedBase.id}`
    : '/knowledge-bases';

  return (
    <div className="space-y-6">
      <div>
        <Title level={2} style={{ marginBottom: 8 }}>
          概览
        </Title>
        <Paragraph type="secondary" style={{ marginBottom: 0 }}>
          当前知识库：{selectedBase?.name ?? '未选择'}
        </Paragraph>
      </div>

      {isPermissionDenied && (
        <Alert
          type="error"
          showIcon
          message="权限不足"
          description="当前账号无权访问知识库列表（403）。请联系管理员确认权限配置。"
        />
      )}

      {statsError && !statsPermissionDenied && (
        <Alert type="error" showIcon message={statsError} />
      )}

      {statsPermissionDenied && (
        <Alert
          type="error"
          showIcon
          message="权限不足"
          description="当前账号无权访问概览统计数据（403）。请联系管理员确认权限配置。"
        />
      )}

      {/* 最小状态卡片 */}
      <Row gutter={[16, 16]}>
        <Col xs={12} md={6}>
          <Card
            hoverable
            style={{ cursor: 'pointer' }}
            onClick={() => router.push('/knowledge-bases')}
          >
            <Statistic
              title="知识库"
              value={stats?.kb_count ?? 0}
              loading={loading}
            />
            <Text type="secondary" style={{ fontSize: 12 }}>
              点击管理知识库
            </Text>
          </Card>
        </Col>
        <Col xs={12} md={6}>
          <Card
            hoverable
            style={{ cursor: 'pointer' }}
            onClick={() => router.push('/knowledge-bases')}
          >
            <Statistic
              title="文档"
              value={stats?.document_count ?? 0}
              loading={loading}
            />
            <Text type="secondary" style={{ fontSize: 12 }}>
              点击查看文档列表
            </Text>
          </Card>
        </Col>
        <Col xs={12} md={6}>
          <Card
            hoverable
            style={{ cursor: 'pointer' }}
            onClick={() => router.push(jobsHref)}
          >
            <Statistic
              title="处理中任务"
              value={stats?.processing_job_count ?? 0}
              loading={loading}
              prefix={<SyncOutlined spin={!!stats && stats.processing_job_count > 0} />}
            />
            <Text type="secondary" style={{ fontSize: 12 }}>
              点击查看任务队列
            </Text>
          </Card>
        </Col>
        <Col xs={12} md={6}>
          <Card
            hoverable
            style={{ cursor: 'pointer' }}
            onClick={() => router.push(jobsHref)}
          >
            <Statistic
              title="失败任务"
              value={stats?.failed_job_count ?? 0}
              loading={loading}
              valueStyle={stats && stats.failed_job_count > 0 ? { color: '#cf1322' } : undefined}
              prefix={stats && stats.failed_job_count > 0 ? <WarningOutlined /> : undefined}
            />
            <Text type="secondary" style={{ fontSize: 12 }}>
              点击查看失败任务
            </Text>
          </Card>
        </Col>
      </Row>

      {/* 快捷入口 */}
      <Row gutter={[16, 16]}>
        <Col xs={24} md={12}>
          <Card title="知识库管理" extra={<Link href="/knowledge-bases">打开</Link>}>
            <Paragraph style={{ marginBottom: 0 }}>
              管理知识库与文档，上传文件并触发入库任务。
            </Paragraph>
          </Card>
        </Col>
        <Col xs={24} md={12}>
          <Card title="检索实验室" extra={<Link href="/retrieval-lab">打开</Link>}>
            <Paragraph style={{ marginBottom: 0 }}>
              运行检索测试，验证入库效果与召回质量。
            </Paragraph>
          </Card>
        </Col>
      </Row>

      {/* Phase 1 指标区域（预留，P0 仅展示空状态） */}
      <div>
        <Title level={4} style={{ marginBottom: 4 }}>
          监控指标（Phase 1）
        </Title>
        <Paragraph type="secondary" style={{ marginBottom: 16 }}>
          以下指标将在 Phase 1 接入，当前仅预留位置。所需 API 端点已标注。
        </Paragraph>
        <Row gutter={[16, 16]}>
          {PHASE1_METRICS.map((metric) => (
            <Col key={metric.title} xs={24} md={12} xl={6}>
              <Card title={metric.title} style={{ minHeight: 160 }}>
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description={
                    <span>
                      待接入
                      <br />
                      <Text type="secondary" style={{ fontSize: 11 }}>
                        {metric.api}
                      </Text>
                    </span>
                  }
                />
              </Card>
            </Col>
          ))}
        </Row>
      </div>
    </div>
  );
}
