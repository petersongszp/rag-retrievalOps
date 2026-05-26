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
  icon?: React.ReactNode;
  disabled?: boolean;
  children?: NavItem[];
};

const navItems: NavItem[] = [
  { key: '/dashboard', label: '概览', href: '/dashboard', icon: <BarChartOutlined /> },
  { key: '/knowledge-bases', label: '知识库', href: '/knowledge-bases', icon: <DatabaseOutlined /> },
  { key: '/retrieval-lab', label: '检索实验室', href: '/retrieval-lab', icon: <ExperimentOutlined /> },
  {
    key: '/trace-logs',
    label: '链路日志',
    href: '/trace-logs',
    icon: <FileSearchOutlined />,
    children: [
      { key: '/trace-logs/retrieval', label: '检索日志', href: '/trace-logs/retrieval' },
      { key: '/trace-logs/ingest', label: '入库日志', href: '/trace-logs/ingest' },
    ],
  },
  {
    key: '/evaluation',
    label: '评测',
    href: '/evaluation',
    icon: <BookOutlined />,
    children: [
      { key: '/evaluation/datasets', label: '评测集', href: '/evaluation/datasets' },
      { key: '/evaluation/runs', label: '评测运行', href: '/evaluation/runs' },
    ],
  },
  { key: '/quality-monitor', label: '质量监控', href: '/quality-monitor', icon: <AppstoreOutlined /> },
  { key: '/strategy-center', label: '策略中心', icon: <SettingOutlined />, disabled: true },
  { key: '/cost-ops', label: '成本运营', icon: <WalletOutlined />, disabled: true },
  { key: '/audit', label: '审计', icon: <FolderOpenOutlined />, disabled: true },
];

function getSelectedNavKey(pathname: string): string {
  if (pathname.startsWith('/trace-logs/ingest')) {
    return '/trace-logs/ingest';
  }
  if (pathname.startsWith('/trace-logs/retrieval')) {
    return '/trace-logs/retrieval';
  }
  if (pathname.startsWith('/evaluation/reports/')) {
    return '/evaluation/runs';
  }
  if (pathname.startsWith('/evaluation/datasets')) {
    return '/evaluation/datasets';
  }
  if (pathname.startsWith('/evaluation/runs')) {
    return '/evaluation/runs';
  }
  if (pathname.startsWith('/evaluation')) {
    return '/evaluation';
  }
  if (pathname.startsWith('/quality-monitor')) {
    return '/quality-monitor';
  }
  if (pathname.startsWith('/knowledge-bases')) {
    return '/knowledge-bases';
  }
  if (pathname.startsWith('/retrieval-lab')) {
    return '/retrieval-lab';
  }
  if (pathname.startsWith('/trace-logs')) {
    return '/trace-logs';
  }
  return '/dashboard';
}

function getOpenKeys(pathname: string): string[] {
  if (pathname.startsWith('/trace-logs')) {
    return ['/trace-logs'];
  }
  if (pathname.startsWith('/evaluation')) {
    return ['/evaluation'];
  }
  return [];
}

function AdminShellInner({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { bases, selectedBase, setSelectedBaseId } = useKnowledgeBaseContext();

  const menuItems = useMemo<MenuProps['items']>(() => {
    const toMenuItem = (item: NavItem): NonNullable<MenuProps['items']>[number] => ({
      key: item.key,
      icon: item.icon,
      disabled: item.disabled,
      label: item.href ? <Link href={item.href}>{item.label}</Link> : item.label,
      children: item.children?.map((child) => toMenuItem(child)),
    });

    return navItems.map((item) => toMenuItem(item));
  }, []);

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
    if (pathname.startsWith('/trace-logs/retrieval')) {
      return [
        { title: <Link href="/dashboard">概览</Link> },
        { title: <Link href="/trace-logs">链路日志</Link> },
        { title: '检索日志' },
      ];
    }
    if (pathname.startsWith('/trace-logs/ingest')) {
      return [
        { title: <Link href="/dashboard">概览</Link> },
        { title: <Link href="/trace-logs">链路日志</Link> },
        { title: '入库日志' },
      ];
    }
    if (pathname.startsWith('/trace-logs')) {
      return [{ title: <Link href="/dashboard">概览</Link> }, { title: '链路日志' }];
    }
    if (pathname.startsWith('/evaluation/reports/')) {
      return [
        { title: <Link href="/dashboard">概览</Link> },
        { title: <Link href="/evaluation/datasets">评测</Link> },
        { title: <Link href="/evaluation/runs">评测运行</Link> },
        { title: '评测报告' },
      ];
    }
    if (pathname.startsWith('/evaluation/runs')) {
      return [
        { title: <Link href="/dashboard">概览</Link> },
        { title: <Link href="/evaluation/datasets">评测</Link> },
        { title: '评测运行' },
      ];
    }
    if (pathname.startsWith('/evaluation/datasets')) {
      return [
        { title: <Link href="/dashboard">概览</Link> },
        { title: <Link href="/evaluation/datasets">评测</Link> },
        { title: '评测集' },
      ];
    }
    if (pathname.startsWith('/evaluation')) {
      return [{ title: <Link href="/dashboard">概览</Link> }, { title: '评测' }];
    }
    if (pathname.startsWith('/quality-monitor')) {
      return [{ title: <Link href="/dashboard">概览</Link> }, { title: '质量监控' }];
    }

    return [{ title: '概览' }];
  }, [pathname, selectedBase?.name]);

  const handleBaseChange = (value?: number) => {
    setSelectedBaseId(value ?? null);
    if (value && pathname.startsWith('/knowledge-bases/')) {
      router.push(`/knowledge-bases/${value}`);
    }
  };

  return (
    <Layout className="min-h-screen">
      <Sider width={256} theme="light" className="border-r border-slate-200">
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
                <Text type="secondary">知识库与评测管理台</Text>
              </div>
            </Space>
          </div>

          <div className="flex-1 px-3 py-4">
            <Menu
              mode="inline"
              selectedKeys={[getSelectedNavKey(pathname)]}
              openKeys={getOpenKeys(pathname)}
              items={menuItems}
            />
          </div>

          <div className="border-t border-slate-200 px-5 py-4">
            <Text type="secondary">
              当前版本已接入 P2 评测闭环，策略中心与治理模块继续按后续阶段推进。
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
                options={bases.map((base) => ({ label: base.name, value: base.id }))}
              />
              <Tag color={selectedBase ? 'blue' : 'default'}>
                {selectedBase ? `当前知识库：${selectedBase.name}` : '未选择知识库'}
              </Tag>
              <Button onClick={() => router.refresh()}>刷新</Button>
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
