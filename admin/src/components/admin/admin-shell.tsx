'use client';

import Link from 'next/link';
import { useEffect, useMemo, useState } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import {
  AlertOutlined,
  AppstoreOutlined,
  BarChartOutlined,
  BookOutlined,
  ControlOutlined,
  DatabaseOutlined,
  ExperimentOutlined,
  FileSearchOutlined,
  FolderOpenOutlined,
  InboxOutlined,
  LockOutlined,
  LogoutOutlined,
  SafetyCertificateOutlined,
  SettingOutlined,
  UserOutlined,
  WalletOutlined,
} from '@ant-design/icons';
import {
  Breadcrumb,
  Button,
  Dropdown,
  Form,
  Input,
  Layout,
  Menu,
  Modal,
  Select,
  Space,
  Tag,
  Typography,
  message,
} from 'antd';
import type { MenuProps } from 'antd';
import { TENANT_API } from '@/config/api';
import apiClient from '@/services/api/client';
import {
  canManageAPIKey,
  canViewTenantSettings,
  canViewUsage,
} from '@/services/auth/permissions';
import { useAuth } from '@/services/auth/store';
import type { TenantDetail } from '@/types/tenant';
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
  { key: '/dashboard', label: '工作台', href: '/dashboard', icon: <BarChartOutlined /> },
  {
    key: '/knowledge-bases',
    label: '知识库',
    href: '/knowledge-bases',
    icon: <DatabaseOutlined />,
  },
  {
    key: '/api-keys',
    label: '接入密钥',
    href: '/api-keys',
    icon: <ControlOutlined />,
  },
  {
    key: '/retrieval-lab',
    label: '检索调优',
    href: '/retrieval-lab',
    icon: <ExperimentOutlined />,
  },
  {
    key: '/trace-logs',
    label: '链路追踪',
    href: '/trace-logs',
    icon: <FileSearchOutlined />,
    children: [
      { key: '/trace-logs/retrieval', label: '检索日志', href: '/trace-logs/retrieval' },
      { key: '/trace-logs/ingest', label: '入库日志', href: '/trace-logs/ingest' },
    ],
  },
  {
    key: '/docs',
    label: '接入文档',
    icon: <BookOutlined />,
    children: [{ key: '/docs/integration', label: '集成指南', href: '/docs/integration' }],
  },
  {
    key: '/evaluation',
    label: '质量评测',
    href: '/evaluation',
    icon: <BookOutlined />,
    children: [
      { key: '/evaluation/datasets', label: '评测样本', href: '/evaluation/datasets' },
      { key: '/evaluation/runs', label: '评测任务', href: '/evaluation/runs' },
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
    label: '策略管理',
    href: '/strategy-center',
    icon: <SettingOutlined />,
  },
  {
    key: '/tenant',
    label: '组织',
    icon: <SettingOutlined />,
    children: [
      { key: '/tenant/settings', label: '组织设置', href: '/tenant/settings' },
      { key: '/tenant/usage', label: '组织用量', href: '/tenant/usage' },
    ],
  },
  {
    key: '/cost-ops',
    label: '成本与运维',
    icon: <WalletOutlined />,
    children: [
      { key: '/cost-ops/cost', label: '成本分析', href: '/cost-ops/cost' },
      { key: '/cost-ops/vector-db', label: '向量库运维', href: '/cost-ops/vector-db' },
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
  {
    key: '/semantic-cache',
    label: '语义缓存',
    href: '/semantic-cache',
    icon: <SafetyCertificateOutlined />,
  },
  {
    key: '/embedding-cache',
    label: 'Embedding 缓存',
    href: '/embedding-cache',
    icon: <DatabaseOutlined />,
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
  if (pathname.startsWith('/docs/integration')) {
    return '/docs/integration';
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
  if (pathname.startsWith('/tenant/settings')) {
    return '/tenant/settings';
  }
  if (pathname.startsWith('/tenant/usage')) {
    return '/tenant/usage';
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
  if (pathname.startsWith('/semantic-cache')) {
    return '/semantic-cache/overview';
  }
  if (pathname.startsWith('/embedding-cache')) {
    return '/embedding-cache';
  }
  if (pathname.startsWith('/quality-monitor')) {
    return '/quality-monitor';
  }
  if (pathname.startsWith('/knowledge-bases')) {
    return '/knowledge-bases';
  }
  if (pathname.startsWith('/api-keys')) {
    return '/api-keys';
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
  if (pathname.startsWith('/docs')) {
    return ['/docs'];
  }
  if (pathname.startsWith('/tenant')) {
    return ['/tenant'];
  }
  if (pathname.startsWith('/cost-ops')) {
    return ['/cost-ops'];
  }
  if (pathname.startsWith('/reports')) {
    return ['/reports'];
  }
  if (pathname.startsWith('/semantic-cache') || pathname.startsWith('/embedding-cache')) {
    return ['/semantic-cache'];
  }
  return [];
}

function AdminShellInner({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { user, changePassword, logout } = useAuth();
  const { bases, selectedBase, setSelectedBaseId } = useKnowledgeBaseContext();
  const [tenantDetail, setTenantDetail] = useState<TenantDetail | null>(null);
  const [openKeys, setOpenKeys] = useState<string[]>(() => getOpenKeys(pathname));
  const [passwordModalOpen, setPasswordModalOpen] = useState(false);
  const [updatingPassword, setUpdatingPassword] = useState(false);
  const [passwordForm] = Form.useForm<{
    old_password: string;
    new_password: string;
    confirm_password: string;
  }>();

  useEffect(() => {
    let cancelled = false;

    const loadTenantSummary = async () => {
      try {
        const detail = (await apiClient.get(TENANT_API.DETAIL)) as TenantDetail;
        if (!cancelled) {
          setTenantDetail(detail);
        }
      } catch {
        if (!cancelled) {
          setTenantDetail(null);
        }
      }
    };

    void loadTenantSummary();

    return () => {
      cancelled = true;
    };
  }, []);

  const menuItems = useMemo<MenuProps['items']>(() => {
    const role = user?.role;
    const normalizedNavItems = navItems
      .filter((item) => item.key !== '/embedding-cache')
      .map((item) => {
        if (item.key === '/semantic-cache') {
          return {
            ...item,
            href: undefined,
            children: [
              { key: '/semantic-cache/overview', label: '语义缓存', href: '/semantic-cache' },
              { key: '/embedding-cache', label: 'Embedding 缓存', href: '/embedding-cache' },
            ],
          };
        }

        return item;
      });

    const filteredNavItems = normalizedNavItems
      .map((item) => {
        if (item.key === '/api-keys' && !canManageAPIKey(role)) {
          return null;
        }

        if (item.key === '/tenant') {
          const tenantChildren =
            item.children?.filter((child) => {
              if (child.key === '/tenant/settings') {
                return canViewTenantSettings(role);
              }
              if (child.key === '/tenant/usage') {
                return canViewUsage(role);
              }
              return true;
            }) ?? [];

          if (tenantChildren.length === 0) {
            return null;
          }

          return { ...item, children: tenantChildren };
        }

        return item;
      })
      .filter(Boolean) as NavItem[];

    const toMenuItem = (item: NavItem): NonNullable<MenuProps['items']>[number] => ({
      key: item.key,
      icon: item.icon,
      disabled: item.disabled,
      label: item.href ? <Link href={item.href}>{item.label}</Link> : item.label,
      children: item.children?.map((child) => toMenuItem(child)),
    });

    return filteredNavItems.map((item) => toMenuItem(item));
  }, [user?.role]);

  const breadcrumbItems = useMemo(() => {
    if (pathname.startsWith('/knowledge-bases/')) {
      return [
        { title: <Link href="/dashboard">工作台</Link> },
        { title: <Link href="/knowledge-bases">知识库</Link> },
        { title: selectedBase?.name ?? '知识库详情' },
      ];
    }
    if (pathname.startsWith('/knowledge-bases')) {
      return [{ title: <Link href="/dashboard">工作台</Link> }, { title: '知识库' }];
    }
    if (pathname.startsWith('/api-keys')) {
      return [{ title: <Link href="/dashboard">工作台</Link> }, { title: '接入密钥' }];
    }
    if (pathname.startsWith('/docs/integration')) {
      return [
        { title: <Link href="/dashboard">工作台</Link> },
        { title: '接入文档' },
        { title: '集成指南' },
      ];
    }
    if (pathname.startsWith('/retrieval-lab/debug')) {
      return [
        { title: <Link href="/dashboard">工作台</Link> },
        { title: <Link href="/retrieval-lab">检索调优</Link> },
        { title: '链路分析' },
      ];
    }
    if (pathname.startsWith('/retrieval-lab')) {
      return [{ title: <Link href="/dashboard">工作台</Link> }, { title: '检索调优' }];
    }
    if (pathname.startsWith('/trace-logs/retrieval')) {
      return [
        { title: <Link href="/dashboard">工作台</Link> },
        { title: <Link href="/trace-logs">链路追踪</Link> },
        { title: '检索日志' },
      ];
    }
    if (pathname.startsWith('/trace-logs/ingest')) {
      return [
        { title: <Link href="/dashboard">工作台</Link> },
        { title: <Link href="/trace-logs">链路追踪</Link> },
        { title: '入库日志' },
      ];
    }
    if (pathname.startsWith('/trace-logs')) {
      return [{ title: <Link href="/dashboard">工作台</Link> }, { title: '链路追踪' }];
    }
    if (pathname.startsWith('/tenant/settings')) {
      return [
        { title: <Link href="/dashboard">工作台</Link> },
        { title: '组织' },
        { title: '组织设置' },
      ];
    }
    if (pathname.startsWith('/tenant/usage')) {
      return [
        { title: <Link href="/dashboard">工作台</Link> },
        { title: '组织' },
        { title: '组织用量' },
      ];
    }
    if (pathname.startsWith('/evaluation/reports/')) {
      return [
        { title: <Link href="/dashboard">工作台</Link> },
        { title: <Link href="/evaluation/datasets">质量评测</Link> },
        { title: <Link href="/evaluation/runs">评测任务</Link> },
        { title: '评测报告' },
      ];
    }
    if (pathname.startsWith('/evaluation/runs')) {
      return [
        { title: <Link href="/dashboard">工作台</Link> },
        { title: <Link href="/evaluation/datasets">质量评测</Link> },
        { title: '评测任务' },
      ];
    }
    if (pathname.startsWith('/evaluation/datasets')) {
      return [
        { title: <Link href="/dashboard">工作台</Link> },
        { title: <Link href="/evaluation/datasets">质量评测</Link> },
        { title: '评测样本' },
      ];
    }
    if (pathname.startsWith('/evaluation')) {
      return [{ title: <Link href="/dashboard">工作台</Link> }, { title: '质量评测' }];
    }
    if (pathname.startsWith('/quality-monitor')) {
      return [{ title: <Link href="/dashboard">工作台</Link> }, { title: '质量监控' }];
    }
    if (pathname.startsWith('/strategy-center')) {
      return [{ title: <Link href="/dashboard">工作台</Link> }, { title: '策略管理' }];
    }
    if (pathname.startsWith('/cost-ops/cost')) {
      return [
        { title: <Link href="/dashboard">工作台</Link> },
        { title: '成本与运维' },
        { title: '成本分析' },
      ];
    }
    if (pathname.startsWith('/cost-ops/vector-db')) {
      return [
        { title: <Link href="/dashboard">工作台</Link> },
        { title: '成本与运维' },
        { title: '向量库运维' },
      ];
    }
    if (pathname.startsWith('/audit')) {
      return [{ title: <Link href="/dashboard">工作台</Link> }, { title: '审计中心' }];
    }
    if (pathname.startsWith('/alerts')) {
      return [{ title: <Link href="/dashboard">工作台</Link> }, { title: '告警中心' }];
    }
    if (pathname.startsWith('/reports/weekly')) {
      return [
        { title: <Link href="/dashboard">工作台</Link> },
        { title: '报告' },
        { title: '周报' },
      ];
    }
    if (pathname.startsWith('/semantic-cache')) {
      return [
        { title: <Link href="/dashboard">工作台</Link> },
        { title: '语义缓存' },
        { title: '语义缓存总览' },
      ];
    }
    if (pathname.startsWith('/embedding-cache')) {
      return [
        { title: <Link href="/dashboard">工作台</Link> },
        { title: '语义缓存' },
        { title: 'Embedding 缓存' },
      ];
    }

    return [{ title: '工作台' }];
  }, [pathname, selectedBase?.name]);

  const userMenuItems: MenuProps['items'] = [
    {
      key: 'change-password',
      icon: <LockOutlined />,
      label: '修改密码',
      onClick: () => setPasswordModalOpen(true),
    },
    {
      key: 'logout',
      icon: <LogoutOutlined />,
      label: '退出登录',
      onClick: () => logout(),
    },
  ];

  const handleBaseChange = (value?: number) => {
    setSelectedBaseId(value ?? null);
    if (value && pathname.startsWith('/knowledge-bases/')) {
      router.push(`/knowledge-bases/${value}`);
    }
  };

  const handleChangePassword = async () => {
    const values = await passwordForm.validateFields();

    try {
      setUpdatingPassword(true);
      await changePassword({
        old_password: values.old_password,
        new_password: values.new_password,
      });
      message.success('密码修改成功，请重新登录');
      setPasswordModalOpen(false);
      passwordForm.resetFields();
      logout({ redirectTo: '/login?passwordChanged=1', silent: false });
    } finally {
      setUpdatingPassword(false);
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
                  智能知识库管理平台
                </Title>
                <Text type="secondary">知识资产、检索质量与运营治理</Text>
              </div>
            </Space>
          </div>

          <div className="flex-1 px-3 py-4">
            <Menu
              mode="inline"
              selectedKeys={[getSelectedNavKey(pathname)]}
              openKeys={openKeys}
              onOpenChange={setOpenKeys}
              items={menuItems}
            />
          </div>

          <div className="border-t border-slate-200 px-5 py-4">
            <Space direction="vertical" size={4}>
              <Text type="secondary">当前平台状态：核心功能可用，持续完善中。</Text>
              <Text type="secondary">帮助入口：查看组织使用规范与常见问题。</Text>
              <Text type="secondary">接入指南入口：前往接入文档查看集成说明。</Text>
            </Space>
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
                {selectedBase ? `当前知识库: ${selectedBase.name}` : '未选择知识库'}
              </Tag>
              {user ? (
                <Space size={6}>
                  <Tag color="geekblue">{user.tenant_name || `组织 ${user.tenant_id}`}</Tag>
                  {tenantDetail?.plan ? <Tag color="purple">{tenantDetail.plan}</Tag> : null}
                  <Tag color="cyan">{user.role}</Tag>
                </Space>
              ) : null}
              <Dropdown menu={{ items: userMenuItems }} trigger={['click']}>
                <Button icon={<UserOutlined />}>{user ? user.name || user.email : '账户'}</Button>
              </Dropdown>
              <Button onClick={() => router.refresh()}>刷新</Button>
            </Space>
          </div>
        </Header>

        <Content className="bg-slate-50 p-6">{children}</Content>
      </Layout>

      <Modal
        title="修改密码"
        open={passwordModalOpen}
        okText="保存并重新登录"
        cancelText="取消"
        okButtonProps={{ loading: updatingPassword }}
        onCancel={() => {
          setPasswordModalOpen(false);
          passwordForm.resetFields();
        }}
        onOk={() => void handleChangePassword()}
      >
        <Form form={passwordForm} layout="vertical">
          <Form.Item
            label="当前密码"
            name="old_password"
            rules={[{ required: true, message: '请输入当前密码' }]}
          >
            <Input.Password autoComplete="current-password" />
          </Form.Item>

          <Form.Item
            label="新密码"
            name="new_password"
            rules={[
              { required: true, message: '请输入新密码' },
              { min: 12, message: '新密码至少需要 12 个字符' },
            ]}
          >
            <Input.Password autoComplete="new-password" />
          </Form.Item>

          <Form.Item
            label="确认新密码"
            name="confirm_password"
            dependencies={['new_password']}
            rules={[
              { required: true, message: '请再次输入新密码' },
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (!value || getFieldValue('new_password') === value) {
                    return Promise.resolve();
                  }
                  return Promise.reject(new Error('两次输入的新密码不一致'));
                },
              }),
            ]}
          >
            <Input.Password autoComplete="new-password" />
          </Form.Item>
        </Form>
      </Modal>
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
