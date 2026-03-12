'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useTranslations } from 'next-intl';
import Link from 'next/link';
import {
  Typography,
  Row,
  Col,
  Card as AntCard,
  Form,
  Select,
  Button,
  Tag,
  message,
  Alert,
} from 'antd';
import { CheckCircleOutlined } from '@ant-design/icons';
import apiClient from '@/services/api/client';
import { API_BASE_URL } from '@/config/api';

const { Title, Paragraph, Text } = Typography;

export default function SpecialInterviewPage() {
  const t = useTranslations('Special');
  const [stack, setStack] = useState<string>('Go');
  const [starting, setStarting] = useState(false);
  const [modelConfigured, setModelConfigured] = useState<boolean | null>(null);
  const [checkingConfig, setCheckingConfig] = useState<boolean>(false);
  const [form] = Form.useForm();
  const router = useRouter();

  useEffect(() => {
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
  }, []);

  const handleStart = async () => {
    if (!modelConfigured) {
      message.error(t('modelNotConfigured'));
      return;
    }
    try {
      const values = await form.validateFields();
      setStarting(true);

      const params = {
        type: '专项面试',
        domain: values.stack,
        difficulty: values.level,
      };

      (window as any).__interviewParams = { ...params };
      try {
        sessionStorage.setItem('interviewParams', JSON.stringify(params));
      } catch {}

      router.push('/interview/special/start');
    } catch (e) {
      message.error('请选择专项类别和难度等级'); // Could translate this too but it's a validation error
    } finally {
      setStarting(false);
    }
  };

  const GROUPED_OPTIONS = [
    {
      label: t('groups.language'),
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
      label: t('groups.backend'),
      options: [
        { value: 'Redis', label: 'Redis' },
        { value: 'MySQL', label: 'MySQL' },
        { value: 'Kafka', label: 'Kafka' },
        { value: 'MongoDB', label: 'MongoDB' },
      ],
    },
    {
      label: t('groups.cloud'),
      options: [
        { value: 'Docker', label: 'Docker' },
        { value: 'Kubernetes', label: 'Kubernetes' },
        { value: 'Nginx', label: 'Nginx' },
      ],
    },
    {
      label: t('groups.cs'),
      options: [
        { value: '操作系统', label: t('options.os') },
        { value: '计算机网络', label: t('options.network') },
        { value: '数据结构与算法', label: t('options.algo') },
      ],
    },
  ];

  const features = [
    { title: t('features.precision.title'), desc: t('features.precision.desc') },
    { title: t('features.link.title'), desc: t('features.link.desc') },
    { title: t('features.density.title'), desc: t('features.density.desc') },
    { title: t('features.simulation.title'), desc: t('features.simulation.desc') },
  ];

  return (
    <div className="min-h-screen py-12 bg-slate-50/50">
      <div className="max-w-5xl mx-auto px-4">
        {/* Header */}
        <div className="text-center mb-10">
          <Title level={2} className="!text-3xl !font-bold text-slate-800 !mb-3">
            {t('title')} · <span className="text-purple-600">{stack}</span>
          </Title>
          <Paragraph className="text-slate-500 text-base max-w-2xl mx-auto">
            {t('description')}
          </Paragraph>
        </div>

        {/* Main Card */}
        <div className="bg-white rounded-[32px] shadow-[0_8px_30px_rgb(0,0,0,0.04)] border border-slate-100 p-8 md:p-10 relative overflow-hidden">
          {/* Decorative Background - Purple theme */}
          <div className="absolute top-0 right-0 w-96 h-96 bg-purple-50/50 rounded-full blur-3xl -translate-y-1/2 translate-x-1/3 pointer-events-none" />
          <div className="absolute bottom-0 left-0 w-96 h-96 bg-pink-50/50 rounded-full blur-3xl translate-y-1/2 -translate-x-1/3 pointer-events-none" />

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
                    {t('featuresTitle')}
                  </Title>
                  <Text className="text-slate-400 text-sm">{t('featuresDesc')}</Text>
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
                    {t('tip')}
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
                  <span className="w-1.5 h-6 bg-purple-500 rounded-full block"></span>
                  {t('configTitle')}
                </Title>

                <Form
                  form={form}
                  layout="vertical"
                  size="large"
                  initialValues={{ stack: stack, level: '简单' }}
                  className="flex flex-col gap-4"
                >
                  <Form.Item
                    label={<span className="font-medium text-slate-700">{t('form.stackLabel')}</span>}
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
                      placeholder={t('form.stackPlaceholder')}
                    />
                  </Form.Item>

                  <Form.Item
                    label={<span className="font-medium text-slate-700">{t('form.levelLabel')}</span>}
                    name="level"
                    className="!mb-6"
                  >
                    <Select
                      className="!h-12"
                      variant="filled"
                      options={[
                        { value: '简单', label: t('options.level.simple') },
                        { value: '中等', label: t('options.level.normal') },
                        { value: '复杂', label: t('options.level.hard') },
                      ]}
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
                      className="!h-14 !text-lg !font-medium !rounded-xl bg-gradient-to-r from-purple-500 to-pink-600 hover:!from-purple-600 hover:!to-pink-700 border-0 shadow-lg shadow-purple-500/30 hover:shadow-purple-500/40 transition-all duration-300 transform hover:-translate-y-0.5"
                      onClick={handleStart}
                      loading={starting}
                      disabled={starting || checkingConfig || modelConfigured === false}
                    >
                      {t('startTraining')}
                    </Button>
                    <div className="text-center text-slate-400 text-sm mt-4">
                      {t('footerText')}
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
