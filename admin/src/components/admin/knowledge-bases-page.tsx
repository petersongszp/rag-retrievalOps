'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import {
  ArrowRightOutlined,
  DeleteOutlined,
  PlusOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import { Alert, Button, Card, Empty, Modal, Space, Spin, Tag, Typography } from 'antd';
import { canCreateKB, canDeleteKB } from '@/services/auth/permissions';
import { useAuth } from '@/services/auth/store';
import { CreateKnowledgeBaseModal } from './create-knowledge-base-modal';
import { useKnowledgeBaseContext } from './knowledge-base-provider';

const { Paragraph, Text, Title } = Typography;

const KNOWLEDGE_BASE_STATUS_LABELS: Record<string, string> = {
  active: '可用',
  processing: '处理中',
  building: '处理中',
  syncing: '处理中',
  failed: '异常',
  error: '异常',
  inactive: '停用',
  disabled: '停用',
};

function getKnowledgeBaseStatusLabel(status?: string): string {
  if (!status) {
    return '未知';
  }

  return KNOWLEDGE_BASE_STATUS_LABELS[status] ?? status;
}

export function KnowledgeBasesPage() {
  const router = useRouter();
  const { user } = useAuth();
  const {
    bases,
    selectedBase,
    isLoading,
    error,
    isPermissionDenied,
    refreshBases,
    createBase,
    deleteBase,
    setSelectedBaseId,
  } = useKnowledgeBaseContext();
  const [modalOpen, setModalOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [deletingBaseId, setDeletingBaseId] = useState<number | null>(null);
  const allowCreate = canCreateKB(user?.role);
  const allowDelete = canDeleteKB(user?.role);

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <Title level={2} style={{ marginBottom: 8 }}>
            知识库管理
          </Title>
          <Paragraph type="secondary" style={{ marginBottom: 0 }}>
            统一管理知识资产，组织文档、生成向量，并为检索问答提供稳定可维护的知识来源。
          </Paragraph>
        </div>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => void refreshBases()}>
            刷新
          </Button>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            disabled={isPermissionDenied || !allowCreate}
            title={isPermissionDenied || !allowCreate ? '当前角色无权创建知识库' : undefined}
            onClick={() => setModalOpen(true)}
          >
            新建
          </Button>
        </Space>
      </div>

      {error && !isPermissionDenied ? <Alert type="error" showIcon message={error} /> : null}
      {isPermissionDenied ? (
        <Alert
          type="error"
          showIcon
          message="权限不足"
          description="当前账号无权访问知识库列表（403）。请联系管理员确认权限配置。"
        />
      ) : null}

      <Card>
        <Spin spinning={isLoading}>
          {bases.length === 0 ? (
            <Empty
              description="还没有知识库。先创建一个知识库，再上传文档并启动入库流程。"
              image={Empty.PRESENTED_IMAGE_SIMPLE}
            >
              <Space>
                <Button icon={<ReloadOutlined />} onClick={() => void refreshBases()}>
                  刷新列表
                </Button>
                <Button
                  type="primary"
                  icon={<PlusOutlined />}
                  disabled={isPermissionDenied || !allowCreate}
                  onClick={() => setModalOpen(true)}
                >
                  创建第一个知识库
                </Button>
              </Space>
            </Empty>
          ) : (
            <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
              {bases.map((base) => {
                const isSelected = selectedBase?.id === base.id;

                return (
                  <Card
                    key={base.id}
                    hoverable
                    className={isSelected ? 'border-blue-500 shadow-sm' : ''}
                    actions={[
                      <Button
                        key="open"
                        type="link"
                        icon={<ArrowRightOutlined />}
                        onClick={() => {
                          setSelectedBaseId(base.id);
                          router.push(`/knowledge-bases/${base.id}`);
                        }}
                      >
                        打开
                      </Button>,
                      <Button
                        key="delete"
                        danger
                        type="link"
                        icon={<DeleteOutlined />}
                        loading={deletingBaseId === base.id}
                        disabled={isPermissionDenied || !allowDelete}
                        title={isPermissionDenied || !allowDelete ? '当前角色无权删除知识库' : undefined}
                        onClick={() => {
                          Modal.confirm({
                            title: '删除知识库',
                            content: `确认删除 "${base.name}"？这会同时清理它绑定的向量 collection 和已上传文档。`,
                            okText: '确认删除',
                            cancelText: '取消',
                            okButtonProps: { danger: true },
                            onOk: async () => {
                              try {
                                setDeletingBaseId(base.id);
                                await deleteBase(base.id);
                              } finally {
                                setDeletingBaseId(null);
                              }
                            },
                          });
                        }}
                      >
                        删除
                      </Button>,
                    ]}
                  >
                    <Space direction="vertical" size={10} className="w-full">
                      <div className="flex items-center justify-between gap-3">
                        <Title level={4} style={{ margin: 0 }}>
                          {base.name}
                        </Title>
                        <Tag color={base.status === 'active' ? 'success' : 'default'}>
                          {getKnowledgeBaseStatusLabel(base.status)}
                        </Tag>
                      </div>
                      <Text type="secondary">
                        {base.description || '暂未填写说明，可进入详情页补充文档范围和使用场景。'}
                      </Text>
                      <Text type="secondary">
                        向量集合：{base.vector_collection || '待系统生成'}
                      </Text>
                      <Text type="secondary">创建时间：{base.created_at}</Text>
                      <Link href={`/knowledge-bases/${base.id}`}>查看详情</Link>
                    </Space>
                  </Card>
                );
              })}
            </div>
          )}
        </Spin>
      </Card>

      <CreateKnowledgeBaseModal
        open={modalOpen}
        loading={submitting}
        onCancel={() => setModalOpen(false)}
        onSubmit={async (values) => {
          try {
            setSubmitting(true);
            const created = await createBase(values);
            setModalOpen(false);
            router.push(`/knowledge-bases/${created.id}`);
          } finally {
            setSubmitting(false);
          }
        }}
      />
    </div>
  );
}
