'use client';

import { useState, useEffect } from 'react';
import {
  Typography,
  Table,
  DatePicker,
  Select,
  Input,
  Space,
  Button,
  message,
} from 'antd';
import { predictionService } from '@/services/api/prediction';
import { PredictionRecordItem } from '@/types/prediction';
import Link from 'next/link';

const { RangePicker } = DatePicker;

export default function PressRecordsPage() {
  const [selectedKeys, setSelectedKeys] = useState<number[]>([]);
  const [status, setStatus] = useState<string>('全部状态');
  const [company, setCompany] = useState<string>('');
  
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<PredictionRecordItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);

  const fetchData = async () => {
    setLoading(true);
    try {
      const res = await predictionService.getPredictionList(page, pageSize);
      setData(res.list || []);
      setTotal(res.total || 0);
    } catch (error) {
      console.error('Failed to fetch prediction records:', error);
      message.error('获取押题记录失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, [page, pageSize]);

  // Client-side filtering for now, as the API doesn't support filtering yet
  const filtered = data.filter(
    (r) =>
      (status === '全部状态' || r.prediction_type === status) && // Note: API returns prediction_type, mapped to status logic if needed
      (company === '' || r.company.includes(company))
  );

  return (
    <div className="min-h-screen relative font-sans">
      {/* Decorative Background */}
      <div className="fixed top-0 right-0 w-[600px] h-[600px] bg-purple-50/60 rounded-full blur-[120px] -translate-y-1/2 translate-x-1/3 pointer-events-none z-0" />
      <div className="fixed bottom-0 left-0 w-[600px] h-[600px] bg-pink-50/60 rounded-full blur-[120px] translate-y-1/2 -translate-x-1/3 pointer-events-none z-0" />

      <div className="container mx-auto px-4 relative z-10">
        <div className="mb-8 animate-fade-in-up">
          <h1 className="text-3xl font-extrabold text-slate-900 tracking-tight">押题记录</h1>
          <p className="text-slate-500 mt-2">回顾你的历史押题，追踪面试预测准确度</p>
        </div>

        <div
          className="bg-white rounded-3xl p-8 border border-slate-100 shadow-xl shadow-slate-200/50 animate-fade-in-up"
          style={{ animationDelay: '0.1s' }}
        >
          <div className="flex flex-wrap items-center gap-4 mb-8 bg-slate-50/50 p-4 rounded-2xl border border-slate-100">
            <RangePicker
              className="border-slate-200 hover:border-blue-400 rounded-lg h-10"
              variant="filled"
            />
            {/* 
            <Select
              value={status}
              onChange={setStatus}
              options={[
                { value: '全部状态', label: '全部状态' },
                { value: '校招', label: '校招' },
                { value: '社招', label: '社招' },
              ]}
              className="min-w-[140px] h-10"
              size="large"
              variant="filled"
            />
            */}
            <Input
              placeholder="搜索公司名称..."
              value={company}
              onChange={(e) => setCompany(e.target.value)}
              className="w-[240px] h-10 rounded-lg border-slate-200 hover:border-blue-400 focus:border-blue-500"
              variant="filled"
            />
            <div className="flex-1" />
            <Button
              type="primary"
              className="bg-blue-600 hover:bg-blue-500 h-10 px-6 rounded-lg shadow-blue-200"
              onClick={fetchData}
            >
              刷新
            </Button>
          </div>

          <Table
            loading={loading}
            rowKey="id"
            rowSelection={{
              selectedRowKeys: selectedKeys,
              onChange: (keys) => setSelectedKeys(keys as number[]),
            }}
            columns={[
              {
                title: 'ID',
                dataIndex: 'id',
                render: (text) => <span className="font-medium text-slate-700">{text}</span>,
              },
              {
                title: '押题类型',
                dataIndex: 'prediction_type',
                render: (text) => <span className="text-slate-600">{text}</span>,
              },
              {
                title: '难度等级',
                dataIndex: 'difficulty',
                render: (text) => (
                  <span
                    className={`font-medium ${text === '进阶' ? 'text-purple-600' : text === '中级' ? 'text-blue-600' : 'text-slate-600'}`}
                  >
                    {text}
                  </span>
                ),
              },
              {
                title: '公司名称',
                dataIndex: 'company',
                render: (text) => <span className="font-bold text-slate-800">{text || '-'}</span>,
              },
              {
                title: '岗位名称',
                dataIndex: 'job_title',
                render: (text) => <span className="text-slate-600">{text}</span>,
              },
               {
                title: '语言',
                dataIndex: 'language',
                render: (text) => <span className="text-slate-600">{text}</span>,
              },
              {
                title: '押题时间',
                dataIndex: 'created_at',
                render: (text) => <span className="text-slate-500 text-sm font-mono">{text}</span>,
              },
              {
                title: '操作',
                render: (_: any, row: PredictionRecordItem) => (
                  <Space size="small">
                    <Link
                      href={`/user/press/${row.id}`}
                      className="text-blue-600 hover:text-blue-500 font-medium"
                    >
                      查看详情
                    </Link>
                  </Space>
                ),
              },
            ]}
            dataSource={filtered}
            pagination={{
              current: page,
              pageSize: pageSize,
              total: total,
              onChange: (p, s) => {
                setPage(p);
                setPageSize(s);
              },
              className: 'mt-6',
              showTotal: (total) => <span className="text-slate-500">共 {total} 条记录</span>,
            }}
            className="modern-table"
            rowClassName="hover:bg-slate-50 transition-colors"
          />

          {selectedKeys.length > 0 && (
            <div className="mt-4 p-3 bg-blue-50 border border-blue-100 rounded-xl flex items-center gap-2 text-blue-700 text-sm animate-fade-in-up">
              <span className="font-medium">已选择 {selectedKeys.length} 项</span>
              <div className="h-4 w-px bg-blue-200 mx-2" />
              <Button size="small" type="text" danger className="hover:bg-red-50">
                批量删除
              </Button>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
