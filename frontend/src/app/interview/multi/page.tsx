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
                        协作面试 · <span className="text-orange-600">多人模拟面试</span>
                    </Title>
                    <Paragraph className="text-slate-500 text-base max-w-2xl mx-auto">
                        在多人面试模式中，你将面对由“主面试官”、“技术专家”和“项目负责人”组成的面试官小组，模拟最真实、最高压的大厂群面/终面环节。
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
                                            面试官小组构成
                                        </Title>
                                    </div>
                                    <Text className="text-slate-400 text-sm">多角色协作，全方位考察</Text>
                                </div>

                                <div className="space-y-6 flex-1">
                                    {[
                                        { title: '主面试官 (HRD/经理)', desc: '掌控节奏，考察价值观与行为面试' },
                                        { title: '技术专家 (架构师)', desc: '深度硬核提问，探测技能边界' },
                                        { title: '项目负责人 (PO/Lead)', desc: '实战项目复盘，考察落地与避坑能力' },
                                        { title: '多维动态反馈', desc: '各角色分工协作，模拟真实群面压力' },
                                    ].map((t, i) => (
                                        <div key={i} className="flex gap-4 group">
                                            <div className="mt-1 w-10 h-10 rounded-2xl bg-orange-50 text-orange-600 flex items-center justify-center flex-shrink-0 group-hover:bg-orange-500 group-hover:text-white transition-colors duration-300">
                                                <StarOutlined className="text-lg" />
                                            </div>
                                            <div>
                                                <div className="font-medium text-slate-700 mb-1 group-hover:text-orange-600 transition-colors">
                                                    {t.title}
                                                </div>
                                                <div className="text-sm text-slate-400 leading-relaxed">{t.desc}</div>
                                            </div>
                                        </div>
                                    ))}
                                </div>

                                <div className="mt-8 pt-8 border-t border-slate-50 hidden lg:block">
                                    <div className="bg-slate-50 rounded-xl p-4 text-xs text-slate-500 leading-relaxed">
                                        💡 策略：协作模式下，不同面试官会有不同的侧重点，请注意听清提问者的身份并分层次回答。
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
                                    面试团队配置
                                </Title>

                                <Form
                                    form={form}
                                    layout="vertical"
                                    size="large"
                                    initialValues={{ job: '', level: '中等' }}
                                    className="flex flex-col gap-4"
                                >
                                    <Form.Item
                                        label={<span className="font-medium text-slate-700">加载简历</span>}
                                        name="resume_id"
                                        rules={[{ required: true, message: '请选择简历' }]}
                                        className="!mb-2"
                                    >
                                        <Select
                                            placeholder="加载简历进行针对性面试"
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
                                                        <FileOutlined className="text-orange-500" />
                                                        <span className="text-slate-700">{r.file_name}</span>
                                                    </div>
                                                ),
                                            }))}
                                        />
                                    </Form.Item>

                                    <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
                                        <Form.Item
                                            label={<span className="font-medium text-slate-700">面试职位</span>}
                                            name="job"
                                            rules={[{ required: true, message: '请输入意向职位' }]}
                                            className="!mb-2"
                                        >
                                            <Input
                                                placeholder="如：腾讯 Go 后端工程师"
                                                className="!h-12 !bg-slate-50 border-slate-200 hover:bg-white focus:bg-white transition-colors"
                                            />
                                        </Form.Item>

                                        <Form.Item
                                            label={<span className="font-medium text-slate-700">面试难度</span>}
                                            name="level"
                                            rules={[{ required: true, message: '请选择难度等级' }]}
                                            className="!mb-2"
                                        >
                                            <Select
                                                className="!h-12"
                                                variant="filled"
                                                options={[
                                                    { value: '简单', label: '初级 (Junior)' },
                                                    { value: '中等', label: '中高级 (Senior)' },
                                                    { value: '复杂', label: '架构级 (Expert)' },
                                                ]}
                                            />
                                        </Form.Item>
                                    </div>

                                    <Form.Item
                                        label={<span className="font-medium text-slate-700">目标大厂（选填，用于模拟固定风格）</span>}
                                        name="company_name"
                                        className="!mb-6"
                                    >
                                        <Input
                                            placeholder="如：阿里巴巴、字节跳动"
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
                                                    message.error('未配置模型，无法开始面试');
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
                                            开启多人面试挑战 (Eino)
                                        </Button>
                                        <div className="text-center text-slate-400 text-sm mt-4">
                                            多人面试基于 Eino 架构，能够提供极具真实感的团队面试反馈
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
                    <div className="mb-4 text-slate-600 text-lg">检测到您尚未上传简历，无法进行多人面试。</div>
                    <div className="mb-8 text-slate-500">
                        AI 面试官团队需要阅读您的简历才能制定针对性的面试计划。
                    </div>
                    <Button
                        type="primary"
                        size="large"
                        onClick={() => router.push('/user/center')}
                        className="w-full bg-orange-600 hover:bg-orange-500"
                    >
                        去上传简历
                    </Button>
                </div>
            </Modal>
        </div>
    );
}
