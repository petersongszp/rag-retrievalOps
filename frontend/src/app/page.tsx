'use client';

import { Typography, Button, Card as AntCard, Rate, Collapse, Avatar, Tag, Badge } from 'antd';
import {
  BulbOutlined,
  FileTextOutlined,
  CodeOutlined,
  CompassOutlined,
  FlagOutlined,
  EyeOutlined,
  ThunderboltOutlined,
  SmileOutlined,
  SwitcherOutlined,
  SendOutlined,
  TeamOutlined,
  ExperimentOutlined,
  UserOutlined,
  RocketOutlined,
  TrophyOutlined,
  FireOutlined,
  CheckCircleFilled,
  RightOutlined,
  PlayCircleFilled,
} from '@ant-design/icons';
import Link from 'next/link';
import type { FC } from 'react';


const { Title, Paragraph, Text } = Typography;

export default function Home() {
  

  const testimonials = [
    {
      text: "我不是科班出身，基础一直比较薄弱。简历押题功能非常强，我真实面试里 90% 的问题都被命中了，还帮我把依赖注入到面向切面的知识体系串了起来。",
      user: "35+ 转行重启",
      title: "社招后端开发",
      avatar: 'https://api.dicebear.com/7.x/avataaars/svg?seed=a',
    },
    {
      text: "996 的节奏下根本没时间约线下模拟，这个平台效率很高而且 24 小时可用。面试官问到系统设计和逻辑题时，平台里都练到过，最终面试有 70% 都是类似问题。",
      user: "小博",
      title: "后端开发工程师",
      avatar: 'https://api.dicebear.com/7.x/avataaars/svg?seed=b',
    },
    {
      text: "追问环节显著提升了我讲清技术细节的能力。像内存数据库持久化和性能优化都挖得很深，正是高级岗位最看重的内容。",
      user: "乐思",
      title: "架构师",
      avatar: 'https://api.dicebear.com/7.x/avataaars/svg?seed=c',
    },
    {
      text: "我一直想进大厂，但系统设计是短板。详细报告和改进建议帮我理清了思路，现在我能把方案讲得更清楚，也顺利拿到了涨薪机会。",
      user: "静心",
      title: "后端开发工程师",
      avatar: 'https://api.dicebear.com/7.x/avataaars/svg?seed=d',
    },
    {
      text: "太值了！价格只有人工辅导的十分之一，但效果能达到九成，绝对是最划算的投入。",
      user: "杰奇",
      title: "后端开发工程师",
      avatar: 'https://api.dicebear.com/7.x/avataaars/svg?seed=e',
    },
    {
      text: "从对象关系映射优化到导出视图都覆盖到了，准备非常全面。",
      user: "墨然",
      title: "后端开发工程师",
      avatar: 'https://api.dicebear.com/7.x/avataaars/svg?seed=f',
    },
  ];

  return (
    <div className="min-h-screen bg-slate-50 font-sans overflow-hidden relative -my-8">
      {/* Decorative Background Elements */}
      <div className="fixed top-0 left-0 w-[800px] h-[800px] bg-blue-100/40 rounded-full blur-[120px] -translate-x-1/2 -translate-y-1/2 pointer-events-none z-0" />
      <div className="fixed bottom-0 right-0 w-[800px] h-[800px] bg-indigo-100/40 rounded-full blur-[120px] translate-x-1/3 translate-y-1/3 pointer-events-none z-0" />
      <div className="fixed top-1/2 left-1/2 w-[600px] h-[600px] bg-purple-50/40 rounded-full blur-[100px] -translate-x-1/2 -translate-y-1/2 pointer-events-none z-0" />

      {/* Hero Section */}
      <section className="relative pt-20 pb-32 px-4 sm:px-6 lg:px-8 z-10">
        <div className="max-w-7xl mx-auto text-center">
          <div className="inline-flex items-center gap-2 px-4 py-2 rounded-full bg-white border border-slate-200 shadow-sm mb-8 animate-fade-in-up">
            <Badge status="processing" color="blue" />
            <span className="text-sm font-medium text-slate-600">
              {"人工智能驱动面试备战平台 2.0 已上线"}
            </span>
          </div>

          <h1
            className="text-5xl md:text-7xl font-extrabold tracking-tight text-slate-900 mb-8 leading-tight animate-fade-in-up"
            style={{ animationDelay: '0.1s' }}
          >
            {"面试从未如此"} <br className="hidden md:block" />
            <span className="bg-clip-text text-transparent bg-gradient-to-r from-blue-600 via-indigo-600 to-purple-600">
              {"简单且自信"}
            </span>
          </h1>

          <p
            className="mt-4 max-w-2xl mx-auto text-xl text-slate-500 mb-10 animate-fade-in-up"
            style={{ animationDelay: '0.2s' }}
          >
            基于真实大厂题库，通过人工智能还原真实面试场景。<br />
            从简历分析到专项突破，全方位提升你的面试通过率。
          </p>

          <div
            className="flex flex-col items-center gap-6 animate-fade-in-up"
            style={{ animationDelay: '0.3s' }}
          >
            <div className="flex flex-col sm:flex-row gap-4 justify-center items-center">
              <Link href="/resume">
                <Button
                  type="primary"
                  size="large"
                  className="h-14 px-8 text-lg rounded-full bg-gradient-to-r from-blue-600 to-indigo-600 hover:scale-105 hover:shadow-xl transition-all border-0"
                  icon={<RocketOutlined />}
                >
                  {"立即免费试用"}
                </Button>
              </Link>
              <Link
                href="https://www.bilibili.com/video/BV1DavmBzEJu/?spm_id_from=333.1387.homepage.video_card.click&vd_source=94af06afd2820a951e81b9423ab621bb"
                target="_blank"
              >
                <Button
                  size="large"
                  className="h-14 px-8 text-lg rounded-full bg-white hover:bg-slate-50 border-slate-200 hover:border-blue-300 text-slate-700 hover:text-blue-600 hover:scale-105 transition-all shadow-sm hover:shadow-md"
                  icon={<PlayCircleFilled />}
                >
                  {"观看演示视频"}
                </Button>
              </Link>
            </div>

            <div className="flex flex-col sm:flex-row gap-4 justify-center items-center">
              <Link href="https://www.bilibili.com/video/BV1UCmEBQEW8/" target="_blank">
                <Button
                  size="large"
                  className="h-14 px-8 text-lg rounded-full bg-blue-50 hover:bg-blue-100 border-blue-100 hover:border-blue-200 text-blue-600 hover:text-blue-700 hover:scale-105 transition-all shadow-sm hover:shadow-md"
                  icon={<PlayCircleFilled />}
                >
                  {"视频教程"}
                </Button>
              </Link>
              <Link href="https://mp.weixin.qq.com/s/DlKoCQ7zUitCoiSoZzdVCQ" target="_blank">
                <Button
                  size="large"
                  className="h-14 px-8 text-lg rounded-full bg-green-50 hover:bg-green-100 border-green-100 hover:border-green-200 text-green-600 hover:text-green-700 hover:scale-105 transition-all shadow-sm hover:shadow-md"
                  icon={<FileTextOutlined />}
                >
                  {"课程介绍"}
                </Button>
              </Link>
            </div>
          </div>

          {/* Stats Bar */}
          <div
            className="mt-20 max-w-4xl mx-auto bg-white/60 backdrop-blur-xl rounded-2xl border border-white/50 shadow-xl shadow-slate-200/50 p-8 animate-fade-in-up"
            style={{ animationDelay: '0.4s' }}
          >
            <div className="grid grid-cols-2 md:grid-cols-4 gap-8">
              {[
                {
                  label: "用户数",
                  value: '12,000+',
                  color: 'text-blue-600',
                  icon: <UserOutlined />,
                },
                {
                  label: "模拟面试",
                  value: '50,000+',
                  color: 'text-indigo-600',
                  icon: <ExperimentOutlined />,
                },
                {
                  label: "题库问题",
                  value: '100,000+',
                  color: 'text-purple-600',
                  icon: <FileTextOutlined />,
                },
                {
                  label: "斩获录用",
                  value: '2,000+',
                  color: 'text-green-600',
                  icon: <TrophyOutlined />,
                },
              ].map((stat, idx) => (
                <div key={idx} className="flex flex-col items-center">
                  <div className={`text-2xl mb-2 ${stat.color}`}>{stat.icon}</div>
                  <div className="text-3xl font-bold text-slate-800">{stat.value}</div>
                  <div className="text-sm text-slate-500 mt-1">{stat.label}</div>
                </div>
              ))}
            </div>
          </div>
        </div>
      </section>

      {/* Features Section */}
      <section className="relative py-24 px-4 sm:px-6 lg:px-8 z-10 bg-white/50 backdrop-blur-sm">
        <div className="max-w-7xl mx-auto">
          <div className="text-center mb-16">
            <h2 className="text-3xl md:text-4xl font-bold text-slate-900 mb-4">
              {"全方位面试备战"}
            </h2>
            <p className="text-lg text-slate-500">
              {"无论你是校招新人还是社招老手，都有适合你的模式"}
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-8">
            {/* Card 1 */}
            <div className="group bg-white rounded-3xl p-8 border border-slate-100 shadow-lg hover:shadow-2xl hover:-translate-y-1 transition-all duration-300 relative overflow-hidden">
              <div className="absolute top-0 right-0 w-32 h-32 bg-blue-50 rounded-bl-full -mr-8 -mt-8 transition-transform group-hover:scale-110" />
              <div className="relative z-10">
                <div className="w-14 h-14 bg-blue-100 rounded-2xl flex items-center justify-center text-blue-600 text-2xl mb-6">
                  <BulbOutlined />
                </div>
                <h3 className="text-2xl font-bold text-slate-800 mb-4">{"综合模拟面试"}</h3>
                <p className="text-slate-500 mb-6 leading-relaxed h-20 overflow-hidden">
                  {"高度还原真实面试流程，包含自我介绍、项目深挖和技术考察。"}
                </p>
                <ul className="space-y-3 mb-8">
                  {["全流程模拟", "智能追问", "多维评估"].map((item, i) => (
                    <li key={i} className="flex items-center gap-2 text-slate-600 text-sm">
                      <CheckCircleFilled className="text-blue-500" /> {item}
                    </li>
                  ))}
                </ul>
                <Link href="/interview/social">
                  <Button
                    type="primary"
                    size="large"
                    className="w-full bg-blue-600 hover:bg-blue-700 border-0 shadow-blue-200"
                  >
                    {"开始综合面试"}
                  </Button>
                </Link>
              </div>
            </div>

            {/* Card 2 - Multi Agent (New) */}
            <div className="group bg-white rounded-3xl p-8 border border-slate-100 shadow-lg hover:shadow-2xl hover:-translate-y-1 transition-all duration-300 relative overflow-hidden">
              <div className="absolute top-0 right-0 w-32 h-32 bg-orange-50 rounded-bl-full -mr-8 -mt-8 transition-transform group-hover:scale-110" />
              <div className="relative z-10">
                <div className="flex justify-between items-start mb-6">
                  <div className="w-14 h-14 bg-orange-100 rounded-2xl flex items-center justify-center text-orange-600 text-2xl">
                    <TeamOutlined />
                  </div>
                  <Tag color="orange" className="border-0 px-2 py-0.5 text-xs font-bold">
                    新
                  </Tag>
                </div>
                <h3 className="text-2xl font-bold text-slate-800 mb-4">{"多智能体模拟面试"}</h3>
                <p className="text-slate-500 mb-6 leading-relaxed h-20 overflow-hidden">
                  {"由 3 位人工智能面试官（主面、技术、项目）组成专家小组，挑战大厂群面场景。"}
                </p>
                <ul className="space-y-3 mb-8">
                  {["智能体协同", "专家面板模式", "跨领域提问"].map((item, i) => (
                    <li key={i} className="flex items-center gap-2 text-slate-600 text-sm">
                      <CheckCircleFilled className="text-orange-500" /> {item}
                    </li>
                  ))}
                </ul>
                <Link href="/interview/multi">
                  <Button
                    type="primary"
                    size="large"
                    className="w-full bg-orange-600 hover:bg-orange-700 border-0 shadow-orange-200"
                  >
                    {"开启专家面板"}
                  </Button>
                </Link>
              </div>
            </div>

            {/* Card 3 */}
            <div className="group bg-gradient-to-b from-slate-800 to-slate-900 rounded-3xl p-8 shadow-xl hover:shadow-2xl hover:-translate-y-1 transition-all duration-300 relative overflow-hidden text-white">
              <div className="absolute top-0 right-0 w-64 h-64 bg-indigo-500/20 rounded-full blur-3xl -mr-16 -mt-16" />
              <div className="relative z-10">
                <div className="flex justify-between items-start mb-6">
                  <div className="w-14 h-14 bg-white/10 backdrop-blur-md rounded-2xl flex items-center justify-center text-white text-2xl">
                    <FileTextOutlined />
                  </div>
                  <Tag color="gold" className="border-0 px-3 py-1 text-xs font-bold">
                    热门
                  </Tag>
                </div>
                <h3 className="text-2xl font-bold text-white mb-4">{"简历押题预测"}</h3>
                <p className="text-slate-300 mb-6 leading-relaxed h-20 overflow-hidden">
                  {"上传简历后，人工智能将分析你的经历与技能，精准预测高频考点。"}
                </p>
                <ul className="space-y-3 mb-8">
                  {["深度简历分析", "项目细节深挖", "个性化题库"].map((item, i) => (
                    <li key={i} className="flex items-center gap-2 text-slate-600 text-sm">
                      <CheckCircleFilled className="text-indigo-400" /> {item}
                    </li>
                  ))}
                </ul>
                <Link href="/resume">
                  <Button
                    size="large"
                    className="w-full bg-white text-slate-900 hover:bg-slate-100 border-0 font-bold"
                  >
                    {"立即上传简历"}
                  </Button>
                </Link>
              </div>
            </div>

            {/* Card 4 */}
            <div className="group bg-white rounded-3xl p-8 border border-slate-100 shadow-lg hover:shadow-2xl hover:-translate-y-1 transition-all duration-300 relative overflow-hidden">
              <div className="absolute top-0 right-0 w-32 h-32 bg-purple-50 rounded-bl-full -mr-8 -mt-8 transition-transform group-hover:scale-110" />
              <div className="relative z-10">
                <div className="w-14 h-14 bg-purple-100 rounded-2xl flex items-center justify-center text-purple-600 text-2xl mb-6">
                  <CodeOutlined />
                </div>
                <h3 className="text-2xl font-bold text-slate-800 mb-4">{"专项突破训练"}</h3>
                <p className="text-slate-500 mb-6 leading-relaxed h-20 overflow-hidden">
                  {"针对特定技术栈（高并发、JVM、MySQL 调优等）集中训练，快速补齐短板。"}
                </p>
                <ul className="space-y-3 mb-8">
                  {["技术栈强化", "架构设计", "算法与数据结构"].map((item, i) => (
                    <li key={i} className="flex items-center gap-2 text-slate-600 text-sm">
                      <CheckCircleFilled className="text-purple-500" /> {item}
                    </li>
                  ))}
                </ul>
                <Link href="/interview/special">
                  <Button
                    type="primary"
                    size="large"
                    className="w-full bg-purple-600 hover:bg-purple-700 border-0 shadow-purple-200"
                  >
                    {"选择专项方向"}
                  </Button>
                </Link>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Why Choose Us - Grid */}
      <section className="relative py-24 px-4 sm:px-6 lg:px-8 z-10">
        <div className="max-w-7xl mx-auto">
          <div className="text-center mb-16">
            <h2 className="text-3xl md:text-4xl font-bold text-slate-900 mb-4">
              {"为什么选择面试吧？"}
            </h2>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
            {[
              {
                icon: <CompassOutlined />,
                title: "大厂真题题库",
                desc: "基于头部企业真实题目训练，拒绝过时资料",
                color: 'text-blue-500',
                bg: 'bg-blue-50',
              },
              {
                icon: <FlagOutlined />,
                title: "个性化定制",
                desc: "根据你的简历与能力动态调整难度",
                color: 'text-indigo-500',
                bg: 'bg-indigo-50',
              },
              {
                icon: <EyeOutlined />,
                title: "深度复盘",
                desc: "每场面试生成详细评估报告与改进建议",
                color: 'text-purple-500',
                bg: 'bg-purple-50',
              },
              {
                icon: <ThunderboltOutlined />,
                title: "即时反馈",
                desc: "无需等待，随时随地开始面试并获得实时反馈",
                color: 'text-yellow-500',
                bg: 'bg-yellow-50',
              },
              {
                icon: <SmileOutlined />,
                title: "高性价比",
                desc: "仅为传统辅导价格的 1/10，提供 24 小时服务",
                color: 'text-green-500',
                bg: 'bg-green-50',
              },
              {
                icon: <SwitcherOutlined />,
                title: "难度可控",
                desc: "从入门到专家，自由切换",
                color: 'text-pink-500',
                bg: 'bg-pink-50',
              },
              {
                icon: <SendOutlined />,
                title: "实战高压",
                desc: "模拟高压面试环境，克服紧张情绪",
                color: 'text-cyan-500',
                bg: 'bg-cyan-50',
              },
              {
                icon: <TeamOutlined />,
                title: "角色扮演",
                desc: "模拟多种面试风格，从容应对各种场景",
                color: 'text-orange-500',
                bg: 'bg-orange-50',
              },
            ].map((item, idx) => (
              <div
                key={idx}
                className="bg-white rounded-2xl p-6 border border-slate-100 shadow-sm hover:shadow-md transition-all flex flex-col items-start"
              >
                <div
                  className={`w-12 h-12 ${item.bg} rounded-xl flex items-center justify-center text-xl ${item.color} mb-4`}
                >
                  {item.icon}
                </div>
                <h4 className="text-lg font-bold text-slate-800 mb-2">{item.title}</h4>
                <p className="text-slate-500 text-sm leading-relaxed">{item.desc}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Testimonials - Modern Horizontal */}
      <section className="relative py-24 px-4 sm:px-6 lg:px-8 z-10 bg-gradient-to-b from-slate-50 to-white">
        <div className="max-w-7xl mx-auto">
          <div className="text-center mb-16">
            <h2 className="text-3xl md:text-4xl font-bold text-slate-900 mb-4">{"用户反馈"}</h2>
            <p className="text-lg text-slate-500">{"看看他们如何通过我们拿到理想录用通知"}</p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-8">
            {testimonials.map((t, i) => (
              <div
                key={i}
                className="bg-white rounded-2xl p-8 shadow-sm border border-slate-100 hover:shadow-lg transition-all"
              >
                <div className="flex items-center gap-1 text-yellow-400 mb-6">
                  {[1, 2, 3, 4, 5].map((s) => (
                    <FireOutlined key={s} />
                  ))}
                </div>
                <p className="text-slate-600 mb-6 leading-relaxed italic">"{t.text}"</p>
                <div className="flex items-center gap-4">
                  <Avatar src={t.avatar} size={48} className="border-2 border-white shadow-sm" />
                  <div>
                    <div className="font-bold text-slate-800">{t.user}</div>
                    <div className="text-xs text-slate-500 font-medium uppercase tracking-wide">
                      {t.title}
                    </div>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* FAQ Section */}
      <section className="relative py-24 px-4 sm:px-6 lg:px-8 z-10">
        <div className="max-w-3xl mx-auto">
          <div className="text-center mb-12">
            <h2 className="text-3xl font-bold text-slate-900">{"常见问题"}</h2>
          </div>
          <Collapse
            ghost
            expandIconPosition="end"
            items={[
              {
                key: '1',
                label: "为什么使用面试吧，而不是通用问答模型？",
                children: (
                  <p className="text-slate-500 pb-4">
                    {"我们围绕面试场景深度定制，提问标准、追问逻辑和评估体系都更贴近真实招聘要求。"}
                  </p>
                ),
              },
              {
                key: '2',
                label: "它主要解决什么问题？",
                children: (
                  <p className="text-slate-500 pb-4">
                    {"它能帮你在真实面试前发现短板，并提供结构化评估和改进建议。"}
                  </p>
                ),
              },
              {
                key: '3',
                label: "适合哪些人使用？",
                children: (
                  <p className="text-slate-500 pb-4">
                    {"从校招到社招，从技术岗位求职到职业转型人群都适用。"}
                  </p>
                ),
              },
            ]}
          />
        </div>
      </section>
    </div>
  );
}
