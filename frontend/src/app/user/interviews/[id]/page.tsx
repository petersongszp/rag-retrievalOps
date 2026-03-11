'use client';

import { useMemo, useState } from 'react';
import { Typography, Row, Col, Card as AntCard, List, Tag, Space, Divider } from 'antd';
import { BookOutlined, BulbOutlined, CheckCircleOutlined } from '@ant-design/icons';

const { Title, Paragraph, Text } = Typography;

const staticData = {
  questions: [
    {
      title: '我看你简历上提到了RAG驱动AI旅行助手项目，你主要负责的部分吗？',
      question:
        '我看你简历上提到了RAG驱动AI旅行助手项目，能先跟我简单介绍一下这个项目的背景和你主要负责的部分吗？',
      idea: [
        {
          label: '背景',
          value: '项目背景兼顾的真实性、个人职责的清晰度、技术方案的合理性、项目成果的可信度',
        },
        { label: '职责', value: '负责AI助手的研发落地，提升游客户体验和员工效率' },
        { label: '挑战', value: '解决数据检索和知识问答的准确性问题，构建可解释的AI响应' },
        { label: '成果', value: '整体满意度提升，业务指标优化，成功率提升' },
      ],
      reference:
        '该项目为RAG驱动的旅行助手，整合检索与生成，在稳定性与响应速度上做了优化，通过流式SSE提升交互体验，线下部署容器化，核心链路可用性达到99%以上。',
      followups: [
        '这个AI助手为何选择RAG而不是单纯生成式模型驱动？',
        '你在项目中的技术决策有哪些？如何权衡准确性与性能？',
        '项目上线后，用户反馈和指标变化如何？下一步的优化方向？',
      ],
    },
    {
      title: '在这个AI旅行助手中如何做多源数据检索与融合？',
      question: '在这个AI旅行助手中，如何实现多源数据检索与融合以保证答复的可靠性？',
      idea: [
        { label: '背景', value: '多源数据，包括百科、攻略库、商家信息与用户生成内容' },
        { label: '职责', value: '负责检索管道设计与结果融合策略落地' },
        { label: '挑战', value: '异构数据的质量与时效性问题' },
        { label: '成果', value: '答复准确率与一致性提升' },
      ],
      reference:
        '采用分层检索与重排策略，BM25+向量检索结合，针对问句类别使用不同的融合权重与投票机制。',
      followups: ['你如何评估融合策略的效果？', '数据时效性问题如何处理？'],
    },
    {
      title: '在RAG系统中如何处理并发与延迟问题？',
      question: '在RAG系统中如何处理并发与延迟问题，确保用户体验？',
      idea: [
        { label: '背景', value: '高并发场景下检索与生成的协同' },
        { label: '职责', value: '优化请求调度与缓存策略' },
        { label: '挑战', value: '检索延迟与生成阻塞' },
        { label: '成果', value: '端到端延迟稳定在 100-200ms 量级（流式首包更快）' },
      ],
      reference:
        '采用异步管道与消息队列配合，本地向量缓存与热点文档预取，首包用SSE推送提升感知速度。',
      followups: ['为什么选择SSE而非WebSocket？', '缓存失效策略如何设计？'],
    },
  ],
};

export default function InterviewDetailPage() {
  const [selected, setSelected] = useState(0);
  const current = useMemo(
    () => staticData.questions[selected] || staticData.questions[0],
    [selected]
  );

  return (
    <div className="container mx-auto px-4">
      <Title level={2} className="mt-2">
        押题详情
      </Title>

      <Row gutter={[24, 24]} className="mt-2">
        <Col xs={24} md={6}>
          <AntCard className="rounded-2xl" styles={{ body: { padding: 0 } }}>
            <div className="p-4 border-b">
              <Space align="center">
                <BookOutlined />
                <Text>目录</Text>
              </Space>
            </div>
            <List
              itemLayout="horizontal"
              dataSource={staticData.questions}
              renderItem={(item, index) => (
                <List.Item
                  onClick={() => setSelected(index)}
                  style={{
                    cursor: 'pointer',
                    background: index === selected ? '#f6ffed' : undefined,
                  }}
                >
                  <List.Item.Meta
                    title={
                      <span>
                        {index + 1}. {item.title}
                      </span>
                    }
                  />
                </List.Item>
              )}
            />
          </AntCard>
        </Col>
        <Col xs={24} md={18}>
          <AntCard className="rounded-2xl" styles={{ body: { padding: 24 } }}>
            <Space size={12} style={{ width: '100%', justifyContent: 'space-between' }}>
              <Title level={4} style={{ margin: 0 }}>
                {current.question}
              </Title>
              <Space>
                <Tag color="green">整体思路</Tag>
                <Tag>参考答案</Tag>
                <Tag>收藏单题</Tag>
                <Tag>复制题目</Tag>
              </Space>
            </Space>
            <Divider />

            <Space direction="vertical" style={{ width: '100%' }}>
              <Space align="center" className="text-green-700">
                <CheckCircleOutlined />
                <Text strong>答案思路</Text>
              </Space>
              <Row gutter={[12, 12]}>
                {current.idea.map((it, idx) => (
                  <Col key={idx} xs={24} md={12}>
                    <AntCard size="small">
                      <Space>
                        <BulbOutlined />
                        <Text strong>{it.label}</Text>
                      </Space>
                      <Paragraph style={{ marginTop: 8 }}>{it.value}</Paragraph>
                    </AntCard>
                  </Col>
                ))}
              </Row>
            </Space>

            <Divider />

            <Space direction="vertical" style={{ width: '100%' }}>
              <Text strong>参考答案</Text>
              <Paragraph>{current.reference}</Paragraph>
            </Space>

            <Divider />

            <Space direction="vertical" style={{ width: '100%' }}>
              <Text strong>可能追问</Text>
              <List
                dataSource={current.followups}
                renderItem={(t) => (
                  <List.Item>
                    <Text>{t}</Text>
                  </List.Item>
                )}
              />
            </Space>
          </AntCard>
        </Col>
      </Row>
    </div>
  );
}
