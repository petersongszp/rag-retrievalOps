'use client';

import { useMemo, useState, useEffect } from 'react';
import { Typography, Row, Col, Card as AntCard, List, Tag, Space, Spin, message } from 'antd';
import { BookOutlined, BulbOutlined, CheckCircleOutlined, StarOutlined } from '@ant-design/icons';
import { useParams } from 'next/navigation';
import { predictionService } from '@/services/api/prediction';
import { PredictionQuestion } from '@/types/prediction';

const { Title, Paragraph, Text } = Typography;

export default function PressDetailPage() {
  const params = useParams();
  const id = Number(params.id);
  const [loading, setLoading] = useState(true);
  const [questions, setQuestions] = useState<PredictionQuestion[]>([]);
  const [selected, setSelected] = useState(0);

  useEffect(() => {
    if (!id) return;

    const fetchDetail = async () => {
      setLoading(true);
      try {
        const res = await predictionService.getPredictionDetail(id);
        setQuestions(res.questions || []);
      } catch (error) {
        console.error('Failed to fetch prediction detail:', error);
        message.error('获取押题详情失败');
      } finally {
        setLoading(false);
      }
    };

    fetchDetail();
  }, [id]);

  const current = useMemo(
    () => questions[selected] || null,
    [questions, selected]
  );

  const parsedFollowUps = useMemo(() => {
    if (!current?.follow_up) return [];
    try {
      const parsed = JSON.parse(current.follow_up);
      if (Array.isArray(parsed)) return parsed;
      return [current.follow_up];
    } catch (e) {
      return [current.follow_up];
    }
  }, [current]);

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <Spin size="large" tip="加载中..." />
      </div>
    );
  }

  if (!current) {
    return (
      <div className="min-h-screen flex items-center justify-center text-slate-500">
        未找到相关记录
      </div>
    );
  }

  return (
    <div className="min-h-screen relative font-sans">
      {/* Decorative Background */}
      <div className="fixed top-0 right-0 w-[600px] h-[600px] bg-green-50/60 rounded-full blur-[120px] -translate-y-1/2 translate-x-1/3 pointer-events-none z-0" />
      <div className="fixed bottom-0 left-0 w-[600px] h-[600px] bg-blue-50/60 rounded-full blur-[120px] translate-y-1/2 -translate-x-1/3 pointer-events-none z-0" />

      <div className="container mx-auto px-4 relative z-10 pb-12">
        <div className="mb-8 animate-fade-in-up">
          <h1 className="text-3xl font-extrabold text-slate-900 tracking-tight flex items-center gap-3">
            <BookOutlined className="text-blue-600" />
            押题详情
          </h1>
          <p className="text-slate-500 mt-2 ml-11">查看为您生成的精准面试题目与详细解析</p>
        </div>

        <Row
          gutter={[24, 24]}
          className="mt-2 animate-fade-in-up"
          style={{ animationDelay: '0.1s' }}
        >
          <Col xs={24} md={7} lg={6}>
            <AntCard
              className="rounded-2xl border-slate-100 shadow-lg shadow-slate-200/50 h-full"
              styles={{ body: { padding: 0 } }}
            >
              <div className="p-5 border-b border-slate-100 bg-slate-50/50 rounded-t-2xl">
                <Space align="center" className="font-bold text-slate-700">
                  <BookOutlined className="text-blue-500" />
                  <span>题目目录</span>
                </Space>
              </div>
              <div className="max-h-[calc(100vh-300px)] overflow-y-auto custom-scrollbar">
                <List
                  itemLayout="horizontal"
                  dataSource={questions}
                  split={false}
                  renderItem={(item, index) => (
                    <List.Item
                      onClick={() => setSelected(index)}
                      className={`transition-colors duration-200 cursor-pointer border-l-4 px-4 py-3 hover:bg-blue-50/50 ${
                        index === selected ? 'bg-blue-50 border-blue-500' : 'border-transparent'
                      }`}
                    >
                      <div className="w-full">
                        <div
                          className={`font-medium mb-1 line-clamp-2 ${index === selected ? 'text-blue-700' : 'text-slate-700'}`}
                        >
                          <span className="mr-2 text-slate-400">0{index + 1}.</span>
                          {item.question}
                        </div>
                      </div>
                    </List.Item>
                  )}
                />
              </div>
            </AntCard>
          </Col>
          <Col xs={24} md={17} lg={18}>
            <AntCard
              className="rounded-2xl border-slate-100 shadow-lg shadow-slate-200/50 min-h-[600px]"
              styles={{ body: { padding: 32 } }}
            >
              <div className="flex flex-col gap-6">
                <div className="flex flex-col md:flex-row md:items-start justify-between gap-4">
                  <h2 className="text-xl font-bold text-slate-800 leading-relaxed flex-1">
                    <span className="text-blue-600 mr-2 text-2xl">Q{selected + 1}.</span>
                    {current.question}
                  </h2>
                  <Space wrap>
                    <Tag
                      color="blue"
                      className="px-3 py-1 rounded-full border-0 bg-blue-50 text-blue-600 font-medium"
                    >
                      整体思路
                    </Tag>
                    <Tag
                      color="green"
                      className="px-3 py-1 rounded-full border-0 bg-green-50 text-green-600 font-medium"
                    >
                      参考答案
                    </Tag>
                    <Tag className="px-3 py-1 rounded-full border-slate-200 text-slate-500 hover:text-blue-600 cursor-pointer transition-colors">
                      收藏单题
                    </Tag>
                  </Space>
                </div>

                <div className="w-full h-px bg-slate-100" />

                <div className="space-y-8">
                  {/* Key Focus Section - New */}
                  <section>
                     <div className="flex items-center gap-2 mb-4 text-lg font-bold text-slate-800">
                      <div className="w-8 h-8 rounded-lg bg-purple-100 flex items-center justify-center text-purple-600">
                        <StarOutlined />
                      </div>
                      重点考察
                    </div>
                     <div className="bg-purple-50/50 rounded-xl p-4 border border-purple-100 text-slate-700">
                        {current.focus}
                     </div>
                  </section>

                  <section>
                    <div className="flex items-center gap-2 mb-4 text-lg font-bold text-slate-800">
                      <div className="w-8 h-8 rounded-lg bg-green-100 flex items-center justify-center text-green-600">
                        <CheckCircleOutlined />
                      </div>
                      答案思路
                    </div>
                    {/* Changed from structured idea array to text block for thinking_path */}
                     <div className="bg-slate-50 rounded-xl p-6 border border-slate-100 text-slate-700 leading-relaxed whitespace-pre-wrap">
                        {current.thinking_path}
                     </div>
                  </section>

                  <section>
                    <div className="flex items-center gap-2 mb-4 text-lg font-bold text-slate-800">
                      <div className="w-8 h-8 rounded-lg bg-indigo-100 flex items-center justify-center text-indigo-600">
                        <BookOutlined />
                      </div>
                      参考答案
                    </div>
                    <div className="bg-gradient-to-br from-slate-50 to-white rounded-xl p-6 border border-slate-100 text-slate-700 leading-loose whitespace-pre-wrap">
                      {current.reference_answer}
                    </div>
                  </section>

                  <section>
                    <div className="flex items-center gap-2 mb-4 text-lg font-bold text-slate-800">
                      <div className="w-8 h-8 rounded-lg bg-orange-100 flex items-center justify-center text-orange-600">
                        <BulbOutlined />
                      </div>
                      可能追问
                    </div>
                    <div className="bg-white rounded-xl border border-slate-100 overflow-hidden">
                      <List
                        dataSource={parsedFollowUps}
                        renderItem={(t, i) => (
                          <List.Item className="px-6 py-4 hover:bg-slate-50 transition-colors border-slate-50">
                            <div className="flex gap-3">
                              <span className="text-orange-500 font-bold font-mono">0{i + 1}</span>
                              <span className="text-slate-700">{t}</span>
                            </div>
                          </List.Item>
                        )}
                      />
                    </div>
                  </section>
                </div>
              </div>
            </AntCard>
          </Col>
        </Row>
      </div>
    </div>
  );
}
