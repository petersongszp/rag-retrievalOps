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
  Descriptions,
  Divider,
  Empty,
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
const SEMANTIC_CACHE_LATENCY_GUARD_THRESHOLD_MS = 200;

interface SemanticCacheGate {
  generated_at: string;
  passed: boolean;
  enabled: boolean;
  hit_rate: number;
  lookup_p95_ms: number;
  warm_lookup_p95_ms: number;
  false_hit_count: number;
  saved_retrieval_cost: number;
  saved_rerank_cost: number;
  isolation_guard_passed: boolean;
  latency_guard_passed: boolean;
  latency_guard_basis: string;
  latency_guard_note: string;
  observability_guard_passed: boolean;
  rollback_ready: boolean;
  hit_count: number;
  lookup_count: number;
  embedding_cache_observed_count: number;
  embedding_cache_hit_count: number;
  embedding_cache_hit_rate: number;
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

function formatPercent(value?: number) {
  if (value === undefined || value === null) {
    return '-';
  }
  return `${(value * 100).toFixed(1)}%`;
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
      const [gateData, acceptanceData] = await Promise.all([
        apiClient.get(KB_ADMIN_API.SEMANTIC_CACHE_GATE) as Promise<SemanticCacheGate>,
        apiClient.get(KB_ADMIN_API.SEMANTIC_CACHE_ACCEPTANCE) as Promise<SemanticCacheAcceptance>,
      ]);
      setGate(gateData);
      setAcceptance(acceptanceData);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : '加载语义缓存数据失败');
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
        <Spin size="large" tip="正在加载语义缓存数据..." />
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
            <Button onClick={() => void loadData()} icon={<ReloadOutlined />}>
              重试
            </Button>
          }
        />
      </div>
    );
  }

  const latencyDesc =
    gate?.latency_guard_basis === 'warm_lookup_with_embedding_cache_p95'
      ? `延迟校验优先按加速后的 Warm P95 判断，当前 ${gate?.warm_lookup_p95_ms ?? '-'}ms，要求 <= ${SEMANTIC_CACHE_LATENCY_GUARD_THRESHOLD_MS}ms。`
      : `当前还没有足够的加速后样本，临时按冷启动 P95 ${gate?.lookup_p95_ms ?? '-'}ms 兜底判断，要求 <= ${SEMANTIC_CACHE_LATENCY_GUARD_THRESHOLD_MS}ms。`;

  const guardItems = [
    {
      label: '隔离校验 (Isolation Guard)',
      passed: gate?.isolation_guard_passed ?? false,
      desc: '跨租户 / 跨知识库误命中检测。',
    },
    {
      label: '延迟校验 (Latency Guard)',
      passed: gate?.latency_guard_passed ?? false,
      desc: latencyDesc,
    },
    {
      label: '可观测性校验 (Observability Guard)',
      passed: gate?.observability_guard_passed ?? false,
      desc: '日志 / 指标 / 调试链路字段完整。',
    },
    {
      label: '回滚就绪 (Rollback Ready)',
      passed: gate?.rollback_ready ?? false,
      desc: '一键关闭 enable_semantic_cache 即可回退。',
    },
  ];

  return (
    <div style={{ padding: 24 }}>
      <Space style={{ marginBottom: 16, width: '100%', justifyContent: 'space-between' }}>
        <Space>
          <SafetyCertificateOutlined
            style={{ fontSize: 22, color: gate?.passed ? '#16a34a' : '#dc2626' }}
          />
          <Title level={3} style={{ margin: 0 }}>
            语义缓存 (Semantic Cache)
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
                  title="缓存命中率"
                  value={formatPercent(gate?.hit_rate)}
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
                  title="误命中数"
                  value={gate?.false_hit_count ?? '-'}
                  valueStyle={{ color: (gate?.false_hit_count ?? 0) > 0 ? '#dc2626' : '#16a34a' }}
                />
              </Col>
              <Col xs={12} sm={6}>
                <Statistic
                  title="节省检索成本"
                  value={gate ? gate.saved_retrieval_cost.toFixed(4) : '-'}
                  prefix="¥"
                />
              </Col>
            </Row>

            <Divider style={{ margin: '12px 0' }} />

            <Row gutter={[16, 16]}>
              <Col xs={12} sm={6}>
                <Statistic
                  title="冷启动 P95"
                  value={gate?.lookup_p95_ms ?? '-'}
                  suffix="ms"
                />
              </Col>
              <Col xs={12} sm={6}>
                <Statistic
                  title="加速后 P95"
                  value={gate?.warm_lookup_p95_ms ?? '-'}
                  suffix="ms"
                  valueStyle={{
                    color:
                      gate?.warm_lookup_p95_ms !== undefined &&
                      gate?.warm_lookup_p95_ms > SEMANTIC_CACHE_LATENCY_GUARD_THRESHOLD_MS
                        ? '#dc2626'
                        : '#16a34a',
                  }}
                />
              </Col>
              <Col xs={12} sm={6}>
                <Statistic
                  title="Embedding 缓存命中率"
                  value={formatPercent(gate?.embedding_cache_hit_rate)}
                  suffix={
                    gate ? (
                      <Text type="secondary" style={{ fontSize: 12 }}>
                        ({gate.embedding_cache_hit_count}/{gate.embedding_cache_observed_count})
                      </Text>
                    ) : undefined
                  }
                />
              </Col>
              <Col xs={12} sm={6}>
                <Statistic
                  title="节省重排成本"
                  value={gate ? gate.saved_rerank_cost.toFixed(4) : '-'}
                  prefix="¥"
                />
              </Col>
            </Row>

            <Divider style={{ margin: '12px 0' }} />
            <Text type="secondary">
              生成时间: {gate?.generated_at ? new Date(gate.generated_at).toLocaleString() : '-'}
            </Text>
          </Card>
        </Col>
      </Row>

      {gate?.latency_guard_note ? (
        <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
          <Col span={24}>
            <Alert
              type={gate.latency_guard_passed ? 'success' : 'warning'}
              showIcon
              message="延迟校验说明"
              description={gate.latency_guard_note}
            />
          </Col>
        </Row>
      ) : null}

      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col xs={24} md={12}>
          <Card
            title={
              <>
                <ThunderboltOutlined /> 门禁校验
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
                <WarningOutlined /> 风险点
              </>
            }
            size="small"
          >
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
                {acceptance?.generated_at
                  ? new Date(acceptance.generated_at).toLocaleString()
                  : '-'}
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

            <Divider orientation="left" style={{ fontSize: 13 }}>
              验收备注
            </Divider>
            {acceptance?.acceptance_notes?.length ? (
              <ul style={{ paddingLeft: 20, marginBottom: 0 }}>
                {acceptance.acceptance_notes.map((note, index) => (
                  <li key={index}>
                    <Text>{note}</Text>
                  </li>
                ))}
              </ul>
            ) : null}
          </Card>
        </Col>
      </Row>

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
                <Text code>
                  empty_query, debug_request, authorization_abnormal, high_risk_experiment
                </Text>
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
