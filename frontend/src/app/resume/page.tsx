'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import {
  Typography,
  Row,
  Col,
  Card as AntCard,
  Form,
  Select,
  Input,
  Button,
  message,
  Modal,
} from 'antd';
import {
  FileTextOutlined,
  RocketOutlined,
  ThunderboltOutlined,
  ReadOutlined,
  CheckCircleOutlined,
} from '@ant-design/icons';
import apiClient from '@/services/api/client';

const { Title, Paragraph, Text } = Typography;

interface Resume {
  id: number;
  file_name: string;
}

export default function ResumePressPage() {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [resumes, setResumes] = useState<Resume[]>([]);
  const [showNoResumeModal, setShowNoResumeModal] = useState(false);
  const router = useRouter();

  useEffect(() => {
    const fetchResumes = async () => {
      try {
        const data: any = await apiClient.get('/resume/list');
        if (data && data.resumes) {
          setResumes(data.resumes);
          if (data.resumes.length > 0) {
            // Check if default exists or pick first
            const defaultResume = data.resumes.find((r: any) => r.is_default) || data.resumes[0];
            form.setFieldsValue({ resume_id: defaultResume.id });
          } else {
            setShowNoResumeModal(true);
          }
        } else {
          setShowNoResumeModal(true);
        }
      } catch (e) {
        console.error('Failed to fetch resumes:', e);
        // message.error('获取简历列表失败'); // Optional: avoid spamming error on load
      }
    };
    fetchResumes();
  }, [form]);

  const onFinish = async (values: any) => {
    setLoading(true);
    try {
      const payload = {
        resume_id: values.resume_id,
        prediction_type: values.prediction_type,
        language: values.language,
        job_title: values.job,
        difficulty: values.level,
        company_name: values.company_name,
      };

      await apiClient.post('/prediction/start', payload, {
        timeout: 180000, // 3 分钟超时
      });
      message.success('开始生成押题...');
      router.push('/user/press');
    } catch (e: any) {
      message.error(e?.message || '提交失败，请重试');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen relative font-sans pb-12">
      {/* Decorative Background */}
      <div className="fixed top-0 right-0 w-[600px] h-[600px] bg-indigo-50/60 rounded-full blur-[120px] -translate-y-1/2 translate-x-1/3 pointer-events-none z-0" />
      <div className="fixed bottom-0 left-0 w-[600px] h-[600px] bg-purple-50/60 rounded-full blur-[120px] translate-y-1/2 -translate-x-1/3 pointer-events-none z-0" />

      <div className="container mx-auto px-4 relative z-10">
        <div className="mb-10 animate-fade-in-up pt-8">
          <h1 className="text-3xl font-extrabold text-slate-900 tracking-tight flex items-center gap-3">
            <RocketOutlined className="text-indigo-600" />
            简历押题
          </h1>
          <p className="text-slate-500 mt-2 ml-11 max-w-2xl">
            基于您的简历内容与求职意向，利用大模型技术为您精准预测面试题目，助您从容应对每一场面试。
          </p>
        </div>

        <Row gutter={[32, 32]} className="animate-fade-in-up" style={{ animationDelay: '0.1s' }}>
          <Col xs={24} lg={16}>
            <AntCard
              className="rounded-3xl border-slate-100 shadow-xl shadow-slate-200/50 overflow-hidden"
              styles={{ body: { padding: 40 } }}
            >
              <div className="bg-indigo-50/50 rounded-2xl p-5 mb-8 border border-indigo-100 flex items-start gap-3">
                <CheckCircleOutlined className="text-indigo-600 mt-1" />
                <div className="text-sm text-indigo-900">
                  <div className="font-bold mb-1">温馨提示</div>
                  <ul className="list-disc pl-4 space-y-1 text-indigo-800/80">
                    <li>押题会根据你的简历内容生成，一次约提供至少20道题。</li>
                    <li>添加/切换简历内容会影响押题的范围，系统会为你保留历史题目。</li>
                  </ul>
                </div>
              </div>

              <Form
                form={form}
                layout="vertical"
                onFinish={onFinish}
                initialValues={{
                  language: 'Java',
                  job: '后端开发',
                  level: '进阶',
                  prediction_type: '校招',
                }}
                className="flex flex-col gap-4"
              >
                <Form.Item
                  label={<span className="font-bold text-slate-700">选择押题的简历</span>}
                  name="resume_id"
                  rules={[{ required: true, message: '请选择简历' }]}
                >
                  <Select
                    size="large"
                    variant="filled"
                    className="!h-12"
                    options={resumes.map((r) => ({ value: r.id, label: r.file_name }))}
                    placeholder={resumes.length === 0 ? '加载中...' : '请选择简历'}
                    popupMatchSelectWidth={false}
                  />
                </Form.Item>

                <Form.Item
                  label={<span className="font-bold text-slate-700">面试类型</span>}
                  name="prediction_type"
                  rules={[{ required: true, message: '请选择面试类型' }]}
                >
                  <Select
                    size="large"
                    variant="filled"
                    className="!h-12"
                    options={[
                      { value: '校招', label: '校招' },
                      { value: '社招', label: '社招' },
                    ]}
                  />
                </Form.Item>

                <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                  <Form.Item
                    label={<span className="font-bold text-slate-700">编程语言</span>}
                    name="language"
                    rules={[{ required: true, message: '请选择语言' }]}
                  >
                    <Select
                      size="large"
                      variant="filled"
                      className="!h-12"
                      options={[
                        { value: 'Java', label: 'Java' },
                        { value: 'Golang', label: 'Golang' },
                        { value: 'Python', label: 'Python' },
                        { value: 'C++', label: 'C++' },
                        { value: 'Frontend', label: '前端(JS/TS)' },
                      ]}
                    />
                  </Form.Item>

                  <Form.Item
                    label={<span className="font-bold text-slate-700">岗位意向</span>}
                    name="job"
                    rules={[{ required: true, message: '请输入岗位意向' }]}
                  >
                    <Input
                      size="large"
                      variant="filled"
                      className="!h-12 !bg-slate-50 hover:!bg-slate-100 focus:!bg-white border-transparent hover:border-indigo-300 focus:border-indigo-500"
                      placeholder="如：Java后端开发"
                    />
                  </Form.Item>

                  <Form.Item
                    label={<span className="font-bold text-slate-700">难度等级</span>}
                    name="level"
                    rules={[{ required: true, message: '请选择难度' }]}
                  >
                    <Select
                      size="large"
                      variant="filled"
                      className="!h-12"
                      options={[
                        { value: '入门', label: '入门' },
                        { value: '中级', label: '中级' },
                        { value: '进阶', label: '进阶' },
                        { value: '专家', label: '专家' },
                      ]}
                    />
                  </Form.Item>

                  <Form.Item
                    label={<span className="font-bold text-slate-700">目标公司</span>}
                    name="company_name"
                    rules={[{ required: true, message: '请输入目标公司' }]}
                  >
                    <Input
                      size="large"
                      variant="filled"
                      className="!h-12 !bg-slate-50 hover:!bg-slate-100 focus:!bg-white border-transparent hover:border-indigo-300 focus:border-indigo-500"
                      placeholder="如：字节跳动"
                    />
                  </Form.Item>
                </div>

                <div className="mt-8 pt-6 border-t border-slate-100">
                  <Button
                    type="primary"
                    htmlType="submit"
                    loading={loading}
                    size="large"
                    icon={<ThunderboltOutlined />}
                    className="w-full h-14 text-lg font-bold rounded-xl bg-indigo-600 hover:bg-indigo-500 shadow-lg shadow-indigo-200"
                  >
                    开始简历押题
                  </Button>
                  <div className="text-center text-slate-400 text-sm mt-4">
                    首次免费押题 20 题题量 · 生成耗时约 30 秒
                  </div>
                </div>
              </Form>
            </AntCard>
          </Col>

          <Col xs={24} lg={8}>
            <div className="flex flex-col gap-6 sticky top-8">
              {[
                {
                  title: '快速定位',
                  desc: '深入解析简历条目，生成对应问答清单，直击考点。',
                  icon: <FileTextOutlined className="text-2xl text-blue-500" />,
                  bg: 'bg-blue-50',
                  border: 'border-blue-100',
                },
                {
                  title: '快速剖析',
                  desc: '结合岗位要求与项目经历，输出结构化追问路径。',
                  icon: <ThunderboltOutlined className="text-2xl text-amber-500" />,
                  bg: 'bg-amber-50',
                  border: 'border-amber-100',
                },
                {
                  title: '直接学习',
                  desc: '对题清单搭配参考答案与延伸阅读，立即提升。',
                  icon: <ReadOutlined className="text-2xl text-emerald-500" />,
                  bg: 'bg-emerald-50',
                  border: 'border-emerald-100',
                },
              ].map((item, idx) => (
                <div
                  key={idx}
                  className="bg-white p-6 rounded-2xl border border-slate-100 shadow-lg shadow-slate-100/50 hover:-translate-y-1 transition-all duration-300"
                >
                  <div
                    className={`w-12 h-12 ${item.bg} rounded-xl flex items-center justify-center mb-4 border ${item.border}`}
                  >
                    {item.icon}
                  </div>
                  <h3 className="text-lg font-bold text-slate-800 mb-2">{item.title}</h3>
                  <p className="text-slate-500 leading-relaxed m-0">{item.desc}</p>
                </div>
              ))}
            </div>
          </Col>
        </Row>
      </div>
      <Modal
        open={showNoResumeModal}
        title="温馨提示"
        footer={null}
        onCancel={() => setShowNoResumeModal(false)}
        centered
      >
        <div className="text-center py-6">
          <div className="mb-4 text-slate-600 text-lg">检测到您尚未上传简历，无法进行押题。</div>
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
