'use client';

import { Empty, Space, Typography } from 'antd';

const { Paragraph, Text } = Typography;

export function ActionEmpty({
  title,
  description,
  action,
}: {
  title: React.ReactNode;
  description?: React.ReactNode;
  action?: React.ReactNode;
}) {
  return (
    <Empty
      image={Empty.PRESENTED_IMAGE_SIMPLE}
      description={
        <Space direction="vertical" size={8}>
          <Text strong>{title}</Text>
          {description ? <Paragraph className="mb-0 text-sm text-slate-500">{description}</Paragraph> : null}
          {action}
        </Space>
      }
    />
  );
}
