'use client';

import { QuestionCircleOutlined } from '@ant-design/icons';
import { Tooltip } from 'antd';

export function InlineHelp({ title }: { title: React.ReactNode }) {
  return (
    <Tooltip title={title}>
      <QuestionCircleOutlined className="admin-inline-help" />
    </Tooltip>
  );
}
