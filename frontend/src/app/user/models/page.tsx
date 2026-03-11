'use client';

import {
  Typography,
  Card as AntCard,
  Table,
  Button,
  Space,
  Tag,
  Modal,
  Form,
  Input,
  Select,
  InputNumber,
  message,
  Switch,
  Alert,
  Tooltip,
} from 'antd';
import { useEffect, useMemo, useState } from 'react';
import apiClient from '@/services/api/client';
import { API_BASE_URL } from '@/config/api';
import {
  ExperimentOutlined,
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  ApiOutlined,
  InfoCircleOutlined,
  CloudServerOutlined,
} from '@ant-design/icons';

const { Title } = Typography;

type ModelItem = {
  id: number;
  name: string;
  modelKey: string;
  protocol: string;
  baseURL?: string;
  providerName?: string;
  is_default?: number;
  createdAt?: number;
};

export default function UserModelsPage() {
  const [list, setList] = useState<ModelItem[]>([]);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [openCreate, setOpenCreate] = useState(false);
  const [form] = Form.useForm();
  const [openEdit, setOpenEdit] = useState(false);
  const [editForm] = Form.useForm();
  const [editingId, setEditingId] = useState<number | null>(null);

  const fetchList = async (p = page, s = pageSize) => {
    setLoading(true);
    try {
      const res: any = await apiClient.get('/user/model/list', { params: { page: p, size: s } });
      const data = res?.data || res;
      const items: ModelItem[] = (data?.list || []).map((it: any) => ({
        id: it.id ?? it.ID,
        name: it.name ?? it.Name,
        modelKey: it.modelKey ?? it.model_key ?? it.ModelKey,
        protocol: it.protocol ?? it.Protocol,
        baseURL: it.baseURL ?? it.base_url ?? it.BaseURL,
        providerName: it.providerName ?? it.provider_name ?? it.ProviderName,
        is_default: it.is_default ?? it.IsDefault,
        createdAt: it.createdAt ?? it.created_at ?? it.CreatedAt,
      }));
      setList(items);
      setTotal(data?.total || items.length);
    } catch (e: any) {
      message.error(e?.response?.data?.message || '加载失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchList(page, pageSize);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, pageSize]);

  const onCreate = async () => {
    try {
      const v = await form.validateFields();
      const config = {
        icon_uri: v.iconURI,
        temperature: v.temperature,
        max_tokens: v.maxTokens,
        top_p: v.topP,
        top_k: v.topK,
        timeout: v.timeout,
        capability: {
          function_call: v.functionCall === true,
          json_mode: v.jsonMode === true,
          input_tokens: v.inputTokenLimit,
          max_tokens: v.outputTokenLimit,
        },
      };
      const payload = {
        name: v.name,
        model_key: v.modelKey,
        protocol: v.protocol,
        base_url: v.baseURL,
        api_key: v.apiSecret,
        provider_name: v.providerName,
        default_params: v.defaultParams || '{}',
        meta_id:
          v.metaId !== undefined && v.metaId !== null && v.metaId !== ''
            ? Number(v.metaId)
            : undefined,
        config_json: JSON.stringify(config),
        scope: 7,
        is_default: v.is_default === true ? 1 : 0,
      };
      await apiClient.post('/user/create/model', payload);
      message.success('创建成功');
      setOpenCreate(false);
      form.resetFields();
      fetchList(1, pageSize);
      setPage(1);
    } catch (e: any) {
      if (e?.errorFields) return;
      message.error(e?.response?.data?.message || '创建失败');
    }
  };

  const onDelete = async (id: number) => {
    try {
      await apiClient.delete(`/user/model/delete/${id}`);
      message.success('删除成功');
      fetchList(page, pageSize);
    } catch (e: any) {
      message.error(e?.response?.data?.message || '删除失败');
    }
  };

  const columns = useMemo(
    () => [
      {
        title: '模型名称',
        dataIndex: 'name',
        render: (text: string) => <span className="font-bold text-slate-700">{text}</span>,
      },
      {
        title: '模型ID',
        dataIndex: 'modelKey',
        render: (text: string) => (
          <Tag className="font-mono bg-slate-100 text-slate-600 border-slate-200">{text}</Tag>
        ),
      },
      {
        title: '协议',
        dataIndex: 'protocol',
        render: (v: string) => {
          const color = v === 'ark' ? 'blue' : v === 'openai' ? 'green' : 'purple';
          return (
            <Tag color={color} className="capitalize px-2 rounded-md">
              {v}
            </Tag>
          );
        },
      },
      {
        title: '提供商',
        dataIndex: 'providerName',
        render: (text: string) => <span className="text-slate-600">{text}</span>,
      },
      {
        title: '状态',
        dataIndex: 'is_default',
        render: (v: number, row: ModelItem) => (
          <Switch
            checked={v === 1}
            checkedChildren="启用"
            unCheckedChildren="停用"
            className={v === 1 ? 'bg-emerald-500' : 'bg-slate-300'}
            onChange={async (checked) => {
              try {
                const res: any = await apiClient.get(`/user/model/details/${row.id}`);
                const detail = res?.data || res;
                const payload: any = {
                  name: detail?.name ?? row.name,
                  model_key: detail?.model_key ?? detail?.modelKey ?? row.modelKey,
                  protocol: detail?.protocol ?? row.protocol,
                  base_url: detail?.base_url ?? detail?.baseURL ?? row.baseURL,
                  provider_name: detail?.provider_name ?? detail?.providerName ?? row.providerName,
                  is_default: checked ? 1 : 0,
                };
                await apiClient.put(`/user/model/update/${row.id}`, payload);
                message.success('状态已更新');
                fetchList(page, pageSize);
              } catch (e: any) {
                message.error(e?.response?.data?.message || '更新状态失败');
              }
            }}
          />
        ),
      },
      {
        title: '创建时间',
        dataIndex: 'createdAt',
        render: (ts?: any) => (
          <span className="text-slate-400 text-xs font-mono">
            {ts ? (typeof ts === 'number' ? new Date(ts).toLocaleString() : String(ts)) : '-'}
          </span>
        ),
      },
      {
        title: '操作',
        width: 150,
        render: (_: any, row: ModelItem) => (
          <div className="flex items-center gap-2">
            <Tooltip title="编辑配置">
              <Button
                type="text"
                icon={<EditOutlined />}
                className="text-blue-600 hover:bg-blue-50"
                onClick={async () => {
                  const t = typeof window !== 'undefined' ? localStorage.getItem('token') : null;
                  if (!t) {
                    message.warning('请先登录后再编辑');
                    return;
                  }
                  try {
                    const res: any = await apiClient.get(`/user/model/details/${row.id}`);
                    const detail = res?.data || res;
                    let cfg: any = {};
                    try {
                      cfg = detail?.config_json ? JSON.parse(detail.config_json) : {};
                    } catch (_e) {
                      cfg = {};
                    }
                    editForm.setFieldsValue({
                      name: detail?.name ?? row.name,
                      apiSecret: '',
                      modelKey: detail?.model_key ?? detail?.modelKey ?? row.modelKey,
                      providerName:
                        detail?.provider_name ?? detail?.providerName ?? row.providerName,
                      protocol: detail?.protocol ?? row.protocol,
                      metaId: detail?.meta_id,
                      is_default: Boolean(Number(detail?.is_default ?? row.is_default ?? 1)),
                      baseURL: detail?.base_url ?? detail?.baseURL ?? row.baseURL,
                      defaultParams: detail?.default_params ?? '',
                      iconURI: cfg?.icon_uri,
                      temperature: cfg?.temperature,
                      maxTokens: cfg?.max_tokens,
                      topP: cfg?.top_p,
                      topK: cfg?.top_k,
                      timeout: cfg?.timeout,
                      functionCall: cfg?.capability?.function_call,
                      jsonMode: cfg?.capability?.json_mode,
                      inputTokenLimit: cfg?.capability?.input_tokens,
                      outputTokenLimit: cfg?.capability?.max_tokens,
                    });
                    setEditingId(row.id);
                    setOpenEdit(true);
                  } catch (e: any) {
                    message.error(e?.response?.data?.message || '加载详情失败');
                  }
                }}
              />
            </Tooltip>
            <Tooltip title="删除模型">
              <Button
                type="text"
                danger
                icon={<DeleteOutlined />}
                className="hover:bg-red-50"
                onClick={() =>
                  Modal.confirm({
                    title: '确认删除',
                    content: `确定要删除模型 "${row.name}" 吗？`,
                    okType: 'danger',
                    onOk: () => onDelete(row.id),
                  })
                }
              />
            </Tooltip>
          </div>
        ),
      },
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [page, pageSize, list]
  );

  const formLayout = {
    labelCol: { span: 24 },
    wrapperCol: { span: 24 },
  };

  return (
    <div className="min-h-screen relative font-sans">
      {/* Decorative Background */}
      <div className="fixed top-0 right-0 w-[600px] h-[600px] bg-indigo-50/60 rounded-full blur-[120px] -translate-y-1/2 translate-x-1/3 pointer-events-none z-0" />
      <div className="fixed bottom-0 left-0 w-[600px] h-[600px] bg-purple-50/60 rounded-full blur-[120px] translate-y-1/2 -translate-x-1/3 pointer-events-none z-0" />

      <div className="container mx-auto px-4 relative z-10">
        <div className="mb-8 animate-fade-in-up">
          <h1 className="text-3xl font-extrabold text-slate-900 tracking-tight flex items-center gap-3">
            <ExperimentOutlined className="text-indigo-600" />
            用户模型管理
          </h1>
          <p className="text-slate-500 mt-2 ml-11">配置和管理您的 AI 模型接口</p>
        </div>

        <div
          className="bg-white rounded-3xl p-8 border border-slate-100 shadow-xl shadow-slate-200/50 animate-fade-in-up"
          style={{ animationDelay: '0.1s' }}
        >
          <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-4 mb-6">
            <div className="bg-blue-50 border border-blue-100 rounded-xl p-4 flex items-start gap-3 flex-1">
              <InfoCircleOutlined className="text-blue-500 mt-1" />
              <div className="text-sm text-blue-700">
                如果您不知道如何配置或获取免费大模型，请参看顶部导航栏的
                <a
                  href="https://awq7m8b63wy.feishu.cn/wiki/Cl8mwzOayiTtaZknRU2cyoFHndL"
                  target="_blank"
                  className="font-bold underline decoration-blue-300 hover:text-blue-800 mx-1"
                >
                  使用手册
                </a>
                ，我们将为您提供详细的指引。
              </div>
            </div>

            <Button
              type="primary"
              icon={<PlusOutlined />}
              size="large"
              className="bg-indigo-600 hover:bg-indigo-500 shadow-lg shadow-indigo-200 h-12 px-6 rounded-xl"
              onClick={() => {
                const t = typeof window !== 'undefined' ? localStorage.getItem('token') : null;
                if (!t) {
                  message.warning('请先登录后再创建');
                  return;
                }
                setOpenCreate(true);
              }}
            >
              创建模型
            </Button>
          </div>

          <Table
            rowKey="id"
            loading={loading}
            columns={columns as any}
            dataSource={list}
            pagination={{
              current: page,
              pageSize,
              total,
              onChange: setPage,
              showSizeChanger: true,
              onShowSizeChange: (_c, s) => setPageSize(s),
              showTotal: (total) => <span className="text-slate-400">共 {total} 个模型</span>,
              className: 'px-4',
            }}
            className="modern-table"
          />
        </div>

        {/* Create Modal */}
        <Modal
          open={openCreate}
          title={
            <div className="text-lg font-bold flex items-center gap-2">
              <CloudServerOutlined /> 创建新模型
            </div>
          }
          onCancel={() => setOpenCreate(false)}
          onOk={onCreate}
          okText="创建"
          width={800}
          styles={{ body: { maxHeight: '70vh', overflowY: 'auto', padding: '20px' } }}
          destroyOnClose
          centered
        >
          <Form
            form={form}
            layout="vertical"
            initialValues={{
              protocol: 'ark',
              providerName: 'OpenAI',
              is_default: true,
              temperature: 0.7,
              maxTokens: 2048,
              topP: 0.9,
              topK: 40,
              timeout: 30,
              functionCall: true,
              jsonMode: true,
              inputTokenLimit: 128000,
              outputTokenLimit: 128000,
            }}
          >
            <div className="bg-slate-50 p-4 rounded-xl mb-6 border border-slate-100">
              <h3 className="font-bold text-slate-700 mb-4 flex items-center gap-2">
                <ApiOutlined /> 基础配置
              </h3>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <Form.Item
                  label="模型名称"
                  name="name"
                  rules={[{ required: true, message: '请输入模型名称' }]}
                >
                  <Input placeholder="如：My GPT-4 Model" maxLength={100} />
                </Form.Item>
                <Form.Item
                  label="API 秘钥"
                  name="apiSecret"
                  rules={[{ required: true, message: '请输入API秘钥' }]}
                >
                  <Input.Password placeholder="请输入平台 API Key" maxLength={500} />
                </Form.Item>
                <Form.Item
                  label="模型 ID"
                  name="modelKey"
                  rules={[{ required: true, message: '请输入模型 ID' }]}
                >
                  <Input placeholder="如：gpt-4" maxLength={100} />
                </Form.Item>
                <Form.Item label="协议" name="protocol" rules={[{ required: true }]}>
                  <Select
                    options={[
                      { value: 'ark', label: 'ark' },
                      { value: 'openai', label: 'openai' },
                      { value: 'ollama', label: 'ollama' },
                    ]}
                  />
                </Form.Item>
                <Form.Item label="状态" name="is_default" valuePropName="checked">
                  <Switch checkedChildren="启用" unCheckedChildren="停用" />
                </Form.Item>
                <Form.Item
                label="基础 URI"
                name="baseURL"
                rules={[{ required: true, message: '请输入基础 URI' }]}
              >
                <Input placeholder="API 基础接口地址，如：https://api.xxx.com" maxLength={500} />
              </Form.Item>
              <Form.Item name="providerName" hidden>
                <Input />
              </Form.Item>
              <Form.Item name="metaId" hidden>
                <Input />
              </Form.Item>
              <Form.Item name="defaultParams" hidden>
                <Input />
              </Form.Item>
            </div>
          </div>
          </Form>
        </Modal>

        {/* Edit Modal */}
        <Modal
          open={openEdit}
          title={
            <div className="text-lg font-bold flex items-center gap-2">
              <EditOutlined /> 编辑模型
            </div>
          }
          onCancel={() => setOpenEdit(false)}
          onOk={async () => {
            try {
              const v = await editForm.validateFields();
              const config = {
                icon_uri: v.iconURI,
                temperature: v.temperature,
                max_tokens: v.maxTokens,
                top_p: v.topP,
                top_k: v.topK,
                timeout: v.timeout,
                capability: {
                  function_call: v.functionCall === true,
                  json_mode: v.jsonMode === true,
                  input_tokens: v.inputTokenLimit,
                  max_tokens: v.outputTokenLimit,
                },
              };
              const payload: any = {
                name: v.name,
                model_key: v.modelKey,
                protocol: v.protocol,
                base_url: v.baseURL,
                provider_name: v.providerName,
                config_json: JSON.stringify(config),
                scope: 7,
              };
              if (v.defaultParams) payload.default_params = v.defaultParams;
              if (v.metaId !== undefined && v.metaId !== null && v.metaId !== '')
                payload.meta_id = Number(v.metaId);
              if (v.is_default !== undefined && v.is_default !== null)
                payload.is_default = v.is_default === true ? 1 : 0;
              if (v.apiSecret) payload.api_key = v.apiSecret;
              if (!editingId) {
                message.error('未选择编辑的模型');
                return;
              }
              await apiClient.put(`/user/model/update/${editingId}`, payload);
              message.success('更新成功');
              setOpenEdit(false);
              editForm.resetFields();
              fetchList(page, pageSize);
            } catch (e: any) {
              if (e?.errorFields) return;
              message.error(e?.response?.data?.message || '更新失败');
            }
          }}
          okText="更新"
          width={800}
          styles={{ body: { maxHeight: '70vh', overflowY: 'auto', padding: '20px' } }}
          destroyOnClose
          centered
        >
          <Form
            form={editForm}
            layout="vertical"
            initialValues={{
              protocol: 'ark',
              providerName: 'OpenAI',
              is_default: true,
              temperature: 0.7,
              maxTokens: 2048,
              topP: 0.9,
              topK: 40,
              timeout: 30,
              functionCall: true,
              jsonMode: true,
              inputTokenLimit: 128000,
              outputTokenLimit: 128000,
            }}
          >
            <div className="bg-slate-50 p-4 rounded-xl mb-6 border border-slate-100">
              <h3 className="font-bold text-slate-700 mb-4 flex items-center gap-2">
                <ApiOutlined /> 基础配置
              </h3>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <Form.Item
                  label="模型名称"
                  name="name"
                  rules={[{ required: true, message: '请输入模型名称' }]}
                >
                  <Input placeholder="如：My GPT-4 Model" maxLength={100} />
                </Form.Item>
                <Form.Item
                  label="API 秘钥"
                  name="apiSecret"
                >
                  <Input.Password 
                    placeholder="留空则不更新密钥" 
                    maxLength={500} 
                    autoComplete="new-password"
                  />
                </Form.Item>
                <Form.Item
                  label="模型 ID"
                  name="modelKey"
                  rules={[{ required: true, message: '请输入模型 ID' }]}
                >
                  <Input placeholder="如：gpt-4" maxLength={100} />
                </Form.Item>
                <Form.Item label="协议" name="protocol" rules={[{ required: true }]}>
                  <Select
                    options={[
                      { value: 'ark', label: 'ark' },
                      { value: 'openai', label: 'openai' },
                      { value: 'ollama', label: 'ollama' },
                    ]}
                  />
                </Form.Item>
                <Form.Item label="状态" name="is_default" valuePropName="checked">
                  <Switch checkedChildren="启用" unCheckedChildren="停用" />
                </Form.Item>
                <Form.Item
                label="基础 URI"
                name="baseURL"
                rules={[{ required: true, message: '请输入基础 URI' }]}
              >
                <Input placeholder="API 基础接口地址，如：https://api.xxx.com" maxLength={500} />
              </Form.Item>
              <Form.Item name="providerName" hidden>
                <Input />
              </Form.Item>
              <Form.Item name="metaId" hidden>
                <Input />
              </Form.Item>
              <Form.Item name="defaultParams" hidden>
                <Input />
              </Form.Item>
            </div>
          </div>
          </Form>
        </Modal>
      </div>
    </div>
  );
}
