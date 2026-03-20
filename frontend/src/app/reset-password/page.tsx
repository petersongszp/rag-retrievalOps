'use client';

import { useSearchParams, useRouter } from 'next/navigation';
import { Form, Input, Button, message, Card } from 'antd';
import apiClient from '@/services/api/client';
import { useState, Suspense } from 'react';

const ResetPasswordForm = () => {
  const searchParams = useSearchParams();
  const router = useRouter();
  const token = searchParams.get('token');
  const [loading, setLoading] = useState(false);

  if (!token) {
    return (
      <div className="flex justify-center items-center min-h-[50vh]">
        <Card className="w-full max-w-md text-center">
            <h2 className="text-xl font-bold text-red-500 mb-4">链接无效</h2>
            <p className="text-slate-600 mb-6">该重置链接无效或已过期。</p>
            <Button type="primary" onClick={() => router.push('/')}>
                返回首页
            </Button>
        </Card>
      </div>
    );
  }

  const onFinish = async (values: any) => {
    if (values.password !== values.confirm) {
      message.error('两次输入的密码不一致');
      return;
    }
    setLoading(true);
    try {
      await apiClient.post('/user/password/reset', {
        token,
        password: values.password,
        confirm_password: values.confirm,
      });
      message.success('密码重置成功，请重新登录');
      router.push('/');
    } catch (e: any) {
      message.error(e?.response?.data?.message || '重置失败');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex justify-center items-center min-h-[60vh] px-4">
      <Card title="重置密码" className="w-full max-w-md shadow-lg">
        <Form onFinish={onFinish} layout="vertical">
          <Form.Item
            name="password"
            label="新密码"
            rules={[
                { required: true, message: '请输入新密码' },
                { min: 6, message: '密码长度至少为6位' }
            ]}
          >
            <Input.Password placeholder="请输入新密码" />
          </Form.Item>
          <Form.Item
            name="confirm"
            label="确认新密码"
            rules={[
                { required: true, message: '请确认新密码' },
            ]}
          >
            <Input.Password placeholder="请再次输入新密码" />
          </Form.Item>
          <Button type="primary" htmlType="submit" loading={loading} className="w-full" size="large">
            重置密码
          </Button>
        </Form>
      </Card>
    </div>
  );
};

export default function ResetPasswordPage() {
    return (
        <Suspense fallback={<div>Loading...</div>}>
            <ResetPasswordForm />
        </Suspense>
    );
}
