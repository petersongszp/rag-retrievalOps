'use client';

import { Space, Tag, Typography } from 'antd';

const { Text } = Typography;

const COLOR_MAP: Record<string, string> = {
  success: 'success',
  processing: 'processing',
  warning: 'warning',
  error: 'error',
  default: 'default',
};

export function StatusBadge({
  status,
  label,
}: {
  status: 'success' | 'processing' | 'warning' | 'error' | 'default';
  label: React.ReactNode;
}) {
  return (
    <Tag color={COLOR_MAP[status]}>
      <Space size={6}>
        <span
          className="admin-status-dot"
          style={{
            backgroundColor:
              status === 'success'
                ? '#16a34a'
                : status === 'processing'
                  ? '#2563eb'
                  : status === 'warning'
                    ? '#d97706'
                    : status === 'error'
                      ? '#dc2626'
                      : '#94a3b8',
          }}
        />
        <Text>{label}</Text>
      </Space>
    </Tag>
  );
}
