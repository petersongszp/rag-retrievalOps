'use client';

import { useState, useMemo } from 'react';
import { Typography, Row, Col, Card as AntCard, Tag, Avatar, Button, Input, Badge } from 'antd';
import {
  DatabaseOutlined,
  CodeOutlined,
  CloudOutlined,
  RobotOutlined,
  DeploymentUnitOutlined,
  ClusterOutlined,
  ApiOutlined,
  InboxOutlined,
  SearchOutlined,
  FireFilled,
  TrophyFilled,
  CheckCircleFilled,
} from '@ant-design/icons';

const { Title, Paragraph, Text } = Typography;

type Item = {
  key: string;
  title: string;
  desc: string;
  icon: React.ReactNode;
  tags: string[];
  popular?: boolean;
  count?: number;
};

const TAGS = [
  '最受欢迎',
  'Java题库',
  'Golang题库',
  'C++题库',
  '计算机基础',
  '数据库',
  '编程语言',
  '前端题库',
  '后端组件',
  '后端工具',
  '场景设计',
  '云原生',
  'AI题库',
  'AI理论',
  'AI编程',
  '全部',
];

const DATA: Item[] = [
  {
    key: 'java-base',
    title: 'Java基础',
    desc: '聚焦Java 核心入门知识，涵盖变量与数据类型、面向对象等',
    icon: <CodeOutlined className="text-2xl" />,
    tags: ['Java题库', '编程语言'],
    popular: true,
    count: 120,
  },
  {
    key: 'java-collection',
    title: 'Java集合',
    desc: '聚焦 Java 集合框架（List、Map、Set 等）的实现与应用',
    icon: <CodeOutlined className="text-2xl" />,
    tags: ['Java题库', '编程语言'],
    count: 85,
  },
  {
    key: 'java-concurrency',
    title: 'Java并发',
    desc: '剖析 Java 多线程、锁机制、并发工具类与实战',
    icon: <CodeOutlined className="text-2xl" />,
    tags: ['Java题库'],
    count: 96,
  },
  {
    key: 'jvm',
    title: 'Java虚拟机',
    desc: '深入 JVM 内存模型、类加载、垃圾回收等核心机制',
    icon: <CodeOutlined className="text-2xl" />,
    tags: ['Java题库'],
    count: 74,
  },
  {
    key: 'go-base',
    title: 'Golang基础',
    desc: '涵盖变量类型、函数、goroutine、通道与错误处理',
    icon: <CodeOutlined className="text-2xl" />,
    tags: ['Golang题库', '编程语言'],
    popular: true,
    count: 110,
  },
  {
    key: 'go-container',
    title: 'Golang容器',
    desc: '探索 Go 语言容器（数组、切片、映射等）的实现与性能',
    icon: <CodeOutlined className="text-2xl" />,
    tags: ['Golang题库'],
    count: 45,
  },
  {
    key: 'go-concurrency',
    title: 'Golang并发',
    desc: '围绕 Goroutine、Channel、同步原语等并发模型',
    icon: <CodeOutlined className="text-2xl" />,
    tags: ['Golang题库'],
    count: 68,
  },
  {
    key: 'go-gc',
    title: 'GolangGC',
    desc: '剖析 Go 垃圾回收机制、GC 算法与调优',
    icon: <CodeOutlined className="text-2xl" />,
    tags: ['Golang题库'],
    count: 32,
  },
  {
    key: 'cpp',
    title: 'C++',
    desc: '涵盖面向对象、STL 容器、内存管理等核心知识',
    icon: <CodeOutlined className="text-2xl" />,
    tags: ['C++题库', '编程语言'],
    count: 150,
  },
  {
    key: 'redis',
    title: 'Redis',
    desc: '覆盖 Redis 数据结构、缓存策略、集群架构与持久化',
    icon: <DatabaseOutlined className="text-2xl" />,
    tags: ['数据库', '后端组件'],
    popular: true,
    count: 200,
  },
  {
    key: 'mysql',
    title: 'MySQL',
    desc: '涵盖 SQL 优化、索引设计、事务机制、锁与隔离级别',
    icon: <DatabaseOutlined className="text-2xl" />,
    tags: ['数据库'],
    count: 180,
  },
  {
    key: 'mq',
    title: '消息队列',
    desc: '包含 RabbitMQ、Kafka 等主流组件知识',
    icon: <ClusterOutlined className="text-2xl" />,
    tags: ['后端组件'],
    count: 90,
  },
  {
    key: 'ssm',
    title: 'SSM全家桶',
    desc: '涵盖 Spring IoC AOP、Spring MVC 请求流程与实战',
    icon: <DeploymentUnitOutlined className="text-2xl" />,
    tags: ['后端组件'],
    count: 130,
  },
  {
    key: 'os',
    title: '操作系统',
    desc: '解析进程线程、内存调度、文件系统、IO 等',
    icon: <InboxOutlined className="text-2xl" />,
    tags: ['计算机基础'],
    count: 115,
  },
  {
    key: 'network',
    title: '计算机网络',
    desc: '梳理 TCP/IP 协议栈、网络分层、链路与安全',
    icon: <ApiOutlined className="text-2xl" />,
    tags: ['计算机基础'],
    count: 140,
  },
  {
    key: 'scenario',
    title: '场景题',
    desc: '聚焦高并发、数据库存储、缓存策略等核心场景题',
    icon: <CloudOutlined className="text-2xl" />,
    tags: ['场景设计'],
    count: 60,
  },
  {
    key: 'cloud-native',
    title: '云原生',
    desc: '容器编排、服务治理、可观测性与CI/CD流水线',
    icon: <CloudOutlined className="text-2xl" />,
    tags: ['云原生'],
    count: 75,
  },
  {
    key: 'ai-coding',
    title: 'AI编程',
    desc: '模型调用、提示工程、工具函数与自动化工作流',
    icon: <RobotOutlined className="text-2xl" />,
    tags: ['AI编程', 'AI题库'],
    count: 40,
  },
  {
    key: 'ai-theory',
    title: 'AI理论',
    desc: '机器学习基础、优化算法、深度学习核心概念',
    icon: <RobotOutlined className="text-2xl" />,
    tags: ['AI理论', 'AI题库'],
    count: 55,
  },
];

