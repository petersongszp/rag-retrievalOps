'use client';

import { useMemo } from 'react';
import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import {
  AppstoreOutlined,
  BarChartOutlined,
  BookOutlined,
  DatabaseOutlined,
  ExperimentOutlined,
  FileSearchOutlined,
  FolderOpenOutlined,
  SettingOutlined,
  WalletOutlined,
} from '@ant-design/icons';
import { Breadcrumb, Button, Layout, Menu, Select, Space, Tag, Typography } from 'antd';
import type { MenuProps } from 'antd';
import { KnowledgeBaseProvider, useKnowledgeBaseContext } from './knowledge-base-provider';

const { Header, Sider, Content } = Layout;
const { Title, Text } = Typography;

type NavItem = {
  key: string;
  label: string;
  href?: string;
  icon: React.ReactNode;
  disabled?: boolean;
};

const navItems: NavItem[] = [
  { key: '/dashboard', label: '概览', href: '/dashboard', icon: <BarChartOutlined /> },
  {
    key: '/knowledge-bases',
    label: '知识库',
    href: '/knowledge-bases',
    icon: <DatabaseOutlined />,
  },
  {
    key: '/retrieval-lab',
    label: '检索实验室',
    href: '/retrieval-lab',
    icon: <ExperimentOutlined />,
  },
  { key: '/trace-logs', label: '追踪日志', icon: <FileSearchOutlined />, disabled: true },
  { key: '/evaluation', label: '评测', icon: <BookOutlined />, disabled: true },
  { key: '/strategy-center', label: '策略中心', icon: <SettingOutlined />, disabled: true },
  { key: '/quality-monitor', label: '质量监控', icon: <AppstoreOutlined />, disabled: true },
  { key: '/cost-ops', label: '成本运营', icon: <WalletOutlined />, disabled: true },
  { key: '/audit', label: '审计', icon: <FolderOpenOutlined />, disabled: true },
];

function getSelectedNavKey(pathname: string): string {
  if (pathname.startsWith('/knowledge-bases')) {
    return '/knowledge-bases';
  }

  if (pathname.startsWith('/retrieval-lab')) {
    return '/retrieval-lab';
  }

  return '/dashboard';
}

function AdminShellInner({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { bases, selectedBase, setSelectedBaseId } = useKnowledgeBaseContext();

  const menuItems = useMemo<MenuProps['items']>(
    () =>
      navItems.map((item) => ({
        key: item.key,
        disabled: item.disabled,
        icon: item.icon,
        label: item.href ? <Link href={item.href}>{item.label}</Link> : item.label,
      })),
    []
  );

  const breadcrumbItems = useMemo(() => {
    if (pathname.startsWith('/knowledge-bases/')) {
      return [
        { title: <Link href="/dashboard">概览</Link> },
        { title: <Link href="/knowledge-bases">知识库</Link> },
        { title: selectedBase?.name ?? '知识库详情' },
      ];
    }

    if (pathname.startsWith('/knowledge-bases')) {
      return [{ title: <Link href="/dashboard">概览</Link> }, { title: '知识库' }];
    }

    if (pathname.startsWith('/retrieval-lab')) {
      return [{ title: <Link href="/dashboard">概览</Link> }, { title: '检索实验室' }];
    }

    return [{ title: '概览' }];
  }, [pathname, selectedBase?.name]);

  const handleBaseChange = (value: number) => {
    setSelectedBaseId(value);

    if (pathname.startsWith('/knowledge-bases/')) {
      router.push(`/knowledge-bases/${value}`);
    }
  };

  return (
    <Layout className="min-h-screen">
      <Sider width={260} theme="light" className="border-r border-slate-200">
        <div className="flex h-full flex-col">
          <div className="border-b border-slate-200 px-5 py-5">
            <Space align="start">
              <div className="rounded-xl bg-blue-600 p-2 text-white">
                <DatabaseOutlined />
              </div>
              <div>
                <Title level={4} style={{ margin: 0 }}>
                  RAG Admin
                </Title>
                <Text type="secondary">知识库管理控制台</Text>
              </div>
            </Space>
          </div>
          <div className="flex-1 px-3 py-4">
            <Menu mode="inline" selectedKeys={[getSelectedNavKey(pathname)]} items={menuItems} />
          </div>
          <div className="border-t border-slate-200 px-5 py-4">
            <Text type="secondary">
              P1-P4 功能模块已在导航中展示，待页面上线后启用。
            </Text>
          </div>
        </div>
      </Sider>
      <Layout>
        <Header className="border-b border-slate-200 bg-white px-6">
          <div className="flex h-full items-center justify-between gap-4">
            <div className="min-w-0">
              <Breadcrumb items={breadcrumbItems} />
            </div>
            <Space wrap>
              <Select
                allowClear
                className="min-w-[240px]"
                placeholder="选择当前知识库"
                value={selectedBase?.id}
                onChange={handleBaseChange}
                options={bases.map((base) => ({
                  label: base.name,
                  value: base.id,
                }))}
              />
              <Tag color={selectedBase ? 'blue' : 'default'}>
                {selectedBase ? `当前知识库：${selectedBase.name}` : '未选择知识库'}
              </Tag>
              <Button onClick={() => router.refresh()}>刷新路由</Button>
            </Space>
          </div>
        </Header>
        <Content className="bg-slate-50 p-6">{children}</Content>
      </Layout>
    </Layout>
  );
}

export function AdminShell({ children }: { children: React.ReactNode }) {
  return (
    <KnowledgeBaseProvider>
      <AdminShellInner>{children}</AdminShellInner>
    </KnowledgeBaseProvider>
  );
}
