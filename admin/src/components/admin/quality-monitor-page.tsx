'use client';

import { Alert, Card, Empty, Typography } from 'antd';

const { Paragraph, Title } = Typography;

export function QualityMonitorPage() {
  return (
    <div className="space-y-6">
      <div>
        <Title level={2} style={{ marginBottom: 8 }}>
          质量监控
        </Title>
        <Paragraph type="secondary" style={{ marginBottom: 0 }}>
          Phase 1 仅激活导航承接位，真实质量评测内容将在 Phase 2 接入。
        </Paragraph>
      </div>

      <Alert
        type="info"
        showIcon
        message="P2 预留位"
        description="当前页用于承接后续离线评测与质量监控看板，Phase 1 暂不提供真实指标与分析结果。"
      />

      <Card>
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="质量监控能力将在下一阶段接入" />
      </Card>
    </div>
  );
}
