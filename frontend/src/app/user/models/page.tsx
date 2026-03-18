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

import Link from 'next/link';
import apiClient from '@/services/api/client';
import { MODEL_API } from '@/config/api';

const { Title, Paragraph } = Typography;

interface UserModel {
  id: number;
  name: string;
  model_key: string;
  protocol: string; // 'openai' | 'ollama' | 'ark'
  base_url: string;
  api_key: string;
  status: number;
  is_default: number;
  provider_name: string;
  created_at: number;
}

export default function UserModelsPage() {
  
  const [loading, setLoading] = useState(false);
  const [models, setModels] = useState<UserModel[]>([]);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingModel, setEditingModel] = useState<UserModel | null>(null);
  const [form] = Form.useForm();

  // 获取模型列表
  const fetchModels = async () => {
    setLoading(true);
    try {
      const res: any = await apiClient.get(MODEL_API.LIST, {
        params: { page: 1, size: 100 }
      });
      setModels(res?.list || []);
    } catch (e: any) {
      if (e?.response?.status === 401) {
        message.error("Please login first");
      } else {
        message.error("Failed to load");
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
        const updateData = {
          ...editingModel,
          ...values,
        };
        if (!updateData.api_key) {
          delete updateData.api_key;
        }
        await apiClient.put(MODEL_API.UPDATE(editingModel.id), updateData);
        message.success('更新成功');
      } else {
        await apiClient.post(MODEL_API.CREATE, {
          is_default: 1,
          status: 1,
          scope: 7,
          ...values,
        });
        message.success("Created successfully");
      }
      setIsModalOpen(false);
      form.resetFields();
      setEditingModel(null);
      fetchModels();
    } catch (e: any) {
      message.error(e?.message || (editingModel ? '更新失败' : "Creation failed"));
    }
  };

  // 删除模型
  const handleDelete = async (id: number) => {
    try {
      await apiClient.delete(MODEL_API.DELETE(id));
      message.success("Deleted successfully");
      fetchModels();
    } catch (e: any) {
      message.error(e?.message || "Deletion failed");
    }
  };

  // 切换启用状态
  const toggleEnabled = async (record: UserModel) => {
    try {
      await apiClient.put(MODEL_API.UPDATE(record.id), {
        ...record,
        is_default: record.is_default === 1 ? 0 : 1,
        status: 1, // Ensure status is 1
      });
      message.success("Status updated");
      fetchModels();
    } catch (e) {
      message.error("Failed to update status");
    }
  };

  const openEditModal = (record: UserModel) => {
    setEditingModel(record);
    form.setFieldsValue({
      ...record,
      api_key: undefined, // ensure it's empty in UI
    });
    setIsModalOpen(true);
  };

  const columns = [
    {
      title: "Model Name",
      dataIndex: 'name',
      key: 'name',
      render: (text: string) => <span className="font-bold text-slate-700">{text}</span>,
    },
    {
      title: "Model ID",
      dataIndex: 'model_key',
      key: 'model_key',
      render: (text: string) => <Tag className="font-mono bg-slate-50 border-slate-200">{text}</Tag>,
    },
    {
      title: "Protocol",
      dataIndex: 'protocol',
      key: 'protocol',
      render: (text: string) => (
        <Tag color={text === 'openai' ? 'green' : 'geekblue'}>{text.toUpperCase()}</Tag>
      ),
    },
    {
      title: "Provider",
      dataIndex: 'provider_name',
      key: 'provider_name',
      render: (text: string) => text || '-',
    },
    {
      title: "Status",
      dataIndex: 'is_default',
      key: 'is_default',
      render: (is_default: number) => (
        <Tag
          icon={is_default === 1 ? <CheckCircleOutlined /> : <CloseCircleOutlined />}
          color={is_default === 1 ? 'success' : 'default'}
          className="border-0"
        >
          {is_default === 1 ? "Enable" : "Disable"}
        </Tag>
      ),
    },
    {
      title: "Created",
      dataIndex: 'created_at',
      key: 'created_at',
      render: (time: number) => (
        <span className="text-slate-400 text-sm">
          {new Date(time * 1000).toLocaleDateString()}
        </span>
      ),
    },
    {
      title: "Action",
      key: 'action',
      render: (_: any, record: UserModel) => (
        <Space size="middle">
          <Button
            type="text"
            size="small"
            onClick={() => toggleEnabled(record)}
            className={record.is_default === 1 ? 'text-orange-500' : 'text-green-600'}
          >
            {record.is_default === 1 ? "Disable" : "Enable"}
          </Button>
          <Button
            type="text"
            size="small"
            icon={<EditOutlined />}
            onClick={() => openEditModal(record)}
            className="text-blue-600"
          >
            {"Edit"}
          </Button>
          <Popconfirm
            title={"Confirm Delete"}
            description={`Are you sure you want to delete model "${record.name}"?`}
            onConfirm={() => handleDelete(record.id)}
            okText="Yes"
            cancelText="No"
          >
            <Button type="text" size="small" danger icon={<DeleteOutlined />}>
              {"Delete"}
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
            {"User Model Management"}
          </h1>
          <p className="text-slate-500 mt-2">{"Configure and manage your AI model interfaces"}</p>
        </div>

        <div className="bg-white rounded-3xl p-6 border border-slate-100 shadow-xl shadow-slate-200/50 animate-fade-in-up" style={{ animationDelay: '0.1s' }}>
          
          {/* Help Alert */}
          <Alert
            message={
              <div className="flex items-center gap-2">
                <QuestionCircleOutlined className="text-blue-500" />
                <span className="text-slate-600">
                  {"If you don't know how to configure or get free models, please see the"}{' '}
                  <Link href="/manual" className="text-blue-600 font-bold hover:underline flex items-center gap-1 inline-flex">
                    <BookOutlined /> {"User Manual"}
                  </Link>
                  {" in the navigation bar."}
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
                  is_default: 1,
                  status: 1,
                  base_url: 'https://api.openai.com/v1',
                });
                setIsModalOpen(true);
              }}
              size="large"
              className="bg-cyan-600 hover:bg-cyan-500 shadow-lg shadow-cyan-200 rounded-xl"
            >
              {"Create Model"}
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
            {editingModel ? "Edit Model" : "Create New Model"}
          </div>
        }
        open={isModalOpen}
        onOk={handleOk}
        onCancel={() => setIsModalOpen(false)}
        okText={"Save"}
        cancelText="Cancel" // Keep Cancel in English or add key? Antd locale handles it usually if ConfigProvider is used, or hardcode.
        // If I want to be perfect, I should add 'cancel' key. But 'Cancel' is often acceptable or auto-translated by browser if not Antd locale.
        // Actually Antd components use context for locale.
        // I will leave it as "Cancel" for now or use "common.cancel" if I had it.
        // I'll stick to 'Cancel' for now.
        centered
        className="rounded-2xl overflow-hidden"
      >
        <Form form={form} layout="vertical" className="mt-4">
          <div className="bg-slate-50 p-4 rounded-xl mb-4 border border-slate-100">
            <h3 className="text-sm font-bold text-slate-500 mb-3 uppercase tracking-wider">{"Base Config"}</h3>
            <Form.Item
              name="name"
              label={"Model Name"}
              rules={[{ required: true, message: 'Please enter model name' }]} // Validation message i18n?
            >
              <Input placeholder={"e.g., My Model"} className="rounded-lg" />
            </Form.Item>
            
            <div className="grid grid-cols-2 gap-4">
              <Form.Item
                name="protocol"
                label={"Protocol"}
                rules={[{ required: true }]}
              >
                <Select
                  options={[
                    { value: 'openai', label: 'OpenAI (Standard)' },
                    { value: 'ollama', label: 'Ollama (Local)' },
                    { value: 'ark', label: 'Ark' },
                  ]}
                  className="rounded-lg"
                />
              </Form.Item>
              <Form.Item
                name="model_key"
                label={"Model ID"}
                rules={[{ required: true, message: 'Please enter Model ID' }]}
              >
                <Input placeholder={"e.g., gpt-4o"} className="rounded-lg" />
              </Form.Item>
            </div>
          </div>

          <div className="bg-slate-50 p-4 rounded-xl border border-slate-100 mb-4">
            <h3 className="text-sm font-bold text-slate-500 mb-3 uppercase tracking-wider">Provider Info</h3>
            <Form.Item
              name="provider_name"
              label={"Provider Name"}
              rules={[{ required: true, message: 'Please enter Provider Name' }]}
            >
              <Input placeholder={"e.g., OpenAI"} className="rounded-lg" />
            </Form.Item>
          </div>

          <div className="bg-slate-50 p-4 rounded-xl border border-slate-100">
            <h3 className="text-sm font-bold text-slate-500 mb-3 uppercase tracking-wider">API Connection</h3>
            <Form.Item
              name="base_url"
              label={"Base URI"}
              rules={[{ required: true, message: 'Please enter Base URI' }]}
            >
              <Input placeholder={"e.g., https://api.openai.com/v1"} className="rounded-lg" />
            </Form.Item>

            <Form.Item
              name="api_key"
              label={"API Key"}
              rules={[{ required: editingModel ? false : true, message: 'Please enter API Key' }]}
            >
              <Input.Password placeholder={editingModel ? "Leave empty to keep current key" : "Enter API Key"} className="rounded-lg" />
            </Form.Item>
          </div>
        </Form>
      </Modal>
    </div>
  );
}
