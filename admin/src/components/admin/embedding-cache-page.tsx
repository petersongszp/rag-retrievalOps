'use client';

import { useCallback, useEffect, useState } from 'react';
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  DatabaseOutlined,
  ReloadOutlined,
  SafetyCertificateOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  Col,
  Descriptions,
  Divider,
  Empty,
  Progress,
  Row,
  Space,
  Spin,
  Statistic,
  Tag,
  Timeline,
  Typography,
} from 'antd';
import { KB_ADMIN_API } from '@/config/api';
import apiClient from '@/services/api/client';

const { Title, Text } = Typography;

interface EmbeddingCacheGate {
  generated_at: string;
  passed: boolean;
  enabled: boolean;
  hit_rate: number;
  lookup_p95_ms: number;
  isolation_guard_passed: boolean;
  latency_guard_passed: boolean;
  observability_guard_passed: boolean;
  rollback_ready: boolean;
  hit_count: number;
  lookup_count: number;
  risks: string[];
}

interface EmbeddingCacheAcceptance {
  generated_at: string;
  phase: string;
  gate: EmbeddingCacheGate;
  canary_plan: string[];
  rollback_plan: string[];
  accepted: boolean;
  acceptance_notes: string[];
}

interface EmbeddingCacheReport {
  generated_at: string;
  phase: string;
  accepted: boolean;
  artifacts: {
    implementation_guide: string;
    acceptance_report: string;
    meeting_brief: string;
    admin_endpoints: string[];
  };
  implementation_summary: Array<{
    layer: string;
    goal: string;
    delivered: string[];
    why_this_order: string;
    starter_hint: string;
  }>;
  test_summary: {
    focused_coverage: string[];
    acceptance_checks: string[];
    recommended_smoke: string[];
  };
  gate: EmbeddingCacheGate;
  benefit_summary: {
    lookup_count: number;
    hit_count: number;
    hit_rate: number;
    lookup_p95_ms: number;
    observability_healthy: boolean;
  };
  risks: string[];
  next_actions: string[];
}

