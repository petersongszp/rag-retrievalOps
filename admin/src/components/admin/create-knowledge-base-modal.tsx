'use client';

import { Form, Input, Modal } from 'antd';

type CreateKnowledgeBaseModalProps = {
  open: boolean;
  loading?: boolean;
  onCancel: () => void;
  onSubmit: (values: { name: string; description?: string }) => Promise<void> | void;
};

export function CreateKnowledgeBaseModal({
  open,
  loading,
  onCancel,
  onSubmit,
}: CreateKnowledgeBaseModalProps) {
  const [form] = Form.useForm<{ name: string; description?: string }>();

  return (
    <Modal
      title="新建知识库"
      open={open}
      okText="新建"
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
        <Form.Item
          label="名称"
          name="name"
          rules={[{ required: true, message: '请输入知识库名称' }]}
        >
          <Input placeholder="示例：Go 面试指南" />
        </Form.Item>
        <Form.Item label="描述" name="description">
          <Input.TextArea rows={4} placeholder="可选，填写该知识库的描述信息" />
        </Form.Item>
      </Form>
    </Modal>
  );
}
