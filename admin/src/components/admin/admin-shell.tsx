'use client';

import Link from 'next/link';
import { useMemo } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import {
  AppstoreOutlined,
  AlertOutlined,
  BarChartOutlined,
  BookOutlined,
  DatabaseOutlined,
  ExperimentOutlined,
  FileSearchOutlined,
  FolderOpenOutlined,
  InboxOutlined,
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
  {
    key: '/quality-monitor',
    label: '质量监控',
    href: '/quality-monitor',
    icon: <AppstoreOutlined />,
  },
  {
    key: '/strategy-center',
    label: '策略中心',
    href: '/strategy-center',
    icon: <SettingOutlined />,
  },
  {
    key: '/cost-ops',
    label: '成本运营',
    icon: <WalletOutlined />,
    children: [
      { key: '/cost-ops/cost', label: '成本看板', href: '/cost-ops/cost' },
      { key: '/cost-ops/vector-db', label: 'Vector DB', href: '/cost-ops/vector-db' },
    ],
  },
  { key: '/audit', label: '审计中心', href: '/audit', icon: <FolderOpenOutlined /> },
  { key: '/alerts', label: '告警中心', href: '/alerts', icon: <AlertOutlined /> },
  {
    key: '/reports',
    label: '报告',
    icon: <InboxOutlined />,
    children: [{ key: '/reports/weekly', label: '周报', href: '/reports/weekly' }],
  },
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
  if (pathname.startsWith('/strategy-center')) {
    return '/strategy-center';
  }
  if (pathname.startsWith('/cost-ops/cost')) {
    return '/cost-ops/cost';
  }
  if (pathname.startsWith('/cost-ops/vector-db')) {
    return '/cost-ops/vector-db';
  }
  if (pathname.startsWith('/audit')) {
    return '/audit';
  }
  if (pathname.startsWith('/alerts')) {
    return '/alerts';
  }
  if (pathname.startsWith('/reports/weekly')) {
    return '/reports/weekly';
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
  if (pathname.startsWith('/cost-ops')) {
    return ['/cost-ops'];
  }
  if (pathname.startsWith('/reports')) {
    return ['/reports'];
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
    if (pathname.startsWith('/retrieval-lab/debug')) {
      return [
        { title: <Link href="/dashboard">概览</Link> },
        { title: <Link href="/retrieval-lab">检索实验室</Link> },
        { title: '调试详情' },
      ];
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
    if (pathname.startsWith('/strategy-center')) {
      return [{ title: <Link href="/dashboard">概览</Link> }, { title: '策略中心' }];
    }
    if (pathname.startsWith('/cost-ops/cost')) {
      return [
        { title: <Link href="/dashboard">概览</Link> },
        { title: '成本运营' },
        { title: '成本看板' },
      ];
    }
    if (pathname.startsWith('/cost-ops/vector-db')) {
      return [
        { title: <Link href="/dashboard">概览</Link> },
        { title: '成本运营' },
        { title: 'Vector DB' },
      ];
    }
    if (pathname.startsWith('/audit')) {
      return [{ title: <Link href="/dashboard">概览</Link> }, { title: '审计中心' }];
    }
    if (pathname.startsWith('/alerts')) {
      return [{ title: <Link href="/dashboard">概览</Link> }, { title: '告警中心' }];
    }
    if (pathname.startsWith('/reports/weekly')) {
      return [
        { title: <Link href="/dashboard">概览</Link> },
        { title: '报告' },
        { title: '周报' },
      ];
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
              当前版本已接入 P4 治理能力骨架，支持成本、向量运维、审计、告警与周报逐步联调。
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
