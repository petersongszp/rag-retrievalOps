'use client';

import { LockOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Space, Typography } from 'antd';
import { useRouter } from 'next/navigation';

const { Paragraph, Title } = Typography;

export function ForbiddenState({
  title = '当前角色无权访问',
  description = '当前页面或操作超出了你的角色权限范围；即使绕过前端入口，后端也会继续校验并拒绝越权请求。',
}: {
  title?: string;
  description?: string;
}) {
  const router = useRouter();

  return (
    <Card>
      <Space direction="vertical" size={18} className="w-full text-center">
        <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-orange-100 text-2xl text-orange-600">
          <LockOutlined />
        </div>
        <div>
          <Title level={3} style={{ marginBottom: 8 }}>
            {title}
          </Title>
          <Paragraph type="secondary" style={{ marginBottom: 0 }}>
            {description}
          </Paragraph>
        </div>
        <Alert type="warning" showIcon message="403 Forbidden" />
        <Button onClick={() => router.push('/dashboard')}>返回概览</Button>
      </Space>
    </Card>
  );
}
