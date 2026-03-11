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
      text: '非科班出身，自学一年多总感觉基础不扎实。面试吧的简历押题功能太神了，针对我的项目经历生成的题目命中率很高，90%都在实际面试中遇到过。特别是Spring框架深度问题、IOC到AOP，从题目的逻辑出发让我能由浅入深串联知识体系，这种从值到到深度的感觉真的很棒。',
      user: '35岁重启人生',
      title: '社招Java开发',
      avatar: 'https://api.dicebear.com/7.x/avataaars/svg?seed=a',
    },
    {
      text: '996的工作节奏根本没时间找人mock interview。面试吧24小时随时随地陪练，效率极高。每天晚上坚持练习30分钟，岗位面试官会问到系统架构设计、链路梳理思维，面试吧都能覆盖到位。最后还给了提升建议，面完真实的大厂，和正式面试时居然遇到70%相似问题！',
      user: 'smartbob',
      title: '在职提升-Go开发',
      avatar: 'https://api.dicebear.com/7.x/avataaars/svg?seed=b',
    },
    {
      text: '用了面试吧模拟面试，追问技术细节的能力大幅提升！模拟面试会根据我的回答深入追问，连Redis AOF、优化这种偏细节都能展开，最后还给了提升建议。面完真实的大厂，和正式面试时居然遇到70%相似问题！',
      user: 'Lex',
      title: '架构师',
      avatar: 'https://api.dicebear.com/7.x/avataaars/svg?seed=c',
    },
    {
      text: '工作三年想冲击大厂，但系统设计这块一直是短板。面试吧详细评估报告+改进建议与题目分析，高效锤炼，不断改进，亲身实战。现在在面试中能条理清晰地讲解方案，拿到满意的薪资。',
      user: '静以修身',
      title: '后端开发',
      avatar: 'https://api.dicebear.com/7.x/avataaars/svg?seed=d',
    },
    {
      text: '很棒！价格可能就是人工服务的1/10，效果至少是8、9成，性价比超高！',
      user: 'jackey',
      title: '后端开发',
      avatar: 'https://api.dicebear.com/7.x/avataaars/svg?seed=e',
    },
    {
      text: 'Django ORM优化和导出视图这些都能有涉及，面试准备很全面。',
      user: '默然',
      title: 'Python开发',
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
              AI 驱动的面试备战平台 2.0 全新上线
            </span>
          </div>

          <h1
            className="text-5xl md:text-7xl font-extrabold tracking-tight text-slate-900 mb-8 leading-tight animate-fade-in-up"
            style={{ animationDelay: '0.1s' }}
          >
            面试从未如此 <br className="hidden md:block" />
            <span className="bg-clip-text text-transparent bg-gradient-to-r from-blue-600 via-indigo-600 to-purple-600">
              简单且自信
            </span>
          </h1>

          <p
            className="mt-4 max-w-2xl mx-auto text-xl text-slate-500 mb-10 animate-fade-in-up"
            style={{ animationDelay: '0.2s' }}
          >
            基于真实大厂面试题库，通过 AI 模拟真实面试场景。
            <br />
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
                  立即开始免费体验
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
                  查看演示视频
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
                  视频教程
                </Button>
              </Link>
              <Link href="https://mp.weixin.qq.com/s/DlKoCQ7zUitCoiSoZzdVCQ" target="_blank">
                <Button
                  size="large"
                  className="h-14 px-8 text-lg rounded-full bg-green-50 hover:bg-green-100 border-green-100 hover:border-green-200 text-green-600 hover:text-green-700 hover:scale-105 transition-all shadow-sm hover:shadow-md"
                  icon={<FileTextOutlined />}
                >
                  课程介绍
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
                  label: '注册用户',
                  value: '12,000+',
                  color: 'text-blue-600',
                  icon: <UserOutlined />,
                },
                {
                  label: '模拟面试',
                  value: '50,000+',
                  color: 'text-indigo-600',
                  icon: <ExperimentOutlined />,
                },
                {
                  label: '题库收录',
                  value: '100,000+',
                  color: 'text-purple-600',
                  icon: <FileTextOutlined />,
                },
                {
                  label: 'Offer斩获',
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
              全方位的面试备战方案
            </h2>
            <p className="text-lg text-slate-500">
              无论你是校招萌新还是社招大佬，这里都有适合你的练习模式
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
                <h3 className="text-2xl font-bold text-slate-800 mb-4">综合模拟面试</h3>
                <p className="text-slate-500 mb-6 leading-relaxed h-20 overflow-hidden">
                  高度还原真实面试场景，包含自我介绍、项目深挖、技术考察等全流程。
                </p>
                <ul className="space-y-3 mb-8">
                  {['全真流程模拟', '智能追问机制', '多维度能力评估'].map((item, i) => (
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
                    开始综合模拟
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
                    NEW
                  </Tag>
                </div>
                <h3 className="text-2xl font-bold text-slate-800 mb-4">多人模拟面试</h3>
                <p className="text-slate-500 mb-6 leading-relaxed h-20 overflow-hidden">
                  由 3 位不同背景的 AI 专家（主面、技术、项目）组成的面试组，挑战大厂群面。
                </p>
                <ul className="space-y-3 mb-8">
                  {['Eino 智能体协作', '面试官小组模式', '跨领域联合提问'].map((item, i) => (
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
                    开启专家群面
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
                    HOT
                  </Tag>
                </div>
                <h3 className="text-2xl font-bold text-white mb-4">简历押题</h3>
                <p className="text-slate-300 mb-6 leading-relaxed h-20 overflow-hidden">
                  上传你的简历，AI 将深度分析你的项目经历与技能栈，精准预测面试高频考点。
                </p>
                <ul className="space-y-3 mb-8">
                  {['简历深度解析', '项目细节拷问', '定制化题库生成'].map((item, i) => (
                    <li key={i} className="flex items-center gap-2 text-slate-200 text-sm">
                      <CheckCircleFilled className="text-indigo-400" /> {item}
                    </li>
                  ))}
                </ul>
                <Link href="/resume">
                  <Button
                    size="large"
                    className="w-full bg-white text-slate-900 hover:bg-slate-100 border-0 font-bold"
                  >
                    上传简历押题
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
                <h3 className="text-2xl font-bold text-slate-800 mb-4">专项技术突破</h3>
                <p className="text-slate-500 mb-6 leading-relaxed h-20 overflow-hidden">
                  针对特定技术栈（高并发、JVM、MySQL调优等）进行集中训练，快速补齐短板。
                </p>
                <ul className="space-y-3 mb-8">
                  {['技术栈专项训练', '架构设计专题', '算法与数据结构'].map((item, i) => (
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
                    选择专项训练
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
              为什么选择面试吧？
            </h2>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
            {[
              {
                icon: <CompassOutlined />,
                title: '大厂真题库',
                desc: '基于一线大厂历年真题训练，拒绝过时八股文',
                color: 'text-blue-500',
                bg: 'bg-blue-50',
              },
              {
                icon: <FlagOutlined />,
                title: '千人千面',
                desc: '根据你的简历和能力动态调整题目难度',
                color: 'text-indigo-500',
                bg: 'bg-indigo-50',
              },
              {
                icon: <EyeOutlined />,
                title: '深度复盘',
                desc: '每次面试都有详细的评估报告与改进建议',
                color: 'text-purple-500',
                bg: 'bg-purple-50',
              },
              {
                icon: <ThunderboltOutlined />,
                title: '极速反馈',
                desc: '无需等待，随时随地开启面试，实时反馈',
                color: 'text-yellow-500',
                bg: 'bg-yellow-50',
              },
              {
                icon: <SmileOutlined />,
                title: '超高性价比',
                desc: '仅需传统私教 1/10 的价格享受 24h 服务',
                color: 'text-green-500',
                bg: 'bg-green-50',
              },
              {
                icon: <SwitcherOutlined />,
                title: '难度可控',
                desc: '从入门到专家级，难度随心切换',
                color: 'text-pink-500',
                bg: 'bg-pink-50',
              },
              {
                icon: <SendOutlined />,
                title: '实战演练',
                desc: '高压环境模拟，克服面试紧张感',
                color: 'text-cyan-500',
                bg: 'bg-cyan-50',
              },
              {
                icon: <TeamOutlined />,
                title: '角色扮演',
                desc: '模拟不同风格面试官，从容应对各种情况',
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
            <h2 className="text-3xl md:text-4xl font-bold text-slate-900 mb-4">用户真实反馈</h2>
            <p className="text-lg text-slate-500">看看他们如何通过面试吧拿到心仪 Offer</p>
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
            <h2 className="text-3xl font-bold text-slate-900">常见问题</h2>
          </div>
          <Collapse
            ghost
            expandIconPosition="end"
            items={[
              {
                key: '1',
                label: '为什么要用面试吧，而不是豆包、ChatGPT这些通用AI？',
                children: (
                  <p className="text-slate-500 pb-4">
                    面试吧针对求职面试场景深度定制，真实面试标准、追问逻辑与评估体系都更贴近用人单位的要求。
                  </p>
                ),
              },
              {
                key: '2',
                label: '面试吧到底解决了什么问题',
                children: (
                  <p className="text-slate-500 pb-4">
                    帮助你在真实面试来临前发现薄弱点并针对性训练，输出结构化评估与改进建议。
                  </p>
                ),
              },
              {
                key: '3',
                label: '面试吧适合什么样的人使用？',
                children: (
                  <p className="text-slate-500 pb-4">
                    从校招到社招，从转岗到晋升，皆可使用；支持多岗位面试模拟。
                  </p>
                ),
              },
              {
                key: '4',
                label: '收费标准是什么，性价比如何？',
                children: (
                  <p className="text-slate-500 pb-4">
                    单次体验低成本，会员价格更划算；与线下私教相比成本约为1/10。
                  </p>
                ),
              },
            ]}
            className="bg-white rounded-2xl shadow-sm border border-slate-200"
          />
        </div>
      </section>

      {/* Bottom CTA */}
      <section className="relative py-20 px-4 sm:px-6 lg:px-8 z-10">
        <div className="max-w-5xl mx-auto bg-gradient-to-r from-blue-600 to-indigo-700 rounded-3xl p-12 text-center shadow-2xl relative overflow-hidden">
          <div className="absolute top-0 left-0 w-full h-full bg-[url('https://grainy-gradients.vercel.app/noise.svg')] opacity-20"></div>
          <div className="relative z-10">
            <h2 className="text-3xl md:text-4xl font-bold text-white mb-6">
              准备好开始你的面试之旅了吗？
            </h2>
            <p className="text-blue-100 text-lg mb-10 max-w-2xl mx-auto">
              立即加入数万名求职者的行列，用 AI 武装自己，从容应对每一次挑战。
            </p>
            <Link href="/resume">
              <Button
                size="large"
                className="h-14 px-10 text-lg rounded-full bg-white text-blue-700 hover:bg-blue-50 border-0 font-bold shadow-lg hover:scale-105 transition-all"
              >
                免费开始使用
              </Button>
            </Link>
          </div>
        </div>
      </section>
    </div>
  );
}
