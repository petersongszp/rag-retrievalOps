'use client';

import { Typography, Row, Col, Form, Select, Input, Button, message, Spin, Modal } from 'antd';
import { useState, useEffect, useCallback } from 'react';
import { useRouter } from 'next/navigation';

import { CheckCircleOutlined, FileOutlined } from '@ant-design/icons';
import apiClient from '@/services/api/client';

const { Title, Paragraph, Text } = Typography;

interface ResumeInfo {
  id: number;
  file_name: string;
}

export default function CampusInterviewPage() {
  const [form] = Form.useForm();
  const [, setSelectedResumeId] = useState<number | null>(null);
  const [resumes, setResumes] = useState<ResumeInfo[]>([]);
  const [loadingResumes, setLoadingResumes] = useState(false);
  const [starting, setStarting] = useState(false);
  const [showNoResumeModal, setShowNoResumeModal] = useState(false);
  const router = useRouter();

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
  }, [fetchResumes]);

  return (
    <div className="min-h-screen py-12 bg-slate-50/50">
      <div className="max-w-5xl mx-auto px-4">
        <div className="text-center mb-10">
          <Title level={2} className="!text-3xl !font-bold text-slate-800 !mb-3">
            {'Comprehensive Interview'} ·{' '}
            <span className="text-green-600">{'Campus Interview'}</span>
          </Title>
          <Paragraph className="text-slate-500 text-base max-w-2xl mx-auto">
            {'Targeting campus requirements, digging into resume items and technical foundation.'}
          </Paragraph>
        </div>

        <div className="bg-white rounded-[32px] shadow-[0_8px_30px_rgb(0,0,0,0.04)] border border-slate-100 p-8 md:p-10 relative overflow-hidden">
          <div className="absolute top-0 right-0 w-96 h-96 bg-green-50/50 rounded-full blur-3xl -translate-y-1/2 translate-x-1/3 pointer-events-none" />
          <div className="absolute bottom-0 left-0 w-96 h-96 bg-blue-50/50 rounded-full blur-3xl translate-y-1/2 -translate-x-1/3 pointer-events-none" />

          <Row gutter={[48, 32]}>
            <Col
              xs={24}
              lg={9}
              className="relative z-10 border-b lg:border-b-0 lg:border-r border-slate-100 pb-8 lg:pb-0 lg:pr-10"
            >
              <div className="h-full flex flex-col">
                <div className="mb-6">
                  <Title level={4} className="!mb-2 !font-bold text-slate-800">
                    {'Core Dimensions'}
                  </Title>
                  <Text className="text-slate-400 text-sm">
                    {'System will focus on these abilities'}
                  </Text>
                </div>

                <div className="space-y-6 flex-1">
                  {[
                    { title: 'System Foundation', desc: 'Focus on fundamentals and logic' },
                    { title: 'Chained Expression', desc: 'Clear logic path for standard roles' },
                    { title: 'Knowledge Mastery', desc: 'Deep verification of learning' },
                    { title: 'Potential & Growth', desc: 'Structured problem solving' },
                  ].map((item, i) => (
                    <div key={i} className="flex gap-4 group">
                      <div className="mt-1 w-10 h-10 rounded-2xl bg-green-50 text-green-600 flex items-center justify-center flex-shrink-0 group-hover:bg-green-500 group-hover:text-white transition-colors duration-300">
                        <CheckCircleOutlined className="text-lg" />
                      </div>
                      <div>
                        <div className="font-medium text-slate-700 mb-1 group-hover:text-green-600 transition-colors">
                          {item.title}
                        </div>
                        <div className="text-sm text-slate-400 leading-relaxed">{item.desc}</div>
                      </div>
                    </div>
                  ))}
                </div>

                <div className="mt-8 pt-8 border-t border-slate-50 hidden lg:block">
                  <div className="bg-slate-50 rounded-xl p-4 text-xs text-slate-500 leading-relaxed">
                    {'💡 Tip: A well-prepared resume helps AI generate better questions.'}
                  </div>
                </div>
              </div>
            </Col>

            <Col xs={24} lg={15} className="relative z-10">
              <div className="lg:pl-4">
                <Title
                  level={4}
                  className="!mb-8 !font-bold text-slate-800 flex items-center gap-2"
                >
                  <span className="w-1.5 h-6 bg-green-500 rounded-full block"></span>
                  {'Interview Configuration'}
                </Title>

                <Form
                  form={form}
                  layout="vertical"
                  size="large"
                  initialValues={{ job: 'Java Backend Developer', level: '简单' }}
                  className="flex flex-col gap-4"
                >
                  <Form.Item
                    label={<span className="font-medium text-slate-700">{'Select Resume'}</span>}
                    name="resume_id"
                    rules={[{ required: true, message: 'Please select a resume' }]}
                    className="!mb-2"
                  >
                    <Select
                      placeholder={'Please select a resume'}
                      loading={loadingResumes}
                      disabled={starting}
                      className="!h-12"
                      variant="filled"
                      onChange={(value) => setSelectedResumeId(value)}
                      notFoundContent={
                        loadingResumes ? (
                          <Spin size="small" />
                        ) : (
                          'No resume detected, cannot start interview.'
                        )
                      }
                      options={resumes.map((r) => ({
                        value: r.id,
                        label: (
                          <div className="flex items-center gap-2">
                            <FileOutlined className="text-green-500" />
                            <span className="text-slate-700">{r.file_name}</span>
                          </div>
                        ),
                      }))}
                    />
                  </Form.Item>

                  <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
                    <Form.Item
                      label={<span className="font-medium text-slate-700">{'Job Intention'}</span>}
                      name="job"
                      className="!mb-2"
                    >
                      <Input
                        placeholder={'e.g., Java Backend Developer'}
                        className="!h-12 !bg-slate-50 border-slate-200 hover:bg-white focus:bg-white transition-colors"
                      />
                    </Form.Item>

                    <Form.Item
                      label={
                        <span className="font-medium text-slate-700">{'Difficulty Level'}</span>
                      }
                      name="level"
                      rules={[{ required: true, message: 'Difficulty Level' }]}
                      className="!mb-2"
                    >
                      <Select
                        className="!h-12"
                        variant="filled"
                        options={[
                          { value: '简单', label: 'Simple' },
                          { value: '中等', label: 'Normal' },
                          { value: '复杂', label: 'Hard' },
                        ]}
                      />
                    </Form.Item>
                  </div>

                  <Form.Item
                    label={
                      <span className="font-medium text-slate-700">
                        {'Target Company (Optional)'}
                      </span>
                    }
                    name="company_name"
                    className="!mb-6"
                  >
                    <Input
                      placeholder={'e.g., ByteDance'}
                      maxLength={100}
                      className="!h-12 !bg-slate-50 border-slate-200 hover:bg-white focus:bg-white transition-colors"
                    />
                  </Form.Item>

                  <div className="mt-2">
                    <Button
                      type="primary"
                      block
                      size="large"
                      className="!h-14 !text-lg !font-medium !rounded-xl bg-gradient-to-r from-green-500 to-emerald-600 hover:!from-green-600 hover:!to-emerald-700 border-0 shadow-lg shadow-green-500/30 hover:shadow-green-500/40 transition-all duration-300 transform hover:-translate-y-0.5"
                      loading={starting}
                      disabled={starting}
                      onClick={async () => {
                        try {
                          await form.validateFields();
                        } catch (e) {
                          message.error('Please complete the form before starting interview');
                          return;
                        }
                        const values = form.getFieldsValue();
                        const params = {
                          type: '综合面试',
                          domain: '校招简历面试',
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
                        router.push('/interview/campus/start');
                      }}
                    >
                      {'Start Interview'}
                    </Button>
                    <div className="text-center text-slate-400 text-sm mt-4">
                      {'Cost approx 20x AI practice · Auto follow-up'}
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
        title={'Friendly Reminder'}
        footer={null}
        onCancel={() => setShowNoResumeModal(false)}
        centered
      >
        <div className="text-center py-6">
          <div className="mb-4 text-slate-600 text-lg">
            {'No resume detected, cannot start interview.'}
          </div>
          <div className="mb-8 text-slate-500">
            {
              'Please go to personal center to upload your resume, AI will generate questions based on it.'
            }
          </div>
          <Button
            type="primary"
            size="large"
            onClick={() => router.push('/user/center')}
            className="w-full bg-indigo-600 hover:bg-indigo-500"
          >
            {'Go to Upload Resume'}
          </Button>
        </div>
      </Modal>
    </div>
  );
}