// Helper to get color based on tag/type
const getIconColorClass = (key: string) => {
  if (key.includes('java')) return 'bg-red-50 text-red-600';
  if (key.includes('go')) return 'bg-blue-50 text-blue-600';
  if (key.includes('cpp')) return 'bg-indigo-50 text-indigo-600';
  if (key.includes('redis') || key.includes('mysql')) return 'bg-green-50 text-green-600';
  if (key.includes('ai')) return 'bg-purple-50 text-purple-600';
  return 'bg-slate-100 text-slate-600';
};

export default function QuestionsPage() {
  const [active, setActive] = useState<string>('最受欢迎');
  const [search, setSearch] = useState('');

  const filtered = useMemo(() => {
    let res = DATA;
    if (active !== '全部') {
      if (active === '最受欢迎') res = res.filter((i) => i.popular);
      else if (active === '编程语言')
        res = res.filter(
          (i) =>
            i.tags.includes('Java题库') ||
            i.tags.includes('Golang题库') ||
            i.tags.includes('C++题库')
        );
      else res = res.filter((i) => i.tags.includes(active));
    }

    if (search) {
      res = res.filter(
        (i) =>
          i.title.toLowerCase().includes(search.toLowerCase()) ||
          i.desc.toLowerCase().includes(search.toLowerCase())
      );
    }
    return res;
  }, [active, search]);

  return (
    <div className="min-h-screen bg-slate-50 font-sans relative pb-20">
      {/* Decorative Background */}
      <div className="fixed top-0 left-0 w-[600px] h-[600px] bg-emerald-50/60 rounded-full blur-[120px] -translate-x-1/2 -translate-y-1/2 pointer-events-none z-0" />
      <div className="fixed bottom-0 right-0 w-[600px] h-[600px] bg-teal-50/60 rounded-full blur-[120px] translate-x-1/3 translate-y-1/3 pointer-events-none z-0" />

      <div className="container mx-auto px-4 relative z-10 pt-12">
        {/* Header Section */}
        <div className="mb-10 animate-fade-in-up">
          <div className="flex flex-col md:flex-row md:items-end justify-between gap-6">
            <div>
              <h1 className="text-3xl font-extrabold text-slate-900 tracking-tight flex items-center gap-3">
                <DatabaseOutlined className="text-emerald-600" />
                专项面试题库
              </h1>
              <p className="text-slate-500 mt-2 text-lg max-w-2xl">
                精选海量真实面试题，覆盖主流技术栈与核心考点。
              </p>
            </div>

            <div className="w-full md:w-72">
              <Input
                size="large"
                prefix={<SearchOutlined className="text-slate-400" />}
                placeholder="搜索题库..."
                className="!rounded-full !bg-white !border-slate-200 hover:!border-emerald-400 focus:!border-emerald-500 !shadow-sm"
                onChange={(e) => setSearch(e.target.value)}
              />
            </div>
          </div>
        </div>

        {/* Filter Section */}
        <div className="mb-8 animate-fade-in-up" style={{ animationDelay: '0.1s' }}>
          <div className="bg-white rounded-2xl p-6 border border-slate-100 shadow-lg shadow-slate-200/50">
            <div className="flex flex-wrap gap-2">
              {TAGS.map((t) => (
                <button
                  key={t}
                  onClick={() => setActive(t)}
                  className={`
                      px-4 py-2 rounded-full text-sm font-medium transition-all duration-200 border
                      ${
                        active === t
                          ? 'bg-emerald-600 text-white border-emerald-600 shadow-md shadow-emerald-200'
                          : 'bg-slate-50 text-slate-600 border-slate-200 hover:bg-white hover:border-emerald-300 hover:text-emerald-600'
                      }
                    `}
                >
                  {t === '最受欢迎' && (
                    <FireFilled
                      className={`mr-1.5 ${active === t ? 'text-white' : 'text-orange-500'}`}
                    />
                  )}
                  {t}
                </button>
              ))}
            </div>
          </div>
        </div>

        {/* Grid */}
        <div className="animate-fade-in-up" style={{ animationDelay: '0.2s' }}>
          <Row gutter={[24, 24]}>
            {filtered.map((item, idx) => (
              <Col xs={24} md={12} lg={8} key={item.key}>
                <div className="group h-full bg-white rounded-2xl p-6 border border-slate-100 shadow-sm hover:shadow-xl hover:shadow-slate-200/50 transition-all duration-300 hover:-translate-y-1 cursor-pointer relative overflow-hidden">
                  {/* Hover Gradient */}
                  <div className="absolute top-0 left-0 w-full h-1 bg-gradient-to-r from-emerald-400 to-teal-500 transform scale-x-0 group-hover:scale-x-100 transition-transform duration-300 origin-left" />

                  <div className="flex items-start gap-4">
                    <div
                      className={`w-14 h-14 rounded-2xl flex items-center justify-center transition-colors ${getIconColorClass(item.key)}`}
                    >
                      {item.icon}
                    </div>
                    <div className="flex-1">
                      <div className="flex items-center justify-between mb-2">
                        <h3 className="text-lg font-bold text-slate-800 group-hover:text-emerald-700 transition-colors">
                          {item.title}
                        </h3>
                        {item.popular && (
                          <Tag
                            color="orange"
                            className="border-0 flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-bold bg-orange-50 text-orange-600 m-0"
                          >
                            <FireFilled /> 热门
                          </Tag>
                        )}
                      </div>
                      <p className="text-slate-500 text-sm leading-relaxed line-clamp-2 mb-4 h-10">
                        {item.desc}
                      </p>

                      <div className="flex items-center justify-between pt-4 border-t border-slate-50">
                        <div className="flex items-center gap-2 text-xs text-slate-400">
                          <DatabaseOutlined />
                          <span>{item.count || 50}+ 题目</span>
                        </div>
                        <div className="opacity-0 group-hover:opacity-100 transition-opacity text-emerald-600 text-sm font-medium flex items-center gap-1">
                          开始练习 <CheckCircleFilled />
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </Col>
            ))}

            {filtered.length === 0 && (
              <Col span={24}>
                <div className="text-center py-20 bg-white rounded-3xl border border-slate-100 border-dashed">
                  <div className="w-16 h-16 bg-slate-50 rounded-full flex items-center justify-center mx-auto mb-4 text-slate-300 text-2xl">
                    <SearchOutlined />
                  </div>
                  <p className="text-slate-500">未找到相关题库</p>
                </div>
              </Col>
            )}
          </Row>
        </div>
      </div>
    </div>
  );
}
