'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';

import { Typography, Row, Col, Card as AntCard, Form, Select, Button, message } from 'antd';
import { CheckCircleOutlined } from '@ant-design/icons';

const { Title, Paragraph, Text } = Typography;

export default function SpecialInterviewPage() {
  const [stack, setStack] = useState<string>('Go');
  const [starting, setStarting] = useState(false);
  const [form] = Form.useForm();
  const router = useRouter();

  const handleStart = async () => {
    try {
      const values = await form.validateFields();
      setStarting(true);

      const params = {
        type: 'special',
        domain: values.stack,
        difficulty: values.level,
      };

      (window as any).__interviewParams = { ...params };
      try {
        sessionStorage.setItem('interviewParams', JSON.stringify(params));
      } catch {}

      router.push('/interview/special/start');
    } catch (e) {
      message.error('Please complete the form before starting interview');
    } finally {
      setStarting(false);
    }
  };

  const GROUPED_OPTIONS = [
    {
      label: 'Languages',
      options: [
        { value: 'Java', label: 'Java' },
        { value: 'Go', label: 'Go' },
        { value: 'C/C++', label: 'C/C++' },
        { value: 'Rust', label: 'Rust' },
        { value: 'PHP', label: 'PHP' },
        { value: 'Node.js', label: 'Node.js' },
      ],
    },
    {
      label: 'Backend Components',
      options: [
        { value: 'Redis', label: 'Redis' },
        { value: 'MySQL', label: 'MySQL' },
        { value: 'Kafka', label: 'Kafka' },
        { value: 'MongoDB', label: 'MongoDB' },
      ],
    },
    {
      label: 'Cloud Native & Ops',
      options: [
        { value: 'Docker', label: 'Docker' },
        { value: 'Kubernetes', label: 'Kubernetes' },
        { value: 'Nginx', label: 'Nginx' },
      ],
    },
    {
      label: 'CS Fundamentals',
      options: [
        { value: 'os', label: 'Operating Systems' },
        { value: 'network', label: 'Computer Network' },
        { value: 'dsa', label: 'Data Structures & Algos' },
      ],
    },
  ];

  const features = [
    { title: 'Precision', desc: 'Directly hit core high-frequency points' },
    { title: 'Chain Analysis', desc: 'Systematic knowledge map' },
    { title: 'High Density', desc: 'Quickly locate capability boundaries' },
    { title: 'Simulation', desc: 'Restore real high-pressure environment' },
  ];

  return (
    <div className="min-h-screen py-12 bg-slate-50/50">
      <div className="max-w-5xl mx-auto px-4">
        <div className="text-center mb-10">
          <Title level={2} className="!text-3xl !font-bold text-slate-800 !mb-3">
            {'Specialized Interview'} · <span className="text-purple-600">{stack}</span>
          </Title>
          <Paragraph className="text-slate-500 text-base max-w-2xl mx-auto">
            {'Focus on high-frequency questions and deep digging for specific tech stacks.'}
          </Paragraph>
        </div>

        <div className="bg-white rounded-[32px] shadow-[0_8px_30px_rgb(0,0,0,0.04)] border border-slate-100 p-8 md:p-10 relative overflow-hidden">
          <div className="absolute top-0 right-0 w-96 h-96 bg-purple-50/50 rounded-full blur-3xl -translate-y-1/2 translate-x-1/3 pointer-events-none" />
          <div className="absolute bottom-0 left-0 w-96 h-96 bg-pink-50/50 rounded-full blur-3xl translate-y-1/2 -translate-x-1/3 pointer-events-none" />

          <Row gutter={[48, 32]}>
            <Col
              xs={24}
              lg={9}
              className="relative z-10 border-b lg:border-b-0 lg:border-r border-slate-100 pb-8 lg:pb-0 lg:pr-10"
            >
              <div className="h-full flex flex-col">
                <div className="mb-6">
                  <Title level={4} className="!mb-2 !font-bold text-slate-800">
                    {'Specialized Advantages'}
                  </Title>
                  <Text className="text-slate-400 text-sm">
                    {'Deep reinforcement for specific stacks'}
                  </Text>
                </div>

                <div className="space-y-6 flex-1">
                  {features.map((item, i) => (
                    <div key={i} className="flex gap-4 group">
                      <div className="mt-1 w-10 h-10 rounded-2xl bg-purple-50 text-purple-600 flex items-center justify-center flex-shrink-0 group-hover:bg-purple-500 group-hover:text-white transition-colors duration-300">
                        <CheckCircleOutlined className="text-lg" />
                      </div>
                      <div>
                        <div className="font-medium text-slate-700 mb-1 group-hover:text-purple-600 transition-colors">
                          {item.title}
                        </div>
                        <div className="text-sm text-slate-400 leading-relaxed">{item.desc}</div>
                      </div>
                    </div>
                  ))}
                </div>

                <div className="mt-8 pt-8 border-t border-slate-50 hidden lg:block">
                  <div className="bg-slate-50 rounded-xl p-4 text-xs text-slate-500 leading-relaxed">
                    {'💡 Tip: Great for point breakthroughs or final reviews.'}
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
                  <span className="w-1.5 h-6 bg-purple-500 rounded-full block"></span>
                  {'Interview Configuration'}
                </Title>

                <Form
                  form={form}
                  layout="vertical"
                  size="large"
                  initialValues={{ stack: stack, level: 'simple' }}
                  className="flex flex-col gap-4"
                >
                  <Form.Item
                    label={<span className="font-medium text-slate-700">{'Specialization'}</span>}
                    name="stack"
                    className="!mb-2"
                  >
                    <Select
                      popupMatchSelectWidth={false}
                      className="!h-12"
                      variant="filled"
                      options={GROUPED_OPTIONS}
                      value={stack}
                      onChange={(v) => setStack(v)}
                      placeholder={'Select tech stack'}
                    />
                  </Form.Item>

                  <Form.Item
                    label={<span className="font-medium text-slate-700">{'Difficulty Level'}</span>}
                    name="level"
                    className="!mb-6"
                  >
                    <Select
                      className="!h-12"
                      variant="filled"
                      options={[
                        { value: 'simple', label: 'Simple' },
                        { value: 'normal', label: 'Normal' },
                        { value: 'hard', label: 'Hard' },
                      ]}
                    />
                  </Form.Item>

                  <div className="mt-2">
                    <Button
                      type="primary"
                      block
                      size="large"
                      className="!h-14 !text-lg !font-medium !rounded-xl bg-gradient-to-r from-purple-500 to-pink-600 hover:!from-purple-600 hover:!to-pink-700 border-0 shadow-lg shadow-purple-500/30 hover:shadow-purple-500/40 transition-all duration-300 transform hover:-translate-y-0.5"
                      onClick={handleStart}
                      loading={starting}
                      disabled={starting}
                    >
                      {'Start Specialized Training'}
                    </Button>
                    <div className="text-center text-slate-400 text-sm mt-4">
                      {'Approx 30-60 mins per session'}
                    </div>
                  </div>
                </Form>
              </div>
            </Col>
          </Row>
        </div>
      </div>
    </div>
  );
}
