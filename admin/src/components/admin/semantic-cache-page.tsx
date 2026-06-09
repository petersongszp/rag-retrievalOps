'use client';

import { useCallback, useEffect, useState } from 'react';
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  ReloadOutlined,
  SafetyCertificateOutlined,
  ThunderboltOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  Col,
  Collapse,
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

const { Title, Text, Paragraph } = Typography;

interface SemanticCacheGate {
  generated_at: string;
  passed: boolean;
  enabled: boolean;
  hit_rate: number;
  lookup_p95_ms: number;
  false_hit_count: number;
  saved_retrieval_cost: number;
  saved_rerank_cost: number;
  isolation_guard_passed: boolean;
  latency_guard_passed: boolean;
  observability_guard_passed: boolean;
  rollback_ready: boolean;
  hit_count: number;
  lookup_count: number;
  risks: string[];
}

interface SemanticCacheAcceptance {
  generated_at: string;
  phase: string;
  gate: SemanticCacheGate;
  canary_plan: string[];
  rollback_plan: string[];
  accepted: boolean;
  acceptance_notes: string[];
}

export function SemanticCachePage() {
  const [gate, setGate] = useState<SemanticCacheGate | null>(null);
  const [acceptance, setAcceptance] = useState<SemanticCacheAcceptance | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const [gateData, accData] = await Promise.all([
        apiClient.get(KB_ADMIN_API.SEMANTIC_CACHE_GATE) as Promise<SemanticCacheGate>,
        apiClient.get(KB_ADMIN_API.SEMANTIC_CACHE_ACCEPTANCE) as Promise<SemanticCacheAcceptance>,
      ]);
      setGate(gateData);
      setAcceptance(accData);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : '加载失败';
      setError(msg);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadData();
  }, [loadData]);

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: 80 }}>
        <Spin size="large" tip="加载语义缓存数据..." />
      </div>
    );
  }

  if (error && !gate && !acceptance) {
    return (
      <div style={{ padding: 24 }}>
        <Alert
          type="error"
          message="加载失败"
          description={error}
          showIcon
          action={
            <Button onClick={loadData} icon={<ReloadOutlined />}>
              重试
            </Button>
          }
        />
        <Empty
          style={{ marginTop: 48 }}
          description="请确认后端服务已启动且 Semantic Cache API 已注册"
        />
      </div>
    );
  }

  const guardItems = [
    {
      label: '隔离校验 (Isolation Guard)',
      passed: gate?.isolation_guard_passed ?? false,
      desc: '跨租户/跨知识库误命中检测',
    },
    {
      label: '延迟校验 (Latency Guard)',
      passed: gate?.latency_guard_passed ?? false,
      desc: `P95 查找延迟 ≤ 80ms（当前: ${gate?.lookup_p95_ms ?? '-'}ms）`,
    },
    {
      label: '可观测性校验 (Observability Guard)',
      passed: gate?.observability_guard_passed ?? false,
      desc: '日志/指标/调试链路字段完整性',
    },
    {
      label: '回滚就绪 (Rollback Ready)',
      passed: gate?.rollback_ready ?? false,
      desc: '一键关闭 enable_semantic_cache 即可回退',
    },
  ];

  return (
    <div style={{ padding: 24 }}>
      <Space style={{ marginBottom: 16, width: '100%', justifyContent: 'space-between' }}>
        <Space>
          <SafetyCertificateOutlined style={{ fontSize: 22, color: gate?.passed ? '#16a34a' : '#dc2626' }} />
          <Title level={3} style={{ margin: 0 }}>
            语义缓存 (Semantic Cache)
          </Title>
          {gate && (
            <Tag color={gate.passed ? 'success' : 'error'} icon={gate.passed ? <CheckCircleOutlined /> : <CloseCircleOutlined />}>
              {gate.passed ? 'Gate 通过' : 'Gate 未通过'}
            </Tag>
          )}
        </Space>
        <Button icon={<ReloadOutlined />} onClick={loadData}>
          刷新
        </Button>
      </Space>

      {/* Gate Status */}
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
                  title="缓存命中率"
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
                <Statistic title="误命中数" value={gate?.false_hit_count ?? '-'} valueStyle={{ color: (gate?.false_hit_count ?? 0) > 0 ? '#dc2626' : '#16a34a' }} />
              </Col>
              <Col xs={12} sm={6}>
                <Statistic title="节省检索成本" value={gate ? gate.saved_retrieval_cost.toFixed(4) : '-'} prefix="¥" />
              </Col>
            </Row>
            <Divider style={{ margin: '12px 0' }} />
            <Row gutter={[16, 16]}>
              <Col xs={12} sm={6}>
                <Statistic title="P95 查找延迟" value={gate?.lookup_p95_ms ?? '-'} suffix="ms" valueStyle={{ color: (gate?.lookup_p95_ms ?? 0) > 80 ? '#dc2626' : undefined }} />
              </Col>
              <Col xs={12} sm={6}>
                <Statistic title="节省重排成本" value={gate ? gate.saved_rerank_cost.toFixed(4) : '-'} prefix="¥" />
              </Col>
              <Col span={12}>
                <Text type="secondary">生成时间: {gate?.generated_at ? new Date(gate.generated_at).toLocaleString() : '-'}</Text>
              </Col>
            </Row>
          </Card>
        </Col>
      </Row>

      {/* Guard Details */}
      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col xs={24} md={12}>
          <Card title={<><ThunderboltOutlined /> 门禁校验</>} size="small">
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
          <Card title={<><WarningOutlined /> 风险点</>} size="small">
            {gate?.risks && gate.risks.length > 0 ? (
              <Timeline
                items={gate.risks.map((risk) => ({
                  color: 'red',
                  children: <Text type="danger">{risk}</Text>,
                }))}
              />
            ) : (
              <Empty description="暂无风险" image={Empty.PRESENTED_IMAGE_SIMPLE} />
            )}
          </Card>
        </Col>
      </Row>

      {/* Acceptance Report */}
      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col span={24}>
          <Card
            title={
              <Space>
                <SafetyCertificateOutlined />
                验收报告
                <Tag color={acceptance?.accepted ? 'success' : 'warning'}>
                  {acceptance?.accepted ? '已验收' : '待验收'}
                </Tag>
              </Space>
            }
            size="small"
          >
            <Descriptions column={2} size="small">
              <Descriptions.Item label="阶段">{acceptance?.phase ?? '-'}</Descriptions.Item>
              <Descriptions.Item label="生成时间">
                {acceptance?.generated_at ? new Date(acceptance.generated_at).toLocaleString() : '-'}
              </Descriptions.Item>
            </Descriptions>

            <Divider orientation="left" style={{ fontSize: 13 }}>灰度计划</Divider>
            {acceptance?.canary_plan ? (
              <Timeline
                items={acceptance.canary_plan.map((step, i) => ({
                  color: i === 0 ? 'green' : 'blue',
                  children: step,
                }))}
              />
            ) : (
              <Empty description="无灰度计划" image={Empty.PRESENTED_IMAGE_SIMPLE} />
            )}

            <Divider orientation="left" style={{ fontSize: 13 }}>回滚方案</Divider>
            {acceptance?.rollback_plan ? (
              <Timeline
                items={acceptance.rollback_plan.map((step) => ({
                  color: 'orange',
                  children: step,
                }))}
              />
            ) : (
              <Empty description="无回滚方案" image={Empty.PRESENTED_IMAGE_SIMPLE} />
            )}

            <Divider orientation="left" style={{ fontSize: 13 }}>验收备注</Divider>
            {acceptance?.acceptance_notes?.length ? (
              <ul style={{ paddingLeft: 20 }}>
                {acceptance.acceptance_notes.map((note, i) => (
                  <li key={i}>
                    <Text>{note}</Text>
                  </li>
                ))}
              </ul>
            ) : null}
          </Card>
        </Col>
      </Row>

      {/* Design Contract Info */}
      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col span={24}>
          <Card title="命中契约" size="small">
            <Row gutter={[16, 8]}>
              <Col xs={12} sm={6}>
                <Text strong>隔离维度:</Text>
                <br />
                <Text code>tenant_id, kb_ids, strategy_version, query_type</Text>
              </Col>
              <Col xs={12} sm={6}>
                <Text strong>绕过条件:</Text>
                <br />
                <Text code>empty_query, debug_request, authorization_abnormal, high_risk_experiment</Text>
              </Col>
              <Col xs={12} sm={6}>
                <Text strong>结果契约:</Text>
                <br />
                <Text code>retrieve_result_only</Text>
              </Col>
              <Col xs={12} sm={6}>
                <Text strong>TopK 策略:</Text>
                <br />
                <Text code>exact_topk_only</Text>
              </Col>
            </Row>
          </Card>
        </Col>
      </Row>
    </div>
  );
}