export function EmbeddingCachePage() {
  const [gate, setGate] = useState<EmbeddingCacheGate | null>(null);
  const [acceptance, setAcceptance] = useState<EmbeddingCacheAcceptance | null>(null);
  const [report, setReport] = useState<EmbeddingCacheReport | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const [gateData, acceptanceData, reportData] = await Promise.all([
        apiClient.get(KB_ADMIN_API.EMBEDDING_CACHE_GATE) as Promise<EmbeddingCacheGate>,
        apiClient.get(KB_ADMIN_API.EMBEDDING_CACHE_ACCEPTANCE) as Promise<EmbeddingCacheAcceptance>,
        apiClient.get(KB_ADMIN_API.EMBEDDING_CACHE_REPORT) as Promise<EmbeddingCacheReport>,
      ]);
      setGate(gateData);
      setAcceptance(acceptanceData);
      setReport(reportData);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : '加载 Embedding 缓存数据失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: 80 }}>
        <Spin size="large" tip="正在加载 Embedding 缓存数据..." />
      </div>
    );
  }

  if (error && !gate && !acceptance && !report) {
    return (
      <div style={{ padding: 24 }}>
        <Alert
          type="error"
          message="加载失败"
          description={error}
          showIcon
          action={
            <Button onClick={() => void loadData()} icon={<ReloadOutlined />}>
              重试
            </Button>
          }
        />
      </div>
    );
  }

  const guardItems = [
    {
      label: '隔离校验',
      passed: gate?.isolation_guard_passed ?? false,
      desc: '仅允许 query / retrieval path 使用 embedding cache。',
    },
    {
      label: '延迟校验',
      passed: gate?.latency_guard_passed ?? false,
      desc: `Lookup P95 应保持稳定，当前值: ${gate?.lookup_p95_ms ?? '-'} ms。`,
    },
    {
      label: '可观测性校验',
      passed: gate?.observability_guard_passed ?? false,
      desc: 'Retrieve logs 与 debug trace 必须完整记录 hit / miss / reason 字段。',
    },
    {
      label: '回滚就绪',
      passed: gate?.rollback_ready ?? false,
      desc: '关闭 embedding.enable_cache 后应立即恢复到原始链路。',
    },
  ];

  return (
    <div style={{ padding: 24 }}>
      <Space style={{ marginBottom: 16, width: '100%', justifyContent: 'space-between' }}>
        <Space>
          <DatabaseOutlined style={{ fontSize: 22, color: gate?.passed ? '#16a34a' : '#dc2626' }} />
          <Title level={3} style={{ margin: 0 }}>
            Embedding 缓存
          </Title>
          {gate ? (
            <Tag
              color={gate.passed ? 'success' : 'error'}
              icon={gate.passed ? <CheckCircleOutlined /> : <CloseCircleOutlined />}
            >
              {gate.passed ? 'Gate 已通过' : 'Gate 未通过'}
            </Tag>
          ) : null}
        </Space>
        <Button icon={<ReloadOutlined />} onClick={() => void loadData()}>
          刷新
        </Button>
      </Space>

      <Row gutter={[16, 16]}>
        <Col span={24}>
          <Card title="Gate 状态" size="small">
            <Row gutter={[16, 16]}>
              <Col xs={12} sm={6}>
                <Statistic
                  title="功能开关"
                  value={gate?.enabled ? '已开启' : '已关闭'}
                  valueStyle={{ color: gate?.enabled ? '#16a34a' : '#8c8c8c' }}
                />
              </Col>
              <Col xs={12} sm={6}>
                <Statistic
                  title="命中率"
                  value={gate ? `${(gate.hit_rate * 100).toFixed(1)}%` : '-'}
                  suffix={
                    gate ? (
                      <Text type="secondary" style={{ fontSize: 12 }}>
                        ({gate.hit_count}/{gate.lookup_count})
                      </Text>
                    ) : undefined
                  }
                />
              </Col>
              <Col xs={12} sm={6}>
                <Statistic
                  title="Lookup P95"
                  value={gate?.lookup_p95_ms ?? '-'}
                  suffix="ms"
                  valueStyle={{ color: (gate?.lookup_p95_ms ?? 0) > 15 ? '#dc2626' : undefined }}
                />
              </Col>
              <Col xs={12} sm={6}>
                <Statistic
                  title="可观测性"
                  value={report?.benefit_summary.observability_healthy ? '健康' : '待修复'}
                  valueStyle={{
                    color: report?.benefit_summary.observability_healthy ? '#16a34a' : '#dc2626',
                  }}
                />
              </Col>
            </Row>
            <Divider style={{ margin: '12px 0' }} />
            <Space direction="vertical" style={{ width: '100%' }}>
              <Text type="secondary">
                生成时间: {gate?.generated_at ? new Date(gate.generated_at).toLocaleString() : '-'}
              </Text>
              <Progress
                percent={gate ? Number((gate.hit_rate * 100).toFixed(1)) : 0}
                status={gate?.passed ? 'success' : 'active'}
              />
            </Space>
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col xs={24} md={12}>
          <Card
            title={
              <>
                <SafetyCertificateOutlined /> 门禁校验
              </>
            }
            size="small"
          >
            {guardItems.map((item) => (
              <div key={item.label} style={{ marginBottom: 12 }}>
                <Space align="start">
                  {item.passed ? (
                    <CheckCircleOutlined style={{ color: '#16a34a', marginTop: 4 }} />
                  ) : (
                    <CloseCircleOutlined style={{ color: '#dc2626', marginTop: 4 }} />
                  )}
                  <div>
                    <Text strong>{item.label}</Text>
                    <br />
                    <Text type="secondary">{item.desc}</Text>
                  </div>
                </Space>
              </div>
            ))}
          </Card>
        </Col>
        <Col xs={24} md={12}>
          <Card
            title={
              <>
                <WarningOutlined /> 风险点与后续动作
              </>
            }
            size="small"
          >
            {report?.risks && report.risks.length > 0 ? (
              <Timeline
                items={report.risks.map((risk) => ({
                  color: 'red',
                  children: <Text type="danger">{risk}</Text>,
                }))}
              />
            ) : (
              <Empty description="暂无风险" image={Empty.PRESENTED_IMAGE_SIMPLE} />
            )}
            <Divider style={{ margin: '12px 0' }} />
            {report?.next_actions?.length ? (
              <Timeline
                items={report.next_actions.map((action) => ({
                  color: 'blue',
                  children: action,
                }))}
              />
            ) : null}
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col span={24}>
          <Card title="验收报告" size="small">
            <Descriptions column={2} size="small">
              <Descriptions.Item label="阶段">{acceptance?.phase ?? report?.phase ?? '-'}</Descriptions.Item>
              <Descriptions.Item label="结论">
                <Tag color={acceptance?.accepted ? 'success' : 'warning'}>
                  {acceptance?.accepted ? '已验收' : '继续观察'}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="实施文档">
                {report?.artifacts.implementation_guide ?? '-'}
              </Descriptions.Item>
              <Descriptions.Item label="验收文档">
                {report?.artifacts.acceptance_report ?? '-'}
              </Descriptions.Item>
            </Descriptions>

            <Divider orientation="left" style={{ fontSize: 13 }}>
              灰度计划
            </Divider>
            {acceptance?.canary_plan?.length ? (
              <Timeline
                items={acceptance.canary_plan.map((step, index) => ({
                  color: index === 0 ? 'green' : 'blue',
                  children: step,
                }))}
              />
            ) : (
              <Empty description="暂无灰度计划" image={Empty.PRESENTED_IMAGE_SIMPLE} />
            )}

            <Divider orientation="left" style={{ fontSize: 13 }}>
              回滚方案
            </Divider>
            {acceptance?.rollback_plan?.length ? (
              <Timeline
                items={acceptance.rollback_plan.map((step) => ({
                  color: 'orange',
                  children: step,
                }))}
              />
            ) : (
              <Empty description="暂无回滚方案" image={Empty.PRESENTED_IMAGE_SIMPLE} />
            )}
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col span={24}>
          <Card title="L0-L9 实施摘要" size="small">
            {report?.implementation_summary?.length ? (
              <Timeline
                items={report.implementation_summary.map((item) => ({
                  color: 'blue',
                  children: (
                    <div>
                      <Text strong>{item.layer}</Text>
                      <br />
                      <Text>{item.goal}</Text>
                    </div>
                  ),
                }))}
              />
            ) : (
              <Empty description="暂无实施摘要" image={Empty.PRESENTED_IMAGE_SIMPLE} />
            )}
          </Card>
        </Col>
      </Row>
    </div>
  );
}
