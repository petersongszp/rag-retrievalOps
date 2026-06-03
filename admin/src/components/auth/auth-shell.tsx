'use client';

import { SafetyCertificateOutlined } from '@ant-design/icons';
import { Layout, Space, Typography } from 'antd';

const { Content } = Layout;
const { Paragraph, Text, Title } = Typography;

export function AuthShell({
  title,
  description,
  children,
}: {
  title: string;
  description: string;
  children: React.ReactNode;
}) {
  return (
    <Layout className="min-h-screen bg-[radial-gradient(circle_at_top,_rgba(14,165,233,0.12),_transparent_30%),linear-gradient(135deg,_#f8fbff_0%,_#eef5ff_48%,_#f4f7fb_100%)]">
      <Content className="flex items-center justify-center px-6 py-10">
        <div className="grid w-full max-w-6xl gap-8 lg:grid-cols-[1.1fr_0.9fr]">
          <div className="hidden rounded-[32px] border border-slate-200/80 bg-white/70 p-10 shadow-[0_28px_100px_rgba(15,23,42,0.10)] backdrop-blur lg:block">
            <Space direction="vertical" size={24} className="w-full">
              <div className="flex h-16 w-16 items-center justify-center rounded-3xl bg-sky-600 text-3xl text-white shadow-lg shadow-sky-200">
                <SafetyCertificateOutlined />
              </div>
              <div>
                <Text className="text-xs font-semibold uppercase tracking-[0.35em] text-sky-600">
                  Phase 4 Admin Access
                </Text>
                <Title level={1} style={{ marginTop: 16, marginBottom: 16 }}>
                  用真实账号体系接管 Admin 闭环
                </Title>
                <Paragraph className="max-w-xl text-base text-slate-600">
                  登录、知识库管理、API Key 管理和接入文档都会收敛到同一条受保护链路里。
                  这一步开始，管理台不再依赖开发期默认入口。
                </Paragraph>
              </div>
              <div className="grid gap-4 md:grid-cols-2">
                <div className="rounded-2xl border border-sky-100 bg-sky-50/80 p-5">
                  <Text className="text-sm font-semibold text-slate-900">JWT 会话</Text>
                  <Paragraph className="mb-0 mt-2 text-sm text-slate-600">
                    管理台统一通过 `/v1/auth/login`、`/v1/auth/refresh` 和 `/v1/auth/me`
                    维护登录态。
                  </Paragraph>
                </div>
                <div className="rounded-2xl border border-emerald-100 bg-emerald-50/80 p-5">
                  <Text className="text-sm font-semibold text-slate-900">多租户上下文</Text>
                  <Paragraph className="mb-0 mt-2 text-sm text-slate-600">
                    当前租户、当前角色和后续的 API Key 权限都会在同一套身份上下文里闭环。
                  </Paragraph>
                </div>
              </div>
            </Space>
          </div>

          <div className="rounded-[32px] border border-white/80 bg-white/92 p-8 shadow-[0_28px_100px_rgba(15,23,42,0.14)] backdrop-blur sm:p-10">
            <div className="mb-8">
              <Title level={2} style={{ marginBottom: 8 }}>
                {title}
              </Title>
              <Paragraph type="secondary" style={{ marginBottom: 0 }}>
                {description}
              </Paragraph>
            </div>
            {children}
          </div>
        </div>
      </Content>
    </Layout>
  );
}
