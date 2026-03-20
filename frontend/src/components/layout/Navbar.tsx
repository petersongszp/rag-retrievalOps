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
import { USER_API } from '@/config/api';


const { Header } = Layout;
const { Title } = Typography;

const Navbar: FC = () => {
  const router = useRouter();
  const { user, isAuthenticated: authed, login, logout: authLogout } = useAuth();
  const [openAuth, setOpenAuth] = useState(false);
  const [activeKey, setActiveKey] = useState<'login' | 'register' | 'forgot'>('login');
  const [loginForm] = Form.useForm();
  const [registerForm] = Form.useForm();
  const [forgotPasswordForm] = Form.useForm();
  const [guideModalOpen, setGuideModalOpen] = useState(false);
  const [forgotLoading, setForgotLoading] = useState(false);
  const [githubLoading, setGithubLoading] = useState(false);
  const [googleLoading, setGoogleLoading] = useState(false);

  
  

  const doLogin = async (values: { email: string; password: string }) => {
    try {
      const res: any = await apiClient.post(USER_API.LOGIN, values);
      const data = res?.data || res;
      const token = data?.token || data?.accessToken;
      if (!token) {
        message.error("Login Failed" + '：' + "Please enter your email"); // Using placeholder as generic error or keep generic
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
      message.success("Login Successful");
    } catch (e: any) {
      message.error(e?.response?.data?.message || "Login Failed");
    }
  };

  const doRegister = async (values: { username: string; email: string; password: string }) => {
    try {
      const data: any = await apiClient.post(USER_API.REGISTER, values);
      const token = data?.token;
      const userData = data?.user;
      if (!token || !userData) {
        message.error("Registration Failed");
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
      message.success("Registration Successful");
    } catch (e: any) {
      message.error(e?.response?.data?.message || "Registration Failed");
    }
  };

  const doForgotPassword = async (values: { email: string }) => {
    setForgotLoading(true);
    try {
      await apiClient.post('/user/password/forgot', values);
      message.success("Reset link has been sent to your email, please check.");
      setActiveKey('login');
    } catch (e: any) {
      message.error(e?.response?.data?.message || "Failed to send.");
    } finally {
      setForgotLoading(false);
    }
  };

  const doGitHubLogin = async () => {
    setGithubLoading(true);
    try {
      const res: any = await apiClient.get(USER_API.GITHUB_LOGIN);
      const loginUrl = res?.login_url || res?.data?.login_url;
      if (!loginUrl) {
        message.error("Failed to get GitHub login URL.");
        return;
      }
      window.location.href = loginUrl;
    } catch (e: any) {
      message.error(e?.response?.data?.message || e?.message || "GitHub login failed.");
      setGithubLoading(false);
    }
  };

  const doGoogleLogin = async () => {
    setGoogleLoading(true);
    try {
      const res: any = await apiClient.get('/user/google/login');
      const loginUrl = res?.login_url || res?.data?.login_url;
      if (!loginUrl) {
        message.error('获取 Google 登录地址失败');
        setGoogleLoading(false);
        return;
      }
      window.location.href = loginUrl;
    } catch (e: any) {
      message.error(e?.response?.data?.message || e?.message || 'Google 登录失败');
      setGoogleLoading(false);
    }
  };

  const logout = async () => {
    try {
      await apiClient.post('/user/logout', {});
    } catch (e) { }
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    authLogout();
    message.success("Logout");
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
              INTERVIEW MASTER
            </span>
          </div>
        </div>

        <nav className="hidden md:flex items-center gap-8">
          <Link
            href="/"
            className="text-sm font-medium text-slate-600 hover:text-blue-600 transition-colors relative group"
          >
            {"Home"}
            <span className="absolute -bottom-1 left-0 w-0 h-0.5 bg-blue-600 transition-all group-hover:w-full" />
          </Link>
          <Link
            href="/resume"
            className="text-sm font-medium text-slate-600 hover:text-blue-600 transition-colors relative group"
          >
            {"Resume Prediction"}
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
                        <span className="font-medium">{"Experienced Interview"}</span>
                        <span className="text-xs text-slate-400">{"Deep interview for experienced professionals"}</span>
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
                        <span className="font-medium">{"Campus Interview"}</span>
                        <span className="text-xs text-slate-400">{"Basic interview for fresh graduates"}</span>
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
              {"Comprehensive Interview"}{' '}
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
            {"Multi-Agent Interview"}
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
            {"Specialized Interview"}
            <span className="absolute -bottom-1 left-0 w-0 h-0.5 bg-blue-600 transition-all group-hover:w-full" />
          </Link>
          <Link
            href="https://awq7m8b63wy.feishu.cn/wiki/Cl8mwzOayiTtaZknRU2cyoFHndL"
            target="_blank"
            className="text-sm font-medium text-slate-600 hover:text-blue-600 transition-colors relative group"
          >
            {"User Manual"}
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
                  { key: 'center', label: <Link href="/user/center">{"User Center"}</Link> },
                  { key: 'interviews', label: <Link href="/user/interviews">{"Interview Records"}</Link> },
                  { key: 'press', label: <Link href="/user/press">{"Prediction Records"}</Link> },
                  { key: 'notes', label: <Link href="/user/notes">{"Note List"}</Link> },
                  { key: 'models', label: <Link href="/user/models">{"User Models"}</Link> },
                  { type: 'divider' },
                  {
                    key: 'logout',
                    label: (
                      <a onClick={logout} className="text-red-500">
                        {"Logout"}
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
                  {user?.username || user?.email?.split("@")[0] || "User"}
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
              {"Login / Register"}
            </Button>
          )}
        </div>
      </div>
      <Modal
        open={openAuth}
        onCancel={() => setOpenAuth(false)}
        footer={null}
        title={"Account Login / Register"}
        destroyOnClose
      >
        <Tabs
          activeKey={activeKey}
          onChange={(k) => setActiveKey(k as 'login' | 'register' | 'forgot')}
          items={[
            {
              key: 'login',
              label: "Login",
              children: (
                <Form
                  form={loginForm}
                  layout="vertical"
                  onFinish={doLogin}
                  initialValues={{ email: '', password: '' }}
                >
                  <Form.Item
                    label={"Email"}
                    name="email"
                    rules={[
                      { required: true, message: "Please enter your email" },
                      { type: 'email', message: 'Invalid email' },
                    ]}
                  >
                    <Input placeholder={"Please enter your email"} />
                  </Form.Item>
                  <Form.Item
                    label={"Password"}
                    name="password"
                    rules={[{ required: true, message: "Please enter your password" }]}
                  >
                    <Input.Password placeholder={"Please enter your password"} />
                  </Form.Item>
                  <div className="flex justify-end mb-4">
                    <a
                      className="text-sm text-blue-600 hover:text-blue-800"
                      onClick={(e) => {
                        e.preventDefault();
                        setActiveKey('forgot');
                      }}
                    >
                      {"Forgot password?"}
                    </a>
                  </div>
                  <Button type="primary" htmlType="submit" className="w-full">
                    {"Login"}
                  </Button>
                  <div className="mt-4 pt-4 border-t border-slate-100">
                    <div className="text-center text-xs text-slate-400 mb-2">或使用第三方登录</div>
                    <div className="flex flex-col gap-2">
                      <Button
                        type="default"
                        className="w-full flex items-center justify-center gap-2"
                        loading={googleLoading}
                        disabled={githubLoading}
                        onClick={doGoogleLogin}
                      >
                        <svg className="w-5 h-5" viewBox="0 0 24 24" aria-hidden="true">
                          <path
                            fill="#4285F4"
                            d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"
                          />
                          <path
                            fill="#34A853"
                            d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
                          />
                          <path
                            fill="#FBBC05"
                            d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"
                          />
                          <path
                            fill="#EA4335"
                            d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"
                          />
                        </svg>
                        Google 登录
                      </Button>
                      <Button
                        type="default"
                        className="w-full flex items-center justify-center gap-2"
                        loading={githubLoading}
                        disabled={googleLoading}
                        onClick={doGitHubLogin}
                      >
                        <svg className="w-5 h-5" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                          <path
                            fillRule="evenodd"
                            d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z"
                            clipRule="evenodd"
                          />
                        </svg>
                        GitHub 登录
                      </Button>
                    </div>
                  </div>
                </Form>
              ),
            },
            {
              key: 'register',
              label: "Register",
              children: (
                <Form
                  form={registerForm}
                  layout="vertical"
                  onFinish={doRegister}
                  initialValues={{ username: '', email: '', password: '' }}
                >
                  <Form.Item
                    label={"Username"}
                    name="username"
                    rules={[{ required: true, message: "Please enter your username" }]}
                  >
                    <Input placeholder={"Please enter your username"} />
                  </Form.Item>
                  <Form.Item
                    label={"Email"}
                    name="email"
                    rules={[
                      { required: true, message: "Please enter your email" },
                      { type: 'email', message: 'Invalid email' },
                    ]}
                  >
                    <Input placeholder={"Please enter your email"} />
                  </Form.Item>
                  <Form.Item
                    label={"Password"}
                    name="password"
                    rules={[{ required: true, message: "Please enter your password" }]}
                  >
                    <Input.Password placeholder={"Please enter your password"} />
                  </Form.Item>
                  <Button type="primary" htmlType="submit" className="w-full">
                    {"Register and Login"}
                  </Button>
                  <div className="mt-4 pt-4 border-t border-slate-100">
                    <div className="text-center text-xs text-slate-400 mb-2">或使用第三方登录</div>
                    <div className="flex flex-col gap-2">
                      <Button
                        type="default"
                        className="w-full flex items-center justify-center gap-2"
                        loading={googleLoading}
                        disabled={githubLoading}
                        onClick={doGoogleLogin}
                      >
                        <svg className="w-5 h-5" viewBox="0 0 24 24" aria-hidden="true">
                          <path
                            fill="#4285F4"
                            d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"
                          />
                          <path
                            fill="#34A853"
                            d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
                          />
                          <path
                            fill="#FBBC05"
                            d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"
                          />
                          <path
                            fill="#EA4335"
                            d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"
                          />
                        </svg>
                        Google 登录
                      </Button>
                      <Button
                        type="default"
                        className="w-full flex items-center justify-center gap-2"
                        loading={githubLoading}
                        disabled={googleLoading}
                        onClick={doGitHubLogin}
                      >
                        <svg className="w-5 h-5" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                          <path
                            fillRule="evenodd"
                            d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z"
                            clipRule="evenodd"
                          />
                        </svg>
                        GitHub 登录
                      </Button>
                    </div>
                  </div>
                </Form>
              ),
            },
            {
              key: 'forgot',
              label: "Forgot Password",
              children: (
                <Form
                  form={forgotPasswordForm}
                  layout="vertical"
                  onFinish={doForgotPassword}
                  initialValues={{ email: '' }}
                >
                  <Form.Item
                    label={"Email"}
                    name="email"
                    rules={[
                      { required: true, message: "Please enter your email" },
                      { type: 'email', message: 'Invalid email' },
                    ]}
                  >
                    <Input placeholder={"Please enter your email"} />
                  </Form.Item>
                  <Button type="primary" htmlType="submit" loading={forgotLoading} className="w-full mb-4">
                    {"Send Reset Link"}
                  </Button>
                  <div className="text-center">
                    <a
                      className="text-sm text-slate-500 hover:text-slate-700"
                      onClick={(e) => {
                        e.preventDefault();
                        setActiveKey('login');
                      }}
                    >
                      {"Back to Login"}
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
        title={"Welcome to Interview Master"}
        centered
        width={600}
      >
        <div className="py-6 px-4">
          <div className="mb-8 text-center">
            <Title level={4}>{"Just two simple steps to let AI customize your interview plan"}</Title>
            <Typography.Text type="secondary">
              {"Just two simple steps to let AI customize your interview plan"}
            </Typography.Text>
          </div>

          <Steps
            direction="vertical"
            current={0}
            items={[
              {
                title: "Step 1: Configure User Model",
                description: "Configure your large model key (Volcano, Bailian have free models), AI will generate questions based on your model.",
              },
              {
                title: "Step 2: Upload Resume",
                description: "Go to personal center to upload resume, AI will generate targeted questions based on your resume.",
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
              {"Configure Model Now"}
            </Button>
          </div>
        </div>
      </Modal>
    </Header>
  );
};

export default Navbar;
