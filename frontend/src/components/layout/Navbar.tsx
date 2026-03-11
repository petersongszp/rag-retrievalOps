'use client';

import {
  Layout,
  Typography,
  Button,
  Badge,
  Dropdown,
  Modal,
  Tabs,
  Form,
  Input,
  message,
  Steps,
} from 'antd';
import Link from 'next/link';
import { BellOutlined, UserOutlined, DownOutlined, TeamOutlined } from '@ant-design/icons';
import type { FC } from 'react';
import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import apiClient from '@/services/api/client';
import { useAuth } from '@/hooks/useAuth';

const { Header } = Layout;
const { Title } = Typography;

const Navbar: FC = () => {
  const router = useRouter();
  const { user, isAuthenticated: authed, login, logout: authLogout } = useAuth();
  const [openAuth, setOpenAuth] = useState(false);
  const [activeKey, setActiveKey] = useState<'login' | 'register' | 'forgot'>('login');
  // const [authed, setAuthed] = useState(false);
  // const [user, setUser] = useState<{ username?: string; email?: string } | null>(null);
  const [loginForm] = Form.useForm();
  const [registerForm] = Form.useForm();
  const [forgotPasswordForm] = Form.useForm();
  const [guideModalOpen, setGuideModalOpen] = useState(false);
  const [forgotLoading, setForgotLoading] = useState(false);

  // useEffect(() => {
  //   const token = typeof window !== 'undefined' ? localStorage.getItem('token') : null;
  //   const u = typeof window !== 'undefined' ? localStorage.getItem('user') : null;
  //   setAuthed(!!token);
  //   setUser(u ? JSON.parse(u) : null);
  // }, []);

  const doLogin = async (values: { email: string; password: string }) => {
    try {
      const res: any = await apiClient.post('/user/login', values);
      const data = res?.data || res;
      const token = data?.token || data?.accessToken;
      if (!token) {
        message.error('登录失败：缺少令牌');
        return;
      }
      localStorage.setItem('token', token);
      try {
        document.cookie = `token=${token};path=/;max-age=${60 * 60 * 24}`;
      } catch { }
      if (data?.user) {
        localStorage.setItem('user', JSON.stringify(data.user));
        login(data.user);
      } else {
        localStorage.setItem('user', JSON.stringify({ email: values.email }));
        login({ id: '0', email: values.email, name: values.email, username: values.email });
      }
      setOpenAuth(false);
      message.success('登录成功');
    } catch (e: any) {
      message.error(e?.response?.data?.message || '登录失败');
    }
  };

  const doRegister = async (values: { username: string; email: string; password: string }) => {
    try {
      const data: any = await apiClient.post('/user/register', values);
      const token = data?.token;
      const userData = data?.user;
      if (!token || !userData) {
        message.error('注册失败：返回数据缺失');
        return;
      }
      localStorage.setItem('token', token);
      try {
        document.cookie = `token=${token};path=/;max-age=${60 * 60 * 24}`;
      } catch { }
      localStorage.setItem('user', JSON.stringify(userData));
      login(userData);
      setOpenAuth(false);
      setGuideModalOpen(true);
      message.success('注册并登录成功');
    } catch (e: any) {
      message.error(e?.response?.data?.message || '注册失败');
    }
  };

  const doForgotPassword = async (values: { email: string }) => {
    setForgotLoading(true);
    try {
      await apiClient.post('/user/password/forgot', values);
      message.success('重置链接已发送到您的邮箱，请查收');
      setActiveKey('login');
    } catch (e: any) {
      message.error(e?.response?.data?.message || '发送失败');
    } finally {
      setForgotLoading(false);
    }
  };

  const logout = async () => {
    try {
      await apiClient.post('/user/logout', {});
    } catch (e) { }
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    authLogout();
    message.success('已退出登录');
    router.push('/');
  };

  return (
    <Header className="sticky top-0 z-50 w-full bg-white/80 backdrop-blur-md border-b border-slate-200/60 transition-all duration-300">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-full flex items-center justify-between">
        <div className="flex items-center gap-3 cursor-pointer" onClick={() => router.push('/')}>
          <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-blue-600 to-indigo-600 flex items-center justify-center shadow-lg shadow-blue-200">
            <span className="text-white text-xl font-bold">面</span>
          </div>
          <div className="flex flex-col justify-center h-10">
            <span className="text-lg font-bold bg-clip-text text-transparent bg-gradient-to-r from-slate-800 to-slate-600 leading-none mb-0.5 pt-1">
              面试吧
            </span>
            <span className="text-[10px] text-slate-500 tracking-wider uppercase font-medium leading-none scale-90 origin-left">
              Interview Master
            </span>
          </div>
        </div>

        <nav className="hidden md:flex items-center gap-8">
          <Link
            href="/"
            className="text-sm font-medium text-slate-600 hover:text-blue-600 transition-colors relative group"
          >
            首页
            <span className="absolute -bottom-1 left-0 w-0 h-0.5 bg-blue-600 transition-all group-hover:w-full" />
          </Link>
          <Link
            href="/resume"
            className="text-sm font-medium text-slate-600 hover:text-blue-600 transition-colors relative group"
          >
            简历押题
            <span className="absolute -bottom-1 left-0 w-0 h-0.5 bg-blue-600 transition-all group-hover:w-full" />
          </Link>
          <Dropdown
            menu={{
              items: [
                {
                  key: 'social',
                  label: (
                    <Link href="/interview/social" className="flex items-center gap-2 py-1">
                      <div className="w-8 h-8 rounded-lg bg-blue-50 flex items-center justify-center text-blue-600">
                        <UserOutlined />
                      </div>
                      <div className="flex flex-col">
                        <span className="font-medium">社招简历面试</span>
                        <span className="text-xs text-slate-400">针对社招人员的深度面试</span>
                      </div>
                    </Link>
                  ),
                },
                {
                  key: 'campus',
                  label: (
                    <Link href="/interview/campus" className="flex items-center gap-2 py-1">
                      <div className="w-8 h-8 rounded-lg bg-green-50 flex items-center justify-center text-green-600">
                        <TeamOutlined />
                      </div>
                      <div className="flex flex-col">
                        <span className="font-medium">校招简历面试</span>
                        <span className="text-xs text-slate-400">针对应届生的基础面试</span>
                      </div>
                    </Link>
                  ),
                },
              ],
              className: 'p-2',
            }}
            overlayClassName="pt-2"
          >
            <a className="text-sm font-medium text-slate-600 hover:text-blue-600 transition-colors flex items-center gap-1 cursor-pointer group">
              综合面试{' '}
              <DownOutlined className="text-xs transition-transform group-hover:rotate-180" />
              <Badge
                count={'HOT'}
                color="#fa541c"
                offset={[10, -8]}
                className="scale-75 origin-left"
              />
            </a>
          </Dropdown>
          <Link
            href="/interview/multi"
            className="text-sm font-medium text-slate-600 hover:text-blue-600 transition-colors relative group"
          >
            多人面试
            <span className="absolute -bottom-1 left-0 w-0 h-0.5 bg-blue-600 transition-all group-hover:w-full" />
            <Badge
              count={'New'}
              color="#faad14"
              offset={[10, -8]}
              className="scale-75 origin-left"
            />
          </Link>
          <Link
            href="/interview/special"
            className="text-sm font-medium text-slate-600 hover:text-blue-600 transition-colors relative group"
          >
            专项面试
            <span className="absolute -bottom-1 left-0 w-0 h-0.5 bg-blue-600 transition-all group-hover:w-full" />
          </Link>
          <Link
            href="https://awq7m8b63wy.feishu.cn/wiki/Cl8mwzOayiTtaZknRU2cyoFHndL"
            target="_blank"
            className="text-sm font-medium text-slate-600 hover:text-blue-600 transition-colors relative group"
          >
            使用手册
            <span className="absolute -bottom-1 left-0 w-0 h-0.5 bg-blue-600 transition-all group-hover:w-full" />
          </Link>
        </nav>

        <div className="flex items-center gap-4">
          <Button
            type="text"
            shape="circle"
            icon={<BellOutlined className="text-slate-600 text-lg" />}
            className="hover:bg-slate-100 flex items-center justify-center"
          />
          {authed ? (
            <Dropdown
              trigger={['hover']}
              menu={{
                items: [
                  { key: 'center', label: <Link href="/user/center">个人中心</Link> },
                  { key: 'interviews', label: <Link href="/user/interviews">面试记录</Link> },
                  { key: 'press', label: <Link href="/user/press">押题记录</Link> },
                  { key: 'notes', label: <Link href="/user/notes">笔记列表</Link> },
                  { key: 'models', label: <Link href="/user/models">用户模型</Link> },
                  { type: 'divider' },
                  {
                    key: 'logout',
                    label: (
                      <a onClick={logout} className="text-red-500">
                        退出登录
                      </a>
                    ),
                  },
                ],
                className: 'w-40',
              }}
            >
              <Button className="border-slate-200 hover:border-blue-400 hover:text-blue-600 px-4 h-9 rounded-full flex items-center gap-2 transition-all">
                <UserOutlined />
                <span className="max-w-[100px] truncate">
                  {user?.username || user?.email?.split('@')[0] || '用户'}
                </span>
              </Button>
            </Dropdown>
          ) : (
            <Button
              type="primary"
              onClick={() => {
                setActiveKey('login');
                setOpenAuth(true);
              }}
              className="bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-500 hover:to-indigo-500 border-0 h-9 px-6 rounded-full shadow-lg shadow-blue-200 font-medium transition-all hover:scale-105"
            >
              登录 / 注册
            </Button>
          )}
        </div>
      </div>
      <Modal
        open={openAuth}
        onCancel={() => setOpenAuth(false)}
        footer={null}
        title="账号登录 / 注册"
        destroyOnClose
      >
        <Tabs
          activeKey={activeKey}
          onChange={(k) => setActiveKey(k as 'login' | 'register' | 'forgot')}
          items={[
            {
              key: 'login',
              label: '登录',
              children: (
                <Form
                  form={loginForm}
                  layout="vertical"
                  onFinish={doLogin}
                  initialValues={{ email: '', password: '' }}
                >
                  <Form.Item
                    label="邮箱"
                    name="email"
                    rules={[
                      { required: true, message: '请输入邮箱' },
                      { type: 'email', message: '请输入有效的邮箱格式' },
                    ]}
                  >
                    <Input placeholder="请输入邮箱" />
                  </Form.Item>
                  <Form.Item
                    label="密码"
                    name="password"
                    rules={[{ required: true, message: '请输入密码' }]}
                  >
                    <Input.Password placeholder="请输入密码" />
                  </Form.Item>
                  <div className="flex justify-end mb-4">
                    <a
                      className="text-sm text-blue-600 hover:text-blue-800"
                      onClick={(e) => {
                        e.preventDefault();
                        setActiveKey('forgot');
                      }}
                    >
                      忘记密码？
                    </a>
                  </div>
                  <Button type="primary" htmlType="submit" className="w-full">
                    登录
                  </Button>
                </Form>
              ),
            },
            {
              key: 'register',
              label: '注册',
              children: (
                <Form
                  form={registerForm}
                  layout="vertical"
                  onFinish={doRegister}
                  initialValues={{ username: '', email: '', password: '' }}
                >
                  <Form.Item
                    label="用户名"
                    name="username"
                    rules={[{ required: true, message: '请输入用户名' }]}
                  >
                    <Input placeholder="请输入用户名" />
                  </Form.Item>
                  <Form.Item
                    label="邮箱"
                    name="email"
                    rules={[
                      { required: true, message: '请输入邮箱' },
                      { type: 'email', message: '请输入有效的邮箱格式' },
                    ]}
                  >
                    <Input placeholder="请输入邮箱" />
                  </Form.Item>
                  <Form.Item
                    label="密码"
                    name="password"
                    rules={[{ required: true, message: '请输入密码' }]}
                  >
                    <Input.Password placeholder="请输入密码" />
                  </Form.Item>
                  <Button type="primary" htmlType="submit" className="w-full">
                    注册并登录
                  </Button>
                </Form>
              ),
            },
            {
              key: 'forgot',
              label: '找回密码',
              children: (
                <Form
                  form={forgotPasswordForm}
                  layout="vertical"
                  onFinish={doForgotPassword}
                  initialValues={{ email: '' }}
                >
                  <Form.Item
                    label="邮箱"
                    name="email"
                    rules={[
                      { required: true, message: '请输入邮箱' },
                      { type: 'email', message: '请输入有效的邮箱格式' },
                    ]}
                  >
                    <Input placeholder="请输入注册时的邮箱" />
                  </Form.Item>
                  <Button type="primary" htmlType="submit" loading={forgotLoading} className="w-full mb-4">
                    发送重置链接
                  </Button>
                  <div className="text-center">
                    <a
                      className="text-sm text-slate-500 hover:text-slate-700"
                      onClick={(e) => {
                        e.preventDefault();
                        setActiveKey('login');
                      }}
                    >
                      返回登录
                    </a>
                  </div>
                </Form>
              ),
            },
          ]}
        />
      </Modal>

      <Modal
        open={guideModalOpen}
        onCancel={() => setGuideModalOpen(false)}
        footer={null}
        title="欢迎加入面试吧"
        centered
        width={600}
      >
        <div className="py-6 px-4">
          <div className="mb-8 text-center">
            <Title level={4}>开启您的智能面试之旅</Title>
            <Typography.Text type="secondary">
              只需简单两步，让 AI 为您定制专属面试计划
            </Typography.Text>
          </div>

          <Steps
            direction="vertical"
            current={0}
            items={[
              {
                title: '第一步：配置用户模型',
                description:
                  '配置您的大模型key(火山、百炼都有免费大模型)，AI 将根据您的模型生成面试题目。',
              },
              {
                title: '第二步：上传个人简历',
                description: '前往个人中心上传简历，AI 将根据您的简历内容生成针对性的面试题目。',
              },
            ]}
          />

          <div className="mt-8 flex justify-center">
            <Button
              type="primary"
              size="large"
              onClick={() => {
                setGuideModalOpen(false);
                router.push('/user/models');
              }}
              className="w-full md:w-auto px-8"
            >
              立即去配置用户模型
            </Button>
          </div>
        </div>
      </Modal>
    </Header>
  );
};

export default Navbar;
