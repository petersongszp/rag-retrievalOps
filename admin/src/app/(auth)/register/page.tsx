'use client';

import Link from 'next/link';
import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { Alert, Button, Form, Input, Space, Typography } from 'antd';
import { AuthShell } from '@/components/auth/auth-shell';
import { getErrorMessage } from '@/services/api/errors';
import { useAuth } from '@/services/auth/store';
import type { RegisterPayload } from '@/types/auth';

const { Paragraph, Text } = Typography;

export default function RegisterPage() {
  const router = useRouter();
  const { register } = useAuth();
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (values: RegisterPayload) => {
    try {
      setSubmitting(true);
      setError(null);
      await register(values);
      router.replace('/login?registered=1');
    } catch (submitError) {
      setError(getErrorMessage(submitError, '注册失败，请检查输入信息后重试。'));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <AuthShell
      title="创建组织管理员账号"
      description="注册完成后将跳转到登录页，使用新账号进入平台。"
    >
      <Space direction="vertical" size={20} className="w-full">
        {error ? <Alert type="error" showIcon message={error} /> : null}

        <Form<RegisterPayload> layout="vertical" onFinish={(values) => void handleSubmit(values)}>
          <Form.Item label="姓名" name="name" rules={[{ required: true, message: '请输入姓名' }]}>
            <Input size="large" placeholder="张三" autoComplete="name" />
          </Form.Item>

          <Form.Item
            label="邮箱"
            name="email"
            rules={[
              { required: true, message: '请输入邮箱地址' },
              { type: 'email', message: '请输入有效的邮箱地址' },
            ]}
          >
            <Input size="large" placeholder="owner@example.com" autoComplete="email" />
          </Form.Item>

          <Form.Item
            label="组织名称"
            name="tenant_name"
            rules={[{ required: true, message: '请输入组织名称' }]}
          >
            <Input size="large" placeholder="Acme Workspace" />
          </Form.Item>

          <Form.Item
            label="密码"
            name="password"
            rules={[
              { required: true, message: '请输入密码' },
              { min: 12, message: '密码至少需要 12 个字符' },
            ]}
            extra="后端会继续校验密码强度；如果不满足要求，会返回明确错误提示。"
          >
            <Input.Password size="large" placeholder="至少 12 个字符" autoComplete="new-password" />
          </Form.Item>

          <Button block type="primary" htmlType="submit" size="large" loading={submitting}>
            创建账号
          </Button>
        </Form>

        <div className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3">
          <Paragraph className="mb-1 text-sm text-slate-700">
            注册成功后会先创建组织和管理员账号，再引导你回到登录页继续完成会话建立。
          </Paragraph>
          <Text type="secondary">这一步不会把 token 暴露到页面或 URL 中。</Text>
        </div>

        <Text type="secondary">
          已经有账号？<Link href="/login">返回登录</Link>
        </Text>
      </Space>
    </AuthShell>
  );
}
