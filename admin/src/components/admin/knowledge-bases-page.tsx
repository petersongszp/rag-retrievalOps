'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { ArrowRightOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Empty, Space, Spin, Tag, Typography } from 'antd';
import { CreateKnowledgeBaseModal } from './create-knowledge-base-modal';
import { useKnowledgeBaseContext } from './knowledge-base-provider';

const { Paragraph, Text, Title } = Typography;

export function KnowledgeBasesPage() {
  const router = useRouter();
  const { bases, selectedBase, isLoading, error, isPermissionDenied, refreshBases, createBase, setSelectedBaseId } =
    useKnowledgeBaseContext();
  const [modalOpen, setModalOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <Title level={2} style={{ marginBottom: 8 }}>
            知识库
          </Title>
          <Paragraph type="secondary" style={{ marginBottom: 0 }}>
            此路由是知识库列表与创建流程的新入口，已在 L0 阶段冻结。
          </Paragraph>
        </div>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => void refreshBases()}>
            刷新
          </Button>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            disabled={isPermissionDenied}
            title={isPermissionDenied ? '权限不足，无法创建知识库' : undefined}
            onClick={() => setModalOpen(true)}
          >
            新建
          </Button>
        </Space>
      </div>

      {error && !isPermissionDenied && <Alert type="error" showIcon message={error} />}
      {isPermissionDenied && (
        <Alert
          type="error"
          showIcon
          message="权限不足"
          description="当前账号无权访问知识库列表（403）。请联系管理员确认权限配置。"
        />
      )}

      <Card>
        <Spin spinning={isLoading}>
          {bases.length === 0 ? (
            <Empty
              description="暂无知识库。新建一个以解锁详情页、文档流程和检索实验室。"
              image={Empty.PRESENTED_IMAGE_SIMPLE}
            />
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
                    ]}
                  >
                    <Space direction="vertical" size={10} className="w-full">
                      <div className="flex items-center justify-between gap-3">
                        <Title level={4} style={{ margin: 0 }}>
                          {base.name}
                        </Title>
                        <Tag color={base.status === 'active' ? 'success' : 'default'}>
                          {base.status}
                        </Tag>
                      </div>
                      <Text type="secondary">{base.description || '暂无描述。'}</Text>
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
