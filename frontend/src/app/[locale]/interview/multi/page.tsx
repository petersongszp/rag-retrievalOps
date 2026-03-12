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
import { useTranslations } from 'next-intl';
import Link from 'next/link';
import { TeamOutlined, CheckCircleOutlined, FileOutlined, StarOutlined } from '@ant-design/icons';
import apiClient from '@/services/api/client';
import { API_BASE_URL } from '@/config/api';

const { Title, Paragraph, Text } = Typography;

// 简历信息类型
interface ResumeInfo {
    id: number;
    file_name: string;
}

export default function MultiAgentInterviewPage() {
    const t = useTranslations('Multi');
    const tCommon = useTranslations('Common');
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

    const features = [
        { title: t('features.hr.title'), desc: t('features.hr.desc') },
        { title: t('features.tech.title'), desc: t('features.tech.desc') },
        { title: t('features.project.title'), desc: t('features.project.desc') },
        { title: t('features.feedback.title'), desc: t('features.feedback.desc') },
    ];

    return (
        <div className="min-h-screen py-12 bg-slate-50/50">
            <div className="max-w-5xl mx-auto px-4">
                {/* Header */}
                <div className="text-center mb-10">
                    <Title level={2} className="!text-3xl !font-bold text-slate-800 !mb-3">
                        {t('title')} · <span className="text-orange-600">{t('subtitle')}</span>
                    </Title>
                    <Paragraph className="text-slate-500 text-base max-w-2xl mx-auto">
                        {t('description')}
                    </Paragraph>
                </div>

                {/* Main Card */}
                <div className="bg-white rounded-[32px] shadow-[0_8px_30px_rgb(0,0,0,0.04)] border border-slate-100 p-8 md:p-10 relative overflow-hidden">
                    {/* Decorative Background - Orange/Amber theme for Multi-Agent */}
                    <div className="absolute top-0 right-0 w-96 h-96 bg-orange-50/50 rounded-full blur-3xl -translate-y-1/2 translate-x-1/3 pointer-events-none" />
                    <div className="absolute bottom-0 left-0 w-96 h-96 bg-amber-50/50 rounded-full blur-3xl translate-y-1/2 -translate-x-1/3 pointer-events-none" />

                    <Row gutter={[48, 32]}>
                        {/* Left Side: Info & Features */}
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
                                            {t('featuresTitle')}
                                        </Title>
                                    </div>
                                    <Text className="text-slate-400 text-sm">{t('featuresDesc')}</Text>
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
                                    <span className="w-1.5 h-6 bg-orange-500 rounded-full block"></span>
                                    {t('configTitle')}
                                </Title>

                                <Form
                                    form={form}
                                    layout="vertical"
                                    size="large"
                                    initialValues={{ job: '', level: '中等' }}
                                    className="flex flex-col gap-4"
                                >
                                    <Form.Item
                                        label={<span className="font-medium text-slate-700">{t('form.resumeLabel')}</span>}
                                        name="resume_id"
                                        rules={[{ required: true, message: t('form.resumePlaceholder') }]}
                                        className="!mb-2"
                                    >
                                        <Select
                                            placeholder={t('form.resumePlaceholder')}
                                            loading={loadingResumes}
                                            disabled={starting}
                                            className="!h-12"
                                            variant="filled"
                                            onChange={(value) => setSelectedResumeId(value)}
                                            notFoundContent={
                                                loadingResumes ? <Spin size="small" /> : t('noResume')
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
                                            label={<span className="font-medium text-slate-700">{t('form.jobLabel')}</span>}
                                            name="job"
                                            rules={[{ required: true, message: t('form.jobPlaceholder') }]}
                                            className="!mb-2"
                                        >
                                            <Input
                                                placeholder={t('form.jobPlaceholder')}
                                                className="!h-12 !bg-slate-50 border-slate-200 hover:bg-white focus:bg-white transition-colors"
                                            />
                                        </Form.Item>

                                        <Form.Item
                                            label={<span className="font-medium text-slate-700">{t('form.levelLabel')}</span>}
                                            name="level"
                                            rules={[{ required: true, message: t('form.levelLabel') }]}
                                            className="!mb-2"
                                        >
                                            <Select
                                                className="!h-12"
                                                variant="filled"
                                                options={[
                                                    { value: '简单', label: t('options.level.entry') },
                                                    { value: '中等', label: t('options.level.medium') },
                                                    { value: '复杂', label: t('options.level.hard') },
                                                ]}
                                            />
                                        </Form.Item>
                                    </div>

                                    <Form.Item
                                        label={<span className="font-medium text-slate-700">{t('form.companyLabel')}</span>}
                                        name="company_name"
                                        className="!mb-6"
                                    >
                                        <Input
                                            placeholder={t('form.companyPlaceholder')}
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
                                            className="!h-14 !text-lg !font-medium !rounded-xl bg-gradient-to-r from-orange-500 to-amber-600 hover:!from-orange-600 hover:!to-amber-700 border-0 shadow-lg shadow-orange-500/30 hover:shadow-orange-500/40 transition-all duration-300 transform hover:-translate-y-0.5"
                                            loading={starting}
                                            disabled={starting || checkingConfig || modelConfigured === false}
                                            onClick={async () => {
                                                try {
                                                    await form.validateFields();
                                                } catch (e) {
                                                    message.error('请完善配置后再挑战多人面试');
                                                    return;
                                                }
                                                if (!modelConfigured) {
                                                    message.error(t('modelNotConfigured'));
                                                    return;
                                                }
                                                const values = form.getFieldsValue();
                                                const params = {
                                                    type: '多人模拟面试',
                                                    domain: 'Eino多智能体架构',
                                                    difficulty: values.level,
                                                    position_name: values.job || '',
                                                    company_name: String(values.company_name || ''),
                                                    resume_id: values.resume_id,
                                                };
                                                (window as any).__interviewParams = { ...params };
                                                try {
                                                    sessionStorage.setItem('interviewParams', JSON.stringify(params));
                                                } catch { }
                                                setStarting(true);
                                                router.push('/interview/multi/start');
                                            }}
                                        >
                                            {t('startInterview')}
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
            <Modal
                open={showNoResumeModal}
                title={tCommon('noResumeTitle')}
                footer={null}
                onCancel={() => setShowNoResumeModal(false)}
                centered
            >
                <div className="text-center py-6">
                    <div className="mb-4 text-slate-600 text-lg">{tCommon('noResumeDesc')}</div>
                    <div className="mb-8 text-slate-500">
                        {tCommon('noResumeTip')}
                    </div>
                    <Button
                        type="primary"
                        size="large"
                        onClick={() => router.push('/user/center')}
                        className="w-full bg-indigo-600 hover:bg-indigo-500"
                    >
                        {tCommon('uploadResume')}
                    </Button>
                </div>
            </Modal>
        </div>
    );
}
