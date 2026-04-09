'use client';

import { UploadOutlined, FileOutlined, DeleteOutlined, InboxOutlined } from '@ant-design/icons';
import { useEffect, useState, useCallback } from 'react';

import type { UploadProps } from 'antd';
import {
  Typography,
  Row,
  Col,
  Avatar,
  Tag,
  Button,
  Upload,
  message,
  Spin,
  Popconfirm,
  Table,
  Space,
} from 'antd';
import apiClient from '@/services/api/client';
import { USER_API, RESUME_API } from '@/config/api';

const { Dragger } = Upload;
const { Title } = Typography;

// 简历信息类型
interface ResumeInfo {
  id: number;
  user_id: number;
  file_name: string;
  file_size: number;
  file_type: string;
  is_default: number;
  created_at: number;
  updated_at: number;
  status?: string; // pending/processing/completed/failed
  error_msg?: string;
}

export default function UserCenterPage() {
  const [profile, setProfile] = useState<{ id?: number; username?: string; email?: string } | null>(
    null
  );

  const columns = [
    {
      title: 'Order ID',
      dataIndex: 'id',
      key: 'id',
      render: (text: string) => <span className="text-slate-500 font-mono">{text}</span>,
    },
    {
      title: 'Type',
      dataIndex: 'type',
      key: 'type',
      render: (text: string) => <span className="font-medium text-slate-700">{text}</span>,
    },
    {
      title: 'Amount',
      dataIndex: 'amount',
      key: 'amount',
      render: (amount: number) => <span className="text-slate-900 font-bold">¥{amount}</span>,
    },
    {
      title: 'Status',
      dataIndex: 'status',
      key: 'status',
      render: () => (
        <Tag color="success" className="border-0 bg-green-50 text-green-600 rounded-full px-3">
          {'Success'}
        </Tag>
      ),
    },
    {
      title: 'Time',
      dataIndex: 'time',
      key: 'time',
      render: (text: string) => <span className="text-slate-400 text-sm">{text}</span>,
    },
  ];

  const data = [
    {
      key: '1',
      id: 'ORDER_20241101_001',
      type: 'VIP Monthly Pass',
      amount: 29.9,
      status: 'success',
      time: '2024-11-01 12:30:00',
    },
    {
      key: '2',
      id: 'ORDER_20241005_008',
      type: 'Interview Boost Pack',
      amount: 9.9,
      status: 'success',
      time: '2024-10-05 09:15:00',
    },
  ];

  const [resumes, setResumes] = useState<ResumeInfo[]>([]);
  const [uploading, setUploading] = useState(false);
  const [loadingResumes, setLoadingResumes] = useState(false);

  // 获取简历列表
  const fetchResumes = useCallback(async () => {
    setLoadingResumes(true);
    try {
      const data: any = await apiClient.get(RESUME_API.LIST);
      setResumes(data?.resumes || []);
    } catch (err) {
      console.error('获取简历列表失败:', err);
    } finally {
      setLoadingResumes(false);
    }
  }, []);

  // 上传简历（异步处理）
  const handleUpload = async (file: File) => {
    if (resumes.length >= 3) {
      message.warning('Max 3 resumes allowed');
      return false;
    }

    const formData = new FormData();
    formData.append('resume', file);

    setUploading(true);
    try {
      const res: any = await apiClient.post(RESUME_API.UPLOAD, formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
        timeout: 30000, // 异步模式，30秒就足够了
      });

      // 检查返回状态
      if (res?.status === 'pending' || res?.status === 'processing') {
        message.info('Uploaded, parsing...');
        // 开始轮询检查状态
        if (res?.resume_id) {
          pollResumeStatus(res.resume_id);
        }
      } else {
        message.success('Resume uploaded successfully');
      }
      fetchResumes();
    } catch (err: any) {
      message.error(err?.message || 'Resume upload failed');
    } finally {
      setUploading(false);
    }
    return false; // 阻止默认上传行为
  };

  // 轮询简历解析状态
  const pollResumeStatus = async (resumeId: number) => {
    const maxAttempts = 60; // 最多轮询 60 次 (约 5 分钟)
    const interval = 5000; // 每 5 秒轮询一次

    let attempts = 0;
    const poll = async () => {
      attempts++;
      try {
        const data: any = await apiClient.get(RESUME_API.DETAIL(resumeId));
        const resume = data?.resume;

        if (resume?.status === 'completed') {
          message.success('Parsing completed');
          fetchResumes();
          return;
        } else if (resume?.status === 'failed') {
          message.error(`${'Parsing failed'}: ${resume?.error_msg || 'Unknown error'}`);
          fetchResumes();
          return;
        } else if (attempts < maxAttempts) {
          // 继续轮询
          setTimeout(poll, interval);
        } else {
          message.warning('Parsing timeout, please refresh later');
          fetchResumes();
        }
      } catch (err) {
        console.error('轮询简历状态失败:', err);
        if (attempts < maxAttempts) {
          setTimeout(poll, interval);
        }
      }
    };

    // 延迟开始轮询，让后端有时间处理
    setTimeout(poll, 3000);
  };

  // 设置默认简历
  const handleSetDefault = async (resumeId: number) => {
    try {
      await apiClient.post(RESUME_API.SET_DEFAULT, { resume_id: resumeId });
      message.success('Default resume set');
      fetchResumes();
    } catch (err: any) {
      message.error(err?.message || 'Failed to set default');
    }
  };

  // 删除简历
  const handleDelete = async (resumeId: number) => {
    try {
      await apiClient.delete(RESUME_API.DELETE(resumeId));
      message.success('Resume deleted');
      fetchResumes();
    } catch (err: any) {
      message.error(err?.message || 'Delete failed');
    }
  };

  // 格式化文件大小
  const formatFileSize = (bytes: number) => {
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
  };

  // 格式化时间
  const formatTime = (timestamp: number) => {
    const date = new Date(timestamp * 1000);
    return date.toLocaleDateString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    });
  };

  // 上传配置
  const uploadProps: UploadProps = {
    name: 'resume',
    accept: '.pdf',
    showUploadList: false,
    beforeUpload: handleUpload,
  };

  useEffect(() => {
    (async () => {
      try {
        const data: any = await apiClient.get(USER_API.GET_PROFILE);
        setProfile(data || null);
      } catch {}
    })();
    fetchResumes();
  }, [fetchResumes]);

  return (
    <div className="min-h-screen relative font-sans">
      {/* Decorative Background */}
      <div className="fixed top-0 right-0 w-[600px] h-[600px] bg-blue-50/60 rounded-full blur-[120px] -translate-y-1/2 translate-x-1/3 pointer-events-none z-0" />
      <div className="fixed bottom-0 left-0 w-[600px] h-[600px] bg-purple-50/60 rounded-full blur-[120px] translate-y-1/2 -translate-x-1/3 pointer-events-none z-0" />

      <div className="container mx-auto px-4 relative z-10">
        <div className="mb-8 animate-fade-in-up">
          <h1 className="text-3xl font-extrabold text-slate-900 tracking-tight">{'User Center'}</h1>
          <p className="text-slate-500 mt-2">{'Manage your personal info, resumes, and records'}</p>
        </div>

        <Row gutter={[24, 24]}>
          <Col xs={24} md={8} className="animate-fade-in-up" style={{ animationDelay: '0.1s' }}>
            <div className="bg-white rounded-3xl p-8 border border-slate-100 shadow-xl shadow-slate-200/50 relative overflow-hidden">
              <div className="absolute top-0 right-0 w-32 h-32 bg-gradient-to-br from-blue-50 to-indigo-50 rounded-bl-full -mr-8 -mt-8 z-0" />

              <div className="relative z-10 flex flex-col items-center text-center">
                <div className="p-1 rounded-full bg-gradient-to-br from-blue-100 to-indigo-100 mb-4">
                  <Avatar
                    size={80}
                    src="https://api.dicebear.com/7.x/adventurer/svg?seed=LB"
                    className="border-4 border-white shadow-md"
                  />
                </div>
                <h2 className="text-xl font-bold text-slate-800 mb-1">
                  {profile?.username || 'Not logged in'}
                </h2>
                <Tag
                  color="blue"
                  className="border-0 bg-blue-50 text-blue-600 px-3 py-1 rounded-full font-medium"
                >
                  {'Member'}
                </Tag>

                <div className="w-full mt-8 space-y-3 text-left bg-slate-50/50 rounded-2xl p-4 border border-slate-100">
                  <div className="flex justify-between items-center text-sm">
                    <span className="text-slate-500">{'Username'}</span>
                    <span className="font-medium text-slate-700">{profile?.username ?? '-'}</span>
                  </div>
                  <div className="flex justify-between items-center text-sm">
                    <span className="text-slate-500">{'Bound Email'}</span>
                    <span className="font-medium text-slate-700">{profile?.email ?? '-'}</span>
                  </div>
                </div>
              </div>
            </div>
          </Col>

          <Col xs={24} md={16} className="animate-fade-in-up" style={{ animationDelay: '0.2s' }}>
            <Row gutter={[16, 16]}>
              <Col span={24}>
                <div className="bg-white rounded-3xl p-8 border border-slate-100 shadow-xl shadow-slate-200/50">
                  <div className="flex items-center justify-between mb-6">
                    <div className="flex items-center gap-3">
                      <div className="w-10 h-10 rounded-xl bg-blue-50 flex items-center justify-center text-blue-600 text-xl">
                        <FileOutlined />
                      </div>
                      <div>
                        <h3 className="text-lg font-bold text-slate-800">{'My Resumes'}</h3>
                        <p className="text-xs text-slate-400">Uploaded {resumes.length}/3</p>
                      </div>
                    </div>
                  </div>

                  <Spin spinning={loadingResumes}>
                    {resumes.length > 0 ? (
                      <div className="space-y-3 mb-6">
                        {resumes.map((resume) => {
                          const latestId = resumes.length > 0
                            ? resumes.reduce((a, b) => (a.created_at > b.created_at ? a : b)).id
                            : -1;
                          const isDefault = resume.id === latestId;
                          return (                          <div
                            key={resume.id}
                            className="group flex items-center justify-between p-4 bg-slate-50 hover:bg-blue-50/50 border border-slate-100 hover:border-blue-100 rounded-2xl transition-all duration-300"
                          >
                            <div className="flex items-center gap-4">
                              <div className="w-10 h-10 bg-white rounded-xl flex items-center justify-center text-red-500 shadow-sm">
                                <FileOutlined className="text-lg" />
                              </div>
                              <div>
                                <div className="font-medium text-slate-700 group-hover:text-blue-700 transition-colors flex items-center gap-2">
                                  {resume.file_name}
                                  {/* 状态标签 */}
                                  {resume.status === 'pending' && (
                                    <Tag color="processing" className="ml-2 border-0">
                                      Pending
                                    </Tag>
                                  )}
                                  {resume.status === 'processing' && (
                                    <Tag
                                      color="processing"
                                      icon={<Spin size="small" className="mr-1" />}
                                      className="ml-2 border-0"
                                    >
                                      Processing...
                                    </Tag>
                                  )}
                                  {resume.status === 'failed' && (
                                    <Tag
                                      color="error"
                                      className="ml-2 border-0"
                                      title={resume.error_msg}
                                    >
                                      Failed
                                    </Tag>
                                  )}
                                </div>
                                <div className="text-xs text-slate-400 flex gap-2 mt-1">
                                  <span>{formatFileSize(resume.file_size)}</span>
                                  <span>•</span>
                                  <span>{formatTime(resume.created_at)}</span>
                                </div>
                              </div>
                            </div>
                            <Space>
                              {!isDefault && (
                                <Button
                                  type="text"
                                  size="small"
                                  onClick={() => handleSetDefault(resume.id)}
                                  className="opacity-0 group-hover:opacity-100 transition-opacity text-blue-600 bg-white shadow-sm border border-blue-100"
                                >
                                  {'Set Default'}
                                </Button>
                              )}
                              {isDefault ? (
                                <Tag color="blue" className="m-0 border-0 bg-blue-50 text-blue-600">
                                  Default
                                </Tag>
                              ) : null}
                              <Popconfirm
                                title={'Are you sure you want to delete this resume?'}
                                onConfirm={() => handleDelete(resume.id)}
                                okText="Yes"
                                cancelText="No"
                              >
                                <Button
                                  type="text"
                                  size="small"
                                  danger
                                  icon={<DeleteOutlined />}
                                  className="opacity-0 group-hover:opacity-100 transition-opacity bg-white shadow-sm border border-red-100"
                                />
                              </Popconfirm>
                            </Space>
                          </div>
                        );
                        })}
                      </div>
                    ) : null}

                    {/* 上传区域 */}
                    {resumes.length < 3 && (
                      <>
                        <Dragger
                          {...uploadProps}
                          disabled={uploading}
                          className="[&]:!border-0 [&]:!border-none rounded-2xl overflow-hidden"
                          style={{ padding: '40px 0', background: 'rgb(248 250 252)' }}
                        >
                          <p className="ant-upload-drag-icon text-blue-500 mb-4">
                            {uploading ? (
                              <Spin />
                            ) : (
                              <InboxOutlined style={{ fontSize: '48px', color: '#3b82f6' }} />
                            )}
                          </p>
                          <p className="text-base font-medium text-slate-700 mb-2">
                            {uploading ? 'Uploaded, parsing...' : 'Upload Resume'}
                          </p>
                          <p className="text-sm text-slate-400">{'PDF supported, max 10MB'}</p>
                        </Dragger>
                      </>
                    )}

                    {resumes.length >= 3 && (
                      <div className="text-center text-slate-400 py-8 bg-slate-50 rounded-2xl border border-dashed border-slate-200">
                        {'Max 3 resumes allowed'}
                      </div>
                    )}
                  </Spin>
                </div>
              </Col>
            </Row>
          </Col>
        </Row>
      </div>
    </div>
  );
}
