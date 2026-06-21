'use client';

import { Form, Input, Modal, Typography } from 'antd';

type CreateKnowledgeBaseModalProps = {
  open: boolean;
  loading?: boolean;
  onCancel: () => void;
  onSubmit: (values: { name: string; description?: string }) => Promise<void> | void;
};

const { Paragraph } = Typography;

export function CreateKnowledgeBaseModal({
  open,
  loading,
  onCancel,
  onSubmit,
}: CreateKnowledgeBaseModalProps) {
  const [form] = Form.useForm<{ name: string; description?: string }>();

  return (
    <Modal
      title="创建知识库"
      open={open}
      okText="创建并进入"
      cancelText="取消"
      confirmLoading={loading}
      onCancel={() => {
        form.resetFields();
        onCancel();
      }}
      onOk={() => form.submit()}
      destroyOnClose
    >
      <Form
        form={form}
        layout="vertical"
        onFinish={async (values) => {
          await onSubmit(values);
          form.resetFields();
        }}
      >
        <Paragraph type="secondary" style={{ marginBottom: 20 }}>
          知识库用于统一管理文档、生成向量索引，并为检索问答提供可维护的知识来源。
        </Paragraph>
        <Form.Item
          label="知识库名称"
          name="name"
          rules={[{ required: true, message: '请输入知识库名称' }]}
        >
          <Input placeholder="例如：客服 FAQ、产品文档、Go 面试题库" />
        </Form.Item>
        <Form.Item label="知识库说明" name="description">
          <Input.TextArea
            rows={4}
            placeholder="可选，补充适用业务、文档范围或维护人，方便团队后续协作。"
          />
        </Form.Item>
      </Form>
    </Modal>
  );
}
