'use client';

import Link from 'next/link';
import { useMemo, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { Alert, Button, Form, Input, Space, Typography } from 'antd';
import { AuthShell } from '@/components/auth/auth-shell';
import { useAuth } from '@/services/auth/store';
import type { LoginPayload } from '@/types/auth';

const { Paragraph, Text } = Typography;

export default function LoginPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { login } = useAuth();
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const noticeMessage = useMemo(() => {
    if (searchParams.get('registered') === '1') {
      return '注册成功，请使用刚创建的邮箱和密码登录。';
    }
    if (searchParams.get('passwordChanged') === '1') {
      return '密码已更新，请使用新密码重新登录。';
    }
    return null;
  }, [searchParams]);

  const handleSubmit = async (values: LoginPayload) => {
    try {
      setSubmitting(true);
      setError(null);
      await login(values);
      const next = searchParams.get('next');
      router.replace(next || '/dashboard');
    } catch (submitError) {
      setError(
        submitError && typeof submitError === 'object' && 'message' in submitError
          ? String(submitError.message)
          : '登录失败，请检查邮箱和密码后重试。'
      );
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <AuthShell title="登录管理台" description="使用你的租户账号进入受保护的 Admin 控制台。">
      <Space direction="vertical" size={20} className="w-full">
        {noticeMessage ? <Alert type="success" showIcon message={noticeMessage} /> : null}
        {error ? <Alert type="error" showIcon message={error} /> : null}

        <Form<LoginPayload> layout="vertical" onFinish={(values) => void handleSubmit(values)}>
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
            label="密码"
            name="password"
            rules={[{ required: true, message: '请输入密码' }]}
          >
            <Input.Password size="large" placeholder="请输入密码" autoComplete="current-password" />
          </Form.Item>

          <Button block type="primary" htmlType="submit" size="large" loading={submitting}>
            登录
          </Button>
        </Form>

        <div className="rounded-2xl border border-slate-200 bg-slate-50 px-4 py-3">
          <Paragraph className="mb-1 text-sm text-slate-700">
            登录后我们会通过 JWT 读取当前用户、当前租户和角色信息，不会在页面里输出 token。
          </Paragraph>
          <Text type="secondary">如果你还没有账号，请先完成注册。</Text>
        </div>

        <Text type="secondary">
          还没有账号？<Link href="/register">创建 owner 租户</Link>
        </Text>
      </Space>
    </AuthShell>
  );
}
