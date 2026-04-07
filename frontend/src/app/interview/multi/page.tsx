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
  message,
  Modal,
  Spin,
} from 'antd';
import { useState, useEffect, useCallback } from 'react';
import { useRouter } from 'next/navigation';

import { TeamOutlined, CheckCircleOutlined, FileOutlined, StarOutlined } from '@ant-design/icons';
import apiClient from '@/services/api/client';

const { Title, Paragraph, Text } = Typography;

interface ResumeInfo {
  id: number;
  file_name: string;
}

export default function MultiAgentInterviewPage() {
  const [form] = Form.useForm();
  const [selectedResumeId, setSelectedResumeId] = useState<number | null>(null);
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

  const features = [
    {
      title: 'Main Interviewer (Lead/Manager)',
      desc: 'Handle rhythm, values and behavior interview',
    },
    { title: 'Technical Expert (Architect)', desc: 'Hardcore questions, explore skill boundaries' },
    { title: 'Project Lead (PO/Lead)', desc: 'Project review, assessment of implementation' },
    { title: 'Dynamic Feedback', desc: 'Roles work together to simulate real pressure' },
  ];

  return (
    <div className="min-h-screen py-12 bg-slate-50/50">
      <div className="max-w-5xl mx-auto px-4">
        <div className="text-center mb-10">
          <Title level={2} className="!text-3xl !font-bold text-slate-800 !mb-3">
            {'Collaborative Interview'} ·{' '}
            <span className="text-orange-600">{'Multi-Agent Interview'}</span>
          </Title>
          <Paragraph className="text-slate-500 text-base max-w-2xl mx-auto">
            {
              'Face an interview panel (Main, Tech, Project) for the most realistic and high-pressure experience.'
            }
          </Paragraph>
        </div>

        <div className="bg-white rounded-[32px] shadow-[0_8px_30px_rgb(0,0,0,0.04)] border border-slate-100 p-8 md:p-10 relative overflow-hidden">
          <div className="absolute top-0 right-0 w-96 h-96 bg-orange-50/50 rounded-full blur-3xl -translate-y-1/2 translate-x-1/3 pointer-events-none" />
          <div className="absolute bottom-0 left-0 w-96 h-96 bg-amber-50/50 rounded-full blur-3xl translate-y-1/2 -translate-x-1/3 pointer-events-none" />

          <Row gutter={[48, 32]}>
            <Col
              xs={24}
              lg={9}
              className="relative z-10 border-b lg:border-b-0 lg:border-r border-slate-100 pb-8 lg:pb-0 lg:pr-10"
            >
              <div className="h-full flex flex-col">
                <div className="mb-6">
                  <div className="flex items-center gap-2 mb-2">
                    <TeamOutlined className="text-orange-500 text-xl" />
                    <Title level={4} className="!m-0 !font-bold text-slate-800">
                      {'Interview Panel'}
                    </Title>
                  </div>
                  <Text className="text-slate-400 text-sm">
                    {'Multi-role collab, all-round assessment'}
                  </Text>
                </div>

                <div className="space-y-6 flex-1">
                  {features.map((item, i) => (
                    <div key={i} className="flex gap-4 group">
                      <div className="mt-1 w-10 h-10 rounded-2xl bg-orange-50 text-orange-600 flex items-center justify-center flex-shrink-0 group-hover:bg-orange-500 group-hover:text-white transition-colors duration-300">
                        <StarOutlined className="text-lg" />
                      </div>
                      <div>
                        <div className="font-medium text-slate-700 mb-1 group-hover:text-orange-600 transition-colors">
                          {item.title}
                        </div>
                        <div className="text-sm text-slate-400 leading-relaxed">{item.desc}</div>
                      </div>
                    </div>
                  ))}
                </div>

                <div className="mt-8 pt-8 border-t border-slate-50 hidden lg:block">
                  <div className="bg-slate-50 rounded-xl p-4 text-xs text-slate-500 leading-relaxed">
                    {
                      '💡 Strategy: Different roles focus on different aspects, listen carefully to who is asking.'
                    }
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
                  <span className="w-1.5 h-6 bg-orange-500 rounded-full block"></span>
                  {'Panel Configuration'}
                </Title>

                <Form
                  form={form}
                  layout="vertical"
                  size="large"
                  initialValues={{ job: 'Senior Frontend Engineer', level: 'normal' }}
                  className="flex flex-col gap-4"
                >
                  <Form.Item
                    label={<span className="font-medium text-slate-700">{'Load Resume'}</span>}
                    name="resume_id"
                    rules={[{ required: true, message: 'Load resume for targeted interview' }]}
                    className="!mb-2"
                  >
                    <Select
                      placeholder={'Load resume for targeted interview'}
                      loading={loadingResumes}
                      disabled={starting}
                      className="!h-12"
                      variant="filled"
                      onChange={(value) => setSelectedResumeId(value)}
                      notFoundContent={
                        loadingResumes ? (
                          <Spin size="small" />
                        ) : (
                          'No resume, please upload in personal center'
                        )
                      }
                      options={resumes.map((r) => ({
                        value: r.id,
                        label: (
                          <div className="flex items-center gap-2">
                            <FileOutlined className="text-orange-500" />
                            <span className="text-slate-700">{r.file_name}</span>
                          </div>
                        ),
                      }))}
                    />
                  </Form.Item>

                  <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
                    <Form.Item
                      label={
                        <span className="font-medium text-slate-700">{'Interview Position'}</span>
                      }
                      name="job"
                      rules={[{ required: true, message: 'e.g., Senior Frontend Engineer' }]}
                      className="!mb-2"
                    >
                      <Input
                        placeholder={'e.g., Senior Frontend Engineer'}
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
                          { value: 'simple', label: 'Junior' },
                          { value: 'normal', label: 'Senior' },
                          { value: 'hard', label: 'Expert' },
                        ]}
                      />
                    </Form.Item>
                  </div>

                  <Form.Item
                    label={<span className="font-medium text-slate-700">{'Target Company'}</span>}
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
                      className="!h-14 !text-lg !font-medium !rounded-xl bg-gradient-to-r from-orange-500 to-amber-600 hover:!from-orange-600 hover:!to-amber-700 border-0 shadow-lg shadow-orange-500/30 hover:shadow-orange-500/40 transition-all duration-300 transform hover:-translate-y-0.5"
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
                          type: 'multi-agent',
                          domain: 'eino',
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
                        router.push('/interview/multi/start');
                      }}
                    >
                      {'Enter Interview Room'}
                    </Button>
                    <div className="text-center text-slate-400 text-sm mt-4">
                      {'Simulate real group interview process'}
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
