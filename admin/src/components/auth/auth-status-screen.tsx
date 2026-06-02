'use client';

import { LoadingOutlined, SafetyCertificateOutlined } from '@ant-design/icons';
import { Space, Spin, Typography } from 'antd';

const { Paragraph, Title } = Typography;

export function AuthStatusScreen({
  title,
  description,
}: {
  title: string;
  description: string;
}) {
  return (
    <div className="flex min-h-screen items-center justify-center bg-[radial-gradient(circle_at_top,_rgba(14,165,233,0.12),_transparent_35%),linear-gradient(135deg,_#f8fbff_0%,_#eef5ff_48%,_#f4f7fb_100%)] px-6">
      <div className="w-full max-w-md rounded-3xl border border-white/70 bg-white/85 p-10 shadow-[0_24px_80px_rgba(15,23,42,0.12)] backdrop-blur">
        <Space direction="vertical" size={18} className="w-full text-center">
          <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-sky-100 text-2xl text-sky-600">
            <SafetyCertificateOutlined />
          </div>
          <div>
            <Title level={3} style={{ marginBottom: 8 }}>
              {title}
            </Title>
            <Paragraph type="secondary" style={{ marginBottom: 0 }}>
              {description}
            </Paragraph>
          </div>
          <Spin indicator={<LoadingOutlined style={{ fontSize: 22 }} spin />} />
        </Space>
      </div>
    </div>
  );
}
