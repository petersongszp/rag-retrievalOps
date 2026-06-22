'use client';

import { Card, Space, Typography } from 'antd';

const { Text } = Typography;

export function MetricCard({
  label,
  value,
  helper,
  extra,
  onClick,
}: {
  label: React.ReactNode;
  value: React.ReactNode;
  helper?: React.ReactNode;
  extra?: React.ReactNode;
  onClick?: () => void;
}) {
  return (
    <Card
      hoverable={Boolean(onClick)}
      className="admin-metric-card"
      styles={{ body: { padding: 20 } }}
      onClick={onClick}
      style={onClick ? { cursor: 'pointer' } : undefined}
    >
      <Space direction="vertical" size={8} className="w-full">
        <div className="flex items-start justify-between gap-3">
          <Text className="admin-metric-card__label">{label}</Text>
          {extra}
        </div>
        <div className="admin-metric-card__value">{value}</div>
        {helper ? <Text className="admin-metric-card__helper">{helper}</Text> : null}
      </Space>
    </Card>
  );
}
