'use client';

import { Space, Typography } from 'antd';

const { Paragraph, Title } = Typography;

export function PageHeader({
  title,
  subtitle,
  extra,
}: {
  title: React.ReactNode;
  subtitle?: React.ReactNode;
  extra?: React.ReactNode;
}) {
  return (
    <div className="admin-page-header">
      <div className="admin-page-header__meta">
        <Title level={2} style={{ marginBottom: 0 }}>
          {title}
        </Title>
        {subtitle ? (
          <Paragraph type="secondary" style={{ marginBottom: 0 }}>
            {subtitle}
          </Paragraph>
        ) : null}
      </div>
      {extra ? <Space wrap>{extra}</Space> : null}
    </div>
  );
}
