'use client';

import {
  Typography,
  Row,
  Col,
  Card as AntCard,
  Form,
  Select,
  Input,
  Button,
  Tag,
  message,
  Modal,
  Spin,
  Alert,
} from 'antd';
import { useState, useEffect, useCallback } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { CheckCircleOutlined, FileOutlined } from '@ant-design/icons';
import apiClient from '@/services/api/client';
import { API_BASE_URL } from '@/config/api';

const { Title, Paragraph, Text } = Typography;

// 简历信息类型
interface ResumeInfo {
  id: number;
  file_name: string;
}

export default function SocialInterviewPage() {
  const [form] = Form.useForm();
  const [selectedResumeId, setSelectedResumeId] = useState<number | null>(null);
  const [resumes, setResumes] = useState<ResumeInfo[]>([]);
  const [loadingResumes, setLoadingResumes] = useState(false);
  const [starting, setStarting] = useState(false);
  const [modelConfigured, setModelConfigured] = useState<boolean | null>(null);
  const [checkingConfig, setCheckingConfig] = useState<boolean>(false);
  const [showNoResumeModal, setShowNoResumeModal] = useState(false);
  const router = useRouter();

  // 获取用户简历列表
  const fetchResumes = useCallback(async () => {
    setLoadingResumes(true);
    try {
      const data: any = await apiClient.get('/resume/list');
      const list = data?.resumes || [];
      setResumes(list);
      if (list.length === 0) {
        setShowNoResumeModal(true);
      }
    } catch (err) {
      console.error('获取简历列表失败:', err);
    } finally {
      setLoadingResumes(false);
    }
  }, []);

  useEffect(() => {
    fetchResumes();
    const token = typeof window !== 'undefined' ? localStorage.getItem('token') : null;
    setCheckingConfig(true);
    fetch(`${API_BASE_URL}/user/model/check`, {
      method: 'GET',
      headers: {
        Authorization: token ? `Bearer ${token}` : '',
        'X-Auth-Token': token || '',
      },
    })
      .then(async (res) => {
        const data = await res.json().catch(() => null);
        const configured = !!(data && data.data && data.data.configured);
        setModelConfigured(configured);
      })
      .catch(() => {
        setModelConfigured(false);
      })
      .finally(() => {
        setCheckingConfig(false);
      });
  }, [fetchResumes]);

  return (
    <div className="min-h-screen py-12 bg-slate-50/50">
      <div className="max-w-5xl mx-auto px-4">
        {/* Header */}
        <div className="text-center mb-10">
          <Title level={2} className="!text-3xl !font-bold text-slate-800 !mb-3">
            综合面试 · <span className="text-blue-600">社招简历面试</span>
          </Title>
          <Paragraph className="text-slate-500 text-base max-w-2xl mx-auto">
            在综合面试模式中，系统会围绕你的简历、项目经历与岗位胜任力，从技术基础、项目落地、设计能力到沟通协作，构建环环追问的真实面试场景。
          </Paragraph>
        </div>

        {/* Main Card */}
        <div className="bg-white rounded-[32px] shadow-[0_8px_30px_rgb(0,0,0,0.04)] border border-slate-100 p-8 md:p-10 relative overflow-hidden">
          {/* Decorative Background - Blue theme for Social */}
          <div className="absolute top-0 right-0 w-96 h-96 bg-blue-50/50 rounded-full blur-3xl -translate-y-1/2 translate-x-1/3 pointer-events-none" />
          <div className="absolute bottom-0 left-0 w-96 h-96 bg-indigo-50/50 rounded-full blur-3xl translate-y-1/2 -translate-x-1/3 pointer-events-none" />

          <Row gutter={[48, 32]}>
            {/* Left Side: Info & Features */}
            <Col
              xs={24}
              lg={9}
              className="relative z-10 border-b lg:border-b-0 lg:border-r border-slate-100 pb-8 lg:pb-0 lg:pr-10"
            >
              <div className="h-full flex flex-col">
                <div className="mb-6">
                  <Title level={4} className="!mb-2 !font-bold text-slate-800">
                    社招核心考察点
                  </Title>
                  <Text className="text-slate-400 text-sm">面向有经验的工程师，深度挖掘</Text>
                </div>

                <div className="space-y-6 flex-1">
                  {[
                    { title: '技术本质逻辑', desc: '深挖底层原理，构建环环追问' },
                    { title: '设计与决策', desc: '结合项目落地难点，考察架构能力' },
                    { title: '沟通与展示', desc: '从被动回答到主动展示，提升影响力' },
                    { title: '职级能力定位', desc: '对标大厂职级体系，精准定位' },
                  ].map((t, i) => (
                    <div key={i} className="flex gap-4 group">
                      <div className="mt-1 w-10 h-10 rounded-2xl bg-blue-50 text-blue-600 flex items-center justify-center flex-shrink-0 group-hover:bg-blue-500 group-hover:text-white transition-colors duration-300">
                        <CheckCircleOutlined className="text-lg" />
                      </div>
                      <div>
                        <div className="font-medium text-slate-700 mb-1 group-hover:text-blue-600 transition-colors">
                          {t.title}
                        </div>
                        <div className="text-sm text-slate-400 leading-relaxed">{t.desc}</div>
                      </div>
                    </div>
                  ))}
                </div>

                <div className="mt-8 pt-8 border-t border-slate-50 hidden lg:block">
                  <div className="bg-slate-50 rounded-xl p-4 text-xs text-slate-500 leading-relaxed">
                    💡 提示：建议上传包含详细项目难点与解决方案的简历，以获得更具挑战性的面试体验。
                  </div>
                </div>
              </div>
            </Col>

            {/* Right Side: Form */}
            <Col xs={24} lg={15} className="relative z-10">
              <div className="lg:pl-4">
                <Title
                  level={4}
                  className="!mb-8 !font-bold text-slate-800 flex items-center gap-2"
                >
                  <span className="w-1.5 h-6 bg-blue-500 rounded-full block"></span>
                  面试配置
                </Title>

                <Form
                  form={form}
                  layout="vertical"
                  size="large"
                  initialValues={{ job: 'Java后端开发', level: '简单' }}
                  className="flex flex-col gap-4"
                >
                  <Form.Item
                    label={<span className="font-medium text-slate-700">选择简历</span>}
                    name="resume_id"
                    rules={[{ required: true, message: '请选择简历' }]}
                    className="!mb-2"
                  >
                    <Select
                      placeholder="请选择已上传的简历"
                      loading={loadingResumes}
                      disabled={starting}
                      className="!h-12"
                      variant="filled"
                      onChange={(value) => setSelectedResumeId(value)}
                      notFoundContent={
                        loadingResumes ? <Spin size="small" /> : '暂无简历，请先在个人中心上传'
                      }
                      options={resumes.map((r) => ({
                        value: r.id,
                        label: (
                          <div className="flex items-center gap-2">
                            <FileOutlined className="text-blue-500" />
                            <span className="text-slate-700">{r.file_name}</span>
                          </div>
                        ),
                      }))}
                    />
                  </Form.Item>

                  <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
                    <Form.Item
                      label={<span className="font-medium text-slate-700">岗位意向</span>}
                      name="job"
                      className="!mb-2"
                    >
                      <Input
                        placeholder="如：Java后端开发"
                        className="!h-12 !bg-slate-50 border-slate-200 hover:bg-white focus:bg-white transition-colors"
                      />
                    </Form.Item>

                    <Form.Item
                      label={<span className="font-medium text-slate-700">难度等级</span>}
                      name="level"
                      rules={[{ required: true, message: '请选择难度等级' }]}
                      className="!mb-2"
                    >
                      <Select
                        className="!h-12"
                        variant="filled"
                        options={[
                          { value: '简单', label: '简单' },
                          { value: '中等', label: '中等' },
                          { value: '复杂', label: '复杂' },
                        ]}
                      />
                    </Form.Item>
                  </div>

                  <Form.Item
                    label={<span className="font-medium text-slate-700">目标公司（可选）</span>}
                    name="company_name"
                    className="!mb-6"
                  >
                    <Input
                      placeholder="如：字节跳动"
                      maxLength={100}
                      className="!h-12 !bg-slate-50 border-slate-200 hover:bg-white focus:bg-white transition-colors"
                    />
                  </Form.Item>

                  <div className="mt-2">
                    {!checkingConfig && modelConfigured === false && (
                      <Alert
                        message="模型未配置"
                        description={
                          <span>
                            请去{' '}
                            <Link href="/user/models" className="text-blue-500 underline">
                              用户模型页面
                            </Link>{' '}
                            配置模型
                          </span>
                        }
                        type="warning"
                        showIcon
                        className="mb-6 rounded-xl"
                      />
                    )}
                    {checkingConfig && (
                      <div className="mb-4 flex justify-center">
                        <Tag color="default" className="px-3 py-1 rounded-full">
                          正在检查模型配置...
                        </Tag>
                      </div>
                    )}

                    <Button
                      type="primary"
                      block
                      size="large"
                      className="!h-14 !text-lg !font-medium !rounded-xl bg-gradient-to-r from-blue-500 to-indigo-600 hover:!from-blue-600 hover:!to-indigo-700 border-0 shadow-lg shadow-blue-500/30 hover:shadow-blue-500/40 transition-all duration-300 transform hover:-translate-y-0.5"
                      loading={starting}
                      disabled={starting || checkingConfig || modelConfigured === false}
                      onClick={async () => {
                        try {
                          await form.validateFields();
                        } catch (e) {
                          message.error('请完善表单后再开始面试');
                          return;
                        }
                        if (!modelConfigured) {
                          message.error('未配置模型，无法开始面试');
                          return;
                        }
                        const values = form.getFieldsValue();
                        const params = {
                          type: '综合面试',
                          domain: '社招简历面试', // Updated domain
                          difficulty: values.level,
                          position_name: values.job || '',
                          company_name: String(values.company_name || ''),
                          resume_id: values.resume_id,
                        };
                        (window as any).__interviewParams = { ...params };
                        try {
                          sessionStorage.setItem('interviewParams', JSON.stringify(params));
                        } catch {}
                        setStarting(true);
                        router.push('/interview/social/start');
                      }}
                    >
                      开始面试
                    </Button>
                    <div className="text-center text-slate-400 text-sm mt-4">
                      社招模式将包含更深度的架构设计与场景题追问
                    </div>
                  </div>
                </Form>
              </div>
            </Col>
          </Row>
        </div>
      </div>
      <Modal
        open={showNoResumeModal}
        title="温馨提示"
        footer={null}
        onCancel={() => setShowNoResumeModal(false)}
        centered
      >
        <div className="text-center py-6">
          <div className="mb-4 text-slate-600 text-lg">检测到您尚未上传简历，无法进行面试。</div>
          <div className="mb-8 text-slate-500">
            请前往个人中心上传您的简历，AI 将根据您的简历内容生成针对性的面试题目。
          </div>
          <Button
            type="primary"
            size="large"
            onClick={() => router.push('/user/center')}
            className="w-full bg-indigo-600 hover:bg-indigo-500"
          >
            前往上传简历
          </Button>
        </div>
      </Modal>
    </div>
  );
}
