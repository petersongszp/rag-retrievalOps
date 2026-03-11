'use client';

import { useEffect, useMemo, useState } from 'react';
import {
  Typography,
  Row,
  Col,
  Card as AntCard,
  Select,
  Empty,
  Button,
  Tag,
  message,
  Pagination,
  Spin,
} from 'antd';
import { ReloadOutlined, SmileOutlined } from '@ant-design/icons';
import Link from 'next/link';
import apiClient from '@/services/api/client';
import { API_BASE_URL } from '@/config/api';

const { Title } = Typography;

const QUOTES = [
  '面试是双向选择，保持自信，展现最好的自己。',
  '每一次面试都是一次成长的机会，无论结果如何，你都在进步。',
  '相信自己的积累，你比想象中更优秀。',
  '保持平常心，最好的机会往往在不经意间到来。',
  '失败只是暂时的，坚持下去，成功就在拐角处。',
  '准备充分，心态平和，你一定行！',
  '每一个Offer背后，都有无数次的努力与尝试。',
  '面试官也是未来的同事，像朋友一样交流吧。',
  '星光不问赶路人，时光不负有心人。',
  '沉着冷静，你的潜力无限大。',
];

export default function InterviewRecordsPage() {
  const [quote, setQuote] = useState('');
  const [filter, setFilter] = useState('全部');

  const [list, setList] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [currentPage, setCurrentPage] = useState(1); // 前端分页当前页
  const pageSize = 6; // 前端分页：每页显示6条

  const refreshQuote = () => {
    const random = QUOTES[Math.floor(Math.random() * QUOTES.length)];
    setQuote(random);
  };

  useEffect(() => {
    refreshQuote();
  }, []);

  const fetchList = async () => {
    setLoading(true);
    try {
      // 一次性获取所有数据，然后在前端分页
      const res: any = await apiClient.get('/interview/records', {
        params: { page: 1, page_size: 1000 },
      });
      const data = res?.data || res;
      const items = (data?.records || []).map((it: any) => ({
        id: it.id,
        userId: it.user_id,
        title: it.title,
        type: it.type,
        difficulty: it.difficulty,
        domain: it.domain,
        companyName: it.company_name,
        status: it.status,
        createdAt: it.created_at,
        updatedAt: it.updated_at,
      }));
      setList(items);
    } catch (e: any) {
      message.error(e?.response?.data?.message || '加载失败');
    } finally {
      setLoading(false);
    }
  };

  // 移除 mock 接口调用，改用真实接口

  useEffect(() => {
    fetchList();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const completedCount = useMemo(
    () => list.filter((it) => it.status === 'completed').length,
    [list]
  );
  const totalCount = useMemo(() => list.length, [list]);

  const filteredList = useMemo(() => {
    let filtered = list;

    // 根据类型筛选
    if (filter !== '全部') {
      if (filter === '综合面试') {
        filtered = filtered.filter((it) => it.type === '综合面试');
      } else if (filter === '社招' || filter === '校招') {
        filtered = filtered.filter((it) => it.domain === filter);
      }
    }

    return filtered;
  }, [list, filter]);

  // 前端分页展示的数据
  const paginatedList = useMemo(() => {
    const startIndex = (currentPage - 1) * pageSize;
    const endIndex = startIndex + pageSize;
    return filteredList.slice(startIndex, endIndex);
  }, [filteredList, currentPage, pageSize]);

  // 处理筛选变化时重置页码
  useEffect(() => {
    setCurrentPage(1);
  }, [filter]);

  // 处理分页变化
  const handlePageChange = (page: number) => {
    setCurrentPage(page);
    window.scrollTo({ top: 0, behavior: 'smooth' });
  };

  return (
    <div className="min-h-screen relative font-sans">
      {/* Decorative Background */}
      <div className="fixed top-0 right-0 w-[600px] h-[600px] bg-indigo-50/60 rounded-full blur-[120px] -translate-y-1/2 translate-x-1/3 pointer-events-none z-0" />
      <div className="fixed bottom-0 left-0 w-[600px] h-[600px] bg-blue-50/60 rounded-full blur-[120px] translate-y-1/2 -translate-x-1/3 pointer-events-none z-0" />

      <div className="container mx-auto px-4 relative z-10">
        <div className="mb-8 animate-fade-in-up">
          <h1 className="text-3xl font-extrabold text-slate-900 tracking-tight">面试记录</h1>
          <p className="text-slate-500 mt-2">查看你的所有面试历史、评估报告与详细反馈</p>
        </div>

        <AntCard
          className="rounded-3xl border border-slate-100 shadow-xl shadow-slate-200/50 animate-fade-in-up overflow-hidden mb-8"
          styles={{ body: { padding: 0 } }}
          style={{ animationDelay: '0.1s' }}
        >
          <div className="bg-gradient-to-r from-blue-50/50 to-indigo-50/50 p-8">
            <Row gutter={[24, 24]} align="middle">
              <Col xs={24} md={10}>
                <div className="bg-white/80 backdrop-blur-sm rounded-2xl p-6 shadow-sm border border-slate-100">
                  <div className="grid grid-cols-2 gap-6 items-center">
                    <div className="text-center border-r border-slate-100">
                      <div className="text-4xl font-extrabold text-slate-800 mb-1">
                        {totalCount}
                      </div>
                      <div className="text-sm text-slate-500 font-medium">面试总数(次)</div>
                    </div>
                    <div className="text-center">
                      <div className="text-4xl font-extrabold text-blue-600 mb-1">
                        {completedCount}
                      </div>
                      <div className="text-sm text-slate-500 font-medium">已完成面试(次)</div>
                    </div>
                  </div>
                </div>
              </Col>
              <Col xs={24} md={14} className="flex">
                <div className="w-full bg-white/60 backdrop-blur-sm rounded-2xl p-6 shadow-sm border border-slate-100 relative overflow-hidden group flex flex-col justify-center">
                  <div className="absolute top-4 right-4 opacity-0 group-hover:opacity-100 transition-opacity z-10">
                    <Button
                      type="text"
                      icon={<ReloadOutlined />}
                      onClick={refreshQuote}
                      className="text-slate-400 hover:text-blue-600 bg-white/50 hover:bg-white rounded-full"
                      title="换一句"
                    />
                  </div>
                  <div className="flex items-start gap-5">
                    <div className="bg-gradient-to-br from-blue-100 to-indigo-100 p-4 rounded-full text-blue-600 shrink-0 shadow-inner">
                      <SmileOutlined className="text-2xl" />
                    </div>
                    <div className="flex-1 pt-1">
                      <h3 className="text-lg font-bold text-slate-800 mb-2 flex items-center gap-2">
                        每日寄语
                        <span className="text-xs font-normal text-slate-400 bg-slate-100 px-2 py-0.5 rounded-full">
                          Motivation
                        </span>
                      </h3>
                      <p className="text-slate-600 text-base leading-relaxed italic relative">
                        <span className="text-3xl text-slate-300 absolute -top-2 -left-2 font-serif">
                          &quot;
                        </span>
                        <span className="relative z-10 pl-4">{quote}</span>
                        <span className="text-3xl text-slate-300 absolute -bottom-4 -right-2 font-serif">
                          &quot;
                        </span>
                      </p>
                    </div>
                  </div>
                </div>
              </Col>
            </Row>
          </div>
        </AntCard>

        <div className="animate-fade-in-up" style={{ animationDelay: '0.2s' }}>
          <div className="flex items-center justify-between mb-6 bg-white p-4 rounded-2xl border border-slate-100 shadow-sm">
            <div className="flex items-center gap-4">
              <span className="text-slate-600 font-medium">筛选面试记录：</span>
              <Select
                value={filter}
                onChange={setFilter}
                style={{ width: 180 }}
                options={[
                  { value: '全部', label: '全部类型' },
                  { value: '综合面试', label: '综合面试' },
                  { value: '社招', label: '社招专项' },
                  { value: '校招', label: '校招专项' },
                ]}
                className="font-medium"
                size="large"
                variant="filled"
              />
            </div>
          </div>

          <div className="mt-4">
            <Spin spinning={loading}>
              {filteredList.length === 0 ? (
                <Empty
                  imageStyle={{ height: 160 }}
                  description={
                    <div className="flex flex-col items-center">
                      <div className="text-slate-500 text-lg mb-4">暂时没有相关的面试记录</div>
                      <div className="flex gap-4">
                        <Link href="/interview/social">
                          <Button
                            type="primary"
                            className="bg-blue-600 h-10 px-6 rounded-full shadow-blue-200"
                          >
                            社招简历面试
                          </Button>
                        </Link>
                        <Link href="/interview/campus">
                          <Button className="h-10 px-6 rounded-full border-slate-200 text-slate-600">
                            校招简历面试
                          </Button>
                        </Link>
                      </div>
                    </div>
                  }
                  className="bg-white rounded-3xl p-12 border border-slate-100 shadow-sm"
                />
              ) : (
                <>
                  <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                    {paginatedList.map((it: any) => {
                      const statusMap: Record<
                        string,
                        { text: string; color: string; bg: string; border: string }
                      > = {
                        pending: {
                          text: '待面试',
                          color: 'text-blue-600',
                          bg: 'bg-blue-50',
                          border: 'border-blue-100',
                        },
                        in_progress: {
                          text: '进行中',
                          color: 'text-orange-600',
                          bg: 'bg-orange-50',
                          border: 'border-orange-100',
                        },
                        completed: {
                          text: '已完成',
                          color: 'text-green-600',
                          bg: 'bg-green-50',
                          border: 'border-green-100',
                        },
                      };
                      const statusInfo = statusMap[it.status] || {
                        text: it.status,
                        color: 'text-slate-600',
                        bg: 'bg-slate-50',
                        border: 'border-slate-100',
                      };
                      const createdTime = it.createdAt
                        ? new Date(it.createdAt).toLocaleString('zh-CN')
                        : '-';

                      return (
                        <div
                          key={it.id}
                          className="group bg-white rounded-2xl p-6 border border-slate-100 shadow-lg shadow-slate-100/50 hover:shadow-xl hover:shadow-blue-100/50 hover:-translate-y-1 transition-all duration-300 relative overflow-hidden"
                        >
                          <div
                            className={`absolute top-0 right-0 w-24 h-24 ${statusInfo.bg} rounded-bl-full -mr-8 -mt-8 opacity-50`}
                          />

                          <div className="relative z-10">
                            <div className="flex justify-between items-start mb-4">
                              <div
                                className={`px-3 py-1 rounded-full text-xs font-bold ${statusInfo.bg} ${statusInfo.color} ${statusInfo.border} border`}
                              >
                                {statusInfo.text}
                              </div>
                              <div className="text-xs text-slate-400 font-mono">
                                {createdTime.split(' ')[0]}
                              </div>
                            </div>

                            <h3
                              className="text-lg font-bold text-slate-800 mb-2 line-clamp-1"
                              title={it.title || it.companyName}
                            >
                              {it.type === '专项面试' && !it.companyName
                                ? it.title
                                : `${it.companyName || '未命名公司'}${it.title ? ` - ${it.title}` : ''}`}
                            </h3>

                            <div className="space-y-2 mb-6">
                              <div className="flex items-center justify-between text-sm">
                                <span className="text-slate-500">面试类型</span>
                                <span className="font-medium text-slate-700">{it.type || '-'}</span>
                              </div>
                              <div className="flex items-center justify-between text-sm">
                                <span className="text-slate-500">领域/方向</span>
                                <span className="font-medium text-slate-700">
                                  {it.domain || '-'}
                                </span>
                              </div>
                              <div className="flex items-center justify-between text-sm">
                                <span className="text-slate-500">难度等级</span>
                                <span className="font-medium text-slate-700">
                                  {it.difficulty || '-'}
                                </span>
                              </div>
                            </div>

                            <Link href={`/user/interviews/results/${it.id}`} className="block">
                              <Button
                                type="primary"
                                ghost
                                className="w-full h-10 rounded-xl border-blue-200 text-blue-600 hover:bg-blue-50 hover:border-blue-300 font-medium"
                              >
                                查看详情与反馈
                              </Button>
                            </Link>
                          </div>
                        </div>
                      );
                    })}
                  </div>

                  {/* 分页组件 */}
                  {filteredList.length > pageSize && (
                    <div className="flex justify-center mt-10">
                      <Pagination
                        current={currentPage}
                        total={filteredList.length}
                        pageSize={pageSize}
                        onChange={handlePageChange}
                        showSizeChanger={false}
                        showTotal={(total) => (
                          <span className="text-slate-500">共 {total} 条记录</span>
                        )}
                        className="bg-white px-4 py-2 rounded-full shadow-sm border border-slate-100"
                      />
                    </div>
                  )}
                </>
              )}
            </Spin>
          </div>
        </div>
      </div>
    </div>
  );
}
