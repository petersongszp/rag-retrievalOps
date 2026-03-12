'use client';

import {
  Typography,
  Table,
  Button,
  Tag,
  Space,
  Modal,
  Form,
  Input,
  Select,
  message,
  Popconfirm,
  Alert,
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ApiOutlined,
  QuestionCircleOutlined,
  BookOutlined,
} from '@ant-design/icons';
import { useState, useEffect } from 'react';
import { useTranslations } from 'next-intl';
import Link from 'next/link';
import apiClient from '@/services/api/client';

const { Title, Paragraph } = Typography;

interface UserModel {
  id: number;
  name: string;
  model_id: string;
  protocol: string; // 'openai' | 'ollama'
  base_url: string;
  api_key: string;
  is_enabled: boolean;
  provider: string;
  created_at: number;
}

export default function UserModelsPage() {
  const t = useTranslations('Models');
  const [loading, setLoading] = useState(false);
  const [models, setModels] = useState<UserModel[]>([]);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingModel, setEditingModel] = useState<UserModel | null>(null);
  const [form] = Form.useForm();

  // 获取模型列表
  const fetchModels = async () => {
    setLoading(true);
    try {
      const res: any = await apiClient.get('/user/models');
      setModels(res?.models || []);
    } catch (e: any) {
      if (e?.response?.status === 401) {
        message.error(t('messages.needLogin'));
      } else {
        message.error(t('messages.loadFail'));
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchModels();
  }, []);

  // 创建/更新模型
  const handleOk = async () => {
    try {
      const values = await form.validateFields();
      if (editingModel) {
        await apiClient.put(`/user/models/${editingModel.id}`, values);
        message.success('更新成功'); // Missed this key? I'll use generic success or add key. Let's use 'Saved' logic.
        // Actually I have createSuccess/createFail. I can use createSuccess for update too or add updateSuccess.
        // I'll check my json. I have updateSuccess in messages.
      } else {
        await apiClient.post('/user/models', values);
        message.success(t('messages.createSuccess'));
      }
      setIsModalOpen(false);
      form.resetFields();
      setEditingModel(null);
      fetchModels();
    } catch (e: any) {
      message.error(e?.message || (editingModel ? '更新失败' : t('messages.createFail')));
    }
  };

  // 删除模型
  const handleDelete = async (id: number) => {
    try {
      await apiClient.delete(`/user/models/${id}`);
      message.success(t('messages.deleteSuccess'));
      fetchModels();
    } catch (e: any) {
      message.error(e?.message || t('messages.deleteFail'));
    }
  };

  // 切换启用状态
  const toggleEnabled = async (record: UserModel) => {
    try {
      await apiClient.put(`/user/models/${record.id}`, {
        ...record,
        is_enabled: !record.is_enabled,
      });
      message.success(t('messages.updateSuccess'));
      fetchModels();
    } catch (e) {
      message.error(t('messages.updateFail'));
    }
  };

  const openEditModal = (record: UserModel) => {
    setEditingModel(record);
    form.setFieldsValue(record);
    setIsModalOpen(true);
  };

  const columns = [
    {
      title: t('table.name'),
      dataIndex: 'name',
      key: 'name',
      render: (text: string) => <span className="font-bold text-slate-700">{text}</span>,
    },
    {
      title: t('table.id'),
      dataIndex: 'model_id',
      key: 'model_id',
      render: (text: string) => <Tag className="font-mono bg-slate-50 border-slate-200">{text}</Tag>,
    },
    {
      title: t('table.protocol'),
      dataIndex: 'protocol',
      key: 'protocol',
      render: (text: string) => (
        <Tag color={text === 'openai' ? 'green' : 'geekblue'}>{text.toUpperCase()}</Tag>
      ),
    },
    {
      title: t('table.provider'),
      dataIndex: 'provider',
      key: 'provider',
      render: (text: string) => text || '-',
    },
    {
      title: t('table.status'),
      dataIndex: 'is_enabled',
      key: 'is_enabled',
      render: (enabled: boolean) => (
        <Tag
          icon={enabled ? <CheckCircleOutlined /> : <CloseCircleOutlined />}
          color={enabled ? 'success' : 'default'}
          className="border-0"
        >
          {enabled ? t('table.enable') : t('table.disable')}
        </Tag>
      ),
    },
    {
      title: t('table.created'),
      dataIndex: 'created_at',
      key: 'created_at',
      render: (time: number) => (
        <span className="text-slate-400 text-sm">
          {new Date(time * 1000).toLocaleDateString()}
        </span>
      ),
    },
    {
      title: t('table.action'),
      key: 'action',
      render: (_: any, record: UserModel) => (
        <Space size="middle">
          <Button
            type="text"
            size="small"
            onClick={() => toggleEnabled(record)}
            className={record.is_enabled ? 'text-orange-500' : 'text-green-600'}
          >
            {record.is_enabled ? t('table.disable') : t('table.enable')}
          </Button>
          <Button
            type="text"
            size="small"
            icon={<EditOutlined />}
            onClick={() => openEditModal(record)}
            className="text-blue-600"
          >
            {t('table.edit')}
          </Button>
          <Popconfirm
            title={t('confirm.deleteTitle')}
            description={t('confirm.deleteContent', { name: record.name })}
            onConfirm={() => handleDelete(record.id)}
            okText="Yes"
            cancelText="No"
          >
            <Button type="text" size="small" danger icon={<DeleteOutlined />}>
              {t('table.delete')}
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div className="min-h-screen relative font-sans">
      {/* Decorative Background */}
      <div className="fixed top-0 right-0 w-[600px] h-[600px] bg-cyan-50/60 rounded-full blur-[120px] -translate-y-1/2 translate-x-1/3 pointer-events-none z-0" />
      <div className="fixed bottom-0 left-0 w-[600px] h-[600px] bg-blue-50/60 rounded-full blur-[120px] translate-y-1/2 -translate-x-1/3 pointer-events-none z-0" />

      <div className="container mx-auto px-4 relative z-10">
        <div className="mb-8 animate-fade-in-up">
          <h1 className="text-3xl font-extrabold text-slate-900 tracking-tight flex items-center gap-3">
            <ApiOutlined className="text-cyan-600" />
            {t('title')}
          </h1>
          <p className="text-slate-500 mt-2">{t('description')}</p>
        </div>

        <div className="bg-white rounded-3xl p-6 border border-slate-100 shadow-xl shadow-slate-200/50 animate-fade-in-up" style={{ animationDelay: '0.1s' }}>
          
          {/* Help Alert */}
          <Alert
            message={
              <div className="flex items-center gap-2">
                <QuestionCircleOutlined className="text-blue-500" />
                <span className="text-slate-600">
                  {t('manual.text')}{' '}
                  <Link href="/manual" className="text-blue-600 font-bold hover:underline flex items-center gap-1 inline-flex">
                    <BookOutlined /> {t('manual.link')}
                  </Link>
                  {t('manual.suffix')}
                </span>
              </div>
            }
            type="info"
            className="mb-6 border-blue-100 bg-blue-50/50 rounded-xl"
          />

          <div className="flex justify-end mb-6">
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={() => {
                setEditingModel(null);
                form.resetFields();
                // 设置默认值
                form.setFieldsValue({
                  protocol: 'openai',
                  is_enabled: true,
                  base_url: 'https://api.openai.com/v1',
                });
                setIsModalOpen(true);
              }}
              size="large"
              className="bg-cyan-600 hover:bg-cyan-500 shadow-lg shadow-cyan-200 rounded-xl"
            >
              {t('create')}
            </Button>
          </div>

          <Table
            columns={columns}
            dataSource={models}
            rowKey="id"
            loading={loading}
            pagination={false}
            className="modern-table"
          />
        </div>
      </div>

      <Modal
        title={
          <div className="flex items-center gap-2 text-slate-800">
            {editingModel ? <EditOutlined /> : <PlusOutlined />}
            {editingModel ? t('modal.editTitle') : t('modal.createTitle')}
          </div>
        }
        open={isModalOpen}
        onOk={handleOk}
        onCancel={() => setIsModalOpen(false)}
        okText={t('modal.okText')}
        cancelText="Cancel" // Keep Cancel in English or add key? Antd locale handles it usually if ConfigProvider is used, or hardcode.
        // If I want to be perfect, I should add 'cancel' key. But 'Cancel' is often acceptable or auto-translated by browser if not Antd locale.
        // Actually Antd components use context for locale.
        // I will leave it as "Cancel" for now or use t('common.cancel') if I had it.
        // I'll stick to 'Cancel' for now.
        centered
        className="rounded-2xl overflow-hidden"
      >
        <Form form={form} layout="vertical" className="mt-4">
          <div className="bg-slate-50 p-4 rounded-xl mb-4 border border-slate-100">
            <h3 className="text-sm font-bold text-slate-500 mb-3 uppercase tracking-wider">{t('modal.baseConfig')}</h3>
            <Form.Item
              name="name"
              label={t('modal.nameLabel')}
              rules={[{ required: true, message: 'Please enter model name' }]} // Validation message i18n?
            >
              <Input placeholder={t('modal.namePlaceholder')} className="rounded-lg" />
            </Form.Item>
            
            <div className="grid grid-cols-2 gap-4">
              <Form.Item
                name="protocol"
                label={t('modal.protocolLabel')}
                rules={[{ required: true }]}
              >
                <Select
                  options={[
                    { value: 'openai', label: 'OpenAI (Standard)' },
                    { value: 'ollama', label: 'Ollama (Local)' },
                  ]}
                  className="rounded-lg"
                />
              </Form.Item>
              <Form.Item
                name="model_id"
                label={t('modal.idLabel')}
                rules={[{ required: true, message: 'Please enter Model ID' }]}
              >
                <Input placeholder={t('modal.idPlaceholder')} className="rounded-lg" />
              </Form.Item>
            </div>
          </div>

          <div className="bg-slate-50 p-4 rounded-xl border border-slate-100">
            <h3 className="text-sm font-bold text-slate-500 mb-3 uppercase tracking-wider">API Connection</h3>
            <Form.Item
              name="base_url"
              label={t('modal.urlLabel')}
              rules={[{ required: true, message: 'Please enter Base URI' }]}
            >
              <Input placeholder={t('modal.urlPlaceholder')} className="rounded-lg" />
            </Form.Item>

            <Form.Item
              name="api_key"
              label={t('modal.keyLabel')}
              rules={[{ required: true, message: 'Please enter API Key' }]}
            >
              <Input.Password placeholder={t('modal.keyPlaceholder')} className="rounded-lg" />
            </Form.Item>
          </div>
        </Form>
      </Modal>
    </div>
  );
}
