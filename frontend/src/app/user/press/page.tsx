'use client';

import { useState, useEffect } from 'react';
import {
  Table,
  Select,
  Input,
  Space,
  Button,
  message,
  Tag,
  Tooltip,
  Popconfirm,
} from 'antd';
import {
  RocketOutlined,
  SearchOutlined,
  DeleteOutlined,
  ReloadOutlined,
  EyeOutlined,
} from '@ant-design/icons';
import { predictionService } from '@/services/api/prediction';
import { PredictionRecordItem } from '@/types/prediction';

import Link from 'next/link';

export default function PressRecordsPage() {
  
  
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<PredictionRecordItem[]>([]);
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([]);
  const [searchText, setSearchText] = useState('');
  const [statusFilter, setStatusFilter] = useState<string>('all');

  const predictionTypeLabelMap: Record<string, string> = {
    校招: '校招',
    社招: '社招',
  };

  const difficultyLabelMap: Record<string, string> = {
    入门: '入门',
    初级: '入门',
    中级: '中级',
    进阶: '进阶',
    专家: '专家',
  };

  const toDeleteIDs = (keys: React.Key[]): number[] => {
    return keys
      .map((key) => Number(key))
      .filter((id) => Number.isInteger(id) && id > 0);
  };

  const formatCreatedAt = (createdAt: string | number | null | undefined): string => {
    if (createdAt == null || createdAt === '') return '-';

    if (typeof createdAt === 'number' || /^\d+$/.test(String(createdAt))) {
      const numeric = Number(createdAt);
      const ms = numeric < 1e12 ? numeric * 1000 : numeric;
      const date = new Date(ms);
      return Number.isNaN(date.getTime()) ? String(createdAt) : date.toLocaleString();
    }

    const raw = String(createdAt).trim();
    const normalized = raw.includes(' ') ? raw.replace(' ', 'T') : raw;
    const date = new Date(normalized);
    return Number.isNaN(date.getTime()) ? raw : date.toLocaleString();
  };

  // 获取押题记录
  const fetchRecords = async () => {
    setLoading(true);
    try {
      const res = await predictionService.getPredictionList();
      if (res && res.list) {
        setData(res.list);
      }
    } catch (e) {
      console.error(e);
      message.error("加载记录失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchRecords();
  }, []);

  // 批量删除
  const handleBatchDelete = async () => {
    const ids = toDeleteIDs(selectedRowKeys);
    if (ids.length === 0) {
      message.warning('未选择有效记录');
      return;
    }

    try {
      await predictionService.deleteHistory(ids);
      message.success('批量删除成功');
      setSelectedRowKeys([]);
      fetchRecords();
    } catch (e) {
      message.error('删除失败');
    }
  };

  // 单个删除
  const handleDelete = async (id: number) => {
    try {
      await predictionService.deleteHistory([id]);
      message.success('删除成功');
      fetchRecords();
    } catch (e) {
      message.error('删除失败');
    }
  };

  const columns = [
    {
      title: "ID",
      dataIndex: 'id',
      key: 'id',
      width: 80,
      render: (text: any) => <span className="text-slate-400">#{text}</span>,
    },
    {
      title: "类型",
      dataIndex: 'prediction_type',
      key: 'prediction_type',
      render: (type: string) => (
        <Tag color={type === '校招' ? 'green' : 'blue'} className="rounded-full px-2 border-0">
          {predictionTypeLabelMap[type] || type}
        </Tag>
      ),
    },
    {
      title: "难度",
      dataIndex: 'difficulty',
      key: 'difficulty',
      render: (level: string) => {
        const colors: Record<string, string> = {
          入门: 'default',
          初级: 'cyan',
          中级: 'blue',
          进阶: 'purple',
          专家: 'magenta',
        };
        return <Tag color={colors[level] || 'default'}>{difficultyLabelMap[level] || level}</Tag>;
      },
    },
    {
      title: "公司",
      dataIndex: 'company',
      key: 'company',
      render: (text: string) => (
        <span className="font-medium text-slate-700">{text || '-'}</span>
      ),
    },
    {
      title: "岗位",
      dataIndex: 'job_title',
      key: 'job_title',
      render: (text: string) => <span className="text-slate-600">{text}</span>,
    },
    {
      title: "语言",
      dataIndex: 'language',
      key: 'language',
    },
    {
      title: "时间",
      dataIndex: 'created_at',
      key: 'created_at',
      render: (createdAt: string | number) => (
        <span className="text-slate-400 text-sm">
          {formatCreatedAt(createdAt)}
        </span>
      ),
    },
    {
      title: "操作",
      key: 'action',
      render: (_: any, record: PredictionRecordItem) => (
        <Space size="small">
          <Link href={`/user/press/${record.id}`}>
            <Tooltip title={"查看详情"}>
              <Button
                type="text"
                icon={<EyeOutlined />}
                className="text-indigo-600 hover:bg-indigo-50"
              />
            </Tooltip>
          </Link>
          <Popconfirm
            title="确定要删除这条记录吗？"
            onConfirm={() => handleDelete(record.id)}
            okText="确定"
            cancelText="取消"
          >
            <Tooltip title="删除记录">
              <Button
                type="text"
                danger
                icon={<DeleteOutlined />}
                className="hover:bg-red-50"
              />
            </Tooltip>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const rowSelection = {
    selectedRowKeys,
    onChange: (newSelectedRowKeys: React.Key[]) => {
      setSelectedRowKeys(newSelectedRowKeys);
    },
  };

  const filteredData = data.filter((item) => {
    const matchSearch =
      item.company?.toLowerCase().includes(searchText.toLowerCase()) ||
      item.job_title?.toLowerCase().includes(searchText.toLowerCase());
    const matchStatus = statusFilter === 'all' ? true : true; // Status filter placeholder
    return matchSearch && matchStatus;
  });

  return (
    <div className="min-h-screen relative font-sans">
      <div className="fixed top-0 right-0 w-[600px] h-[600px] bg-purple-50/60 rounded-full blur-[120px] -translate-y-1/2 translate-x-1/3 pointer-events-none z-0" />
      <div className="fixed bottom-0 left-0 w-[600px] h-[600px] bg-pink-50/60 rounded-full blur-[120px] translate-y-1/2 -translate-x-1/3 pointer-events-none z-0" />

      <div className="container mx-auto px-4 relative z-10">
        <div className="mb-8 animate-fade-in-up">
          <h1 className="text-3xl font-extrabold text-slate-900 tracking-tight flex items-center gap-3">
            <RocketOutlined className="text-indigo-600" />
            {"押题记录"}
          </h1>
          <p className="text-slate-500 mt-2">{"查看历史记录并追踪押题效果"}</p>
        </div>

        <div className="bg-white rounded-3xl p-6 border border-slate-100 shadow-xl shadow-slate-200/50 animate-fade-in-up" style={{ animationDelay: '0.1s' }}>
          <div className="flex flex-col md:flex-row justify-between items-center mb-6 gap-4">
            <div className="flex items-center gap-4 w-full md:w-auto">
              <Select
                defaultValue="all"
                style={{ width: 120 }}
                onChange={setStatusFilter}
                options={[
                  { value: 'all', label: "全部状态" },
                ]}
                variant="filled"
                size="large"
              />
              <Input
                placeholder={"搜索公司..."}
                prefix={<SearchOutlined className="text-slate-400" />}
                value={searchText}
                onChange={(e) => setSearchText(e.target.value)}
                className="w-full md:w-64"
                variant="filled"
                size="large"
              />
            </div>
            <Space>
              {selectedRowKeys.length > 0 && (
                <span className="text-slate-500 text-sm bg-slate-50 px-3 py-1.5 rounded-lg border border-slate-100">
                  已选择 {selectedRowKeys.length} 条
                </span>
              )}
              {selectedRowKeys.length > 0 && (
                <Button
                  danger
                  icon={<DeleteOutlined />}
                  onClick={handleBatchDelete}
                  className="rounded-xl"
                >
                  {"批量删除"}
                </Button>
              )}
              <Button
                icon={<ReloadOutlined />}
                onClick={fetchRecords}
                className="rounded-xl hover:text-indigo-600 hover:border-indigo-200"
              >
                {"刷新"}
              </Button>
            </Space>
          </div>

          <Table
            rowSelection={rowSelection}
            columns={columns}
            dataSource={filteredData}
            rowKey="id"
            loading={loading}
            pagination={{
              pageSize: 10,
              showSizeChanger: true,
              showTotal: (total) => <span className="text-slate-400">共 {total} 条</span>,
            }}
            className="modern-table"
          />
        </div>
      </div>
    </div>
  );
}

