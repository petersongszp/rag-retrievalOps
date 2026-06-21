'use client';

import { Alert, Modal, Typography } from 'antd';

const { Paragraph } = Typography;

export function RiskConfirmModal({
  open,
  title,
  description,
  riskNote,
  confirmText,
  cancelText,
  confirmLoading,
  onConfirm,
  onCancel,
}: {
  open: boolean;
  title: React.ReactNode;
  description: React.ReactNode;
  riskNote?: React.ReactNode;
  confirmText?: string;
  cancelText?: string;
  confirmLoading?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  return (
    <Modal
      open={open}
      title={title}
      okText={confirmText ?? '确认继续'}
      cancelText={cancelText ?? '取消'}
      okButtonProps={{ danger: true, loading: confirmLoading }}
      onOk={onConfirm}
      onCancel={onCancel}
    >
      <Paragraph>{description}</Paragraph>
      {riskNote ? <Alert type="warning" showIcon message={riskNote} /> : null}
    </Modal>
  );
}
