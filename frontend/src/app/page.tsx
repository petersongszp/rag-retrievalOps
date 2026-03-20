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
      text: "Coming from a non-CS background, I felt my foundation was weak. The resume prediction is incredible; it precisely hit 90% of the questions I faced in real interviews. It helped me connect knowledge from Spring IHC to AOP logically.",
      user: "35+ Restarting",
      title: "Social Java Dev",
      avatar: 'https://api.dicebear.com/7.x/avataaars/svg?seed=a',
    },
    {
      text: "With a 996 schedule, I had no time for mock interviews. This platform is efficient and available 24/7. My interviewer asked about system design and logic, and the platform covered them all. I faced 70% similar questions in the final interview!",
      user: "smartbob",
      title: "Go Developer",
      avatar: 'https://api.dicebear.com/7.x/avataaars/svg?seed=b',
    },
    {
      text: "The follow-up questions drastically improved my ability to explain technical details. It dug into Redis AOF and optimization, exactly what I needed for senior roles.",
      user: "Lex",
      title: "Architect",
      avatar: 'https://api.dicebear.com/7.x/avataaars/svg?seed=c',
    },
    {
      text: "I wanted to join a top company but system design was my weak point. The detailed reports and suggestions helped me clear my thoughts. I can now explain solutions clearly and got a great salary bump.",
      user: "Quiet Mind",
      title: "Backend Developer",
      avatar: 'https://api.dicebear.com/7.x/avataaars/svg?seed=d',
    },
    {
      text: "Great! 1/10 price of manual services with 90% effectiveness. Best investment!",
      user: "jackey",
      title: "Backend Developer",
      avatar: 'https://api.dicebear.com/7.x/avataaars/svg?seed=e',
    },
    {
      text: "Covered everything from Django ORM optimization to export views. Very comprehensive preparation.",
      user: "Mo Ran",
      title: "Python Developer",
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
              {"AI-Driven Interview Preparation Platform 2.0 Launched"}
            </span>
          </div>

          <h1
            className="text-5xl md:text-7xl font-extrabold tracking-tight text-slate-900 mb-8 leading-tight animate-fade-in-up"
            style={{ animationDelay: '0.1s' }}
          >
            {"Interview Never Been So"} <br className="hidden md:block" />
            <span className="bg-clip-text text-transparent bg-gradient-to-r from-blue-600 via-indigo-600 to-purple-600">
              {"Simple and Confident"}
            </span>
          </h1>

          <p
            className="mt-4 max-w-2xl mx-auto text-xl text-slate-500 mb-10 animate-fade-in-up"
            style={{ animationDelay: '0.2s' }}
          >
            Based on real big factory question banks, simulate real interview scenarios via AI.<br />
            From resume analysis to specialized breakthroughs, improve your interview pass rate in all directions.
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
                  {"Start Free Trial Now"}
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
                  {"Watch Demo Video"}
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
                  {"Video Tutorial"}
                </Button>
              </Link>
              <Link href="https://mp.weixin.qq.com/s/DlKoCQ7zUitCoiSoZzdVCQ" target="_blank">
                <Button
                  size="large"
                  className="h-14 px-8 text-lg rounded-full bg-green-50 hover:bg-green-100 border-green-100 hover:border-green-200 text-green-600 hover:text-green-700 hover:scale-105 transition-all shadow-sm hover:shadow-md"
                  icon={<FileTextOutlined />}
                >
                  {"Course Introduction"}
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
                  label: "Users",
                  value: '12,000+',
                  color: 'text-blue-600',
                  icon: <UserOutlined />,
                },
                {
                  label: "Mock Interviews",
                  value: '50,000+',
                  color: 'text-indigo-600',
                  icon: <ExperimentOutlined />,
                },
                {
                  label: "Questions",
                  value: '100,000+',
                  color: 'text-purple-600',
                  icon: <FileTextOutlined />,
                },
                {
                  label: "Offers Won",
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
              {"All-round Interview Preparation"}
            </h2>
            <p className="text-lg text-slate-500">
              {"Whether you are a campus novice or a social veteran, we have the right mode for you"}
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
                <h3 className="text-2xl font-bold text-slate-800 mb-4">{"Comprehensive Mock Interview"}</h3>
                <p className="text-slate-500 mb-6 leading-relaxed h-20 overflow-hidden">
                  {"Highly restore real interview scenarios, including self-intro, project digging, and technical tests."}
                </p>
                <ul className="space-y-3 mb-8">
                  {["Full process simulation", "Intelligent follow-up", "Multi-dim evaluation"].map((item, i) => (
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
                    {"Start Comprehensive"}
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
                <h3 className="text-2xl font-bold text-slate-800 mb-4">{"Multi-Agent Mock Interview"}</h3>
                <p className="text-slate-500 mb-6 leading-relaxed h-20 overflow-hidden">
                  {"An interview panel of 3 AI experts (Main, Tech, Project) to challenge big factory group interviews."}
                </p>
                <ul className="space-y-3 mb-8">
                  {["Eino Agent collab", "Panel mode", "Cross-domain questions"].map((item, i) => (
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
                    {"Start Expert Panel"}
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
                <h3 className="text-2xl font-bold text-white mb-4">{"Resume Prediction"}</h3>
                <p className="text-slate-300 mb-6 leading-relaxed h-20 overflow-hidden">
                  {"Upload your resume, AI will analyze your experience and skills to precisely predict high-frequency points."}
                </p>
                <ul className="space-y-3 mb-8">
                  {["Deep resume analysis", "Project details", "Customized bank"].map((item, i) => (
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
                    {"Upload Resume Now"}
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
                <h3 className="text-2xl font-bold text-slate-800 mb-4">{"Specialized Breakthrough"}</h3>
                <p className="text-slate-500 mb-6 leading-relaxed h-20 overflow-hidden">
                  {"Concentrated training for specific stacks (High concurrency, JVM, MySQL tuning, etc.) to quickly fix weaknesses."}
                </p>
                <ul className="space-y-3 mb-8">
                  {["Stack training", "Architecture design", "Algo & Data structure"].map((item, i) => (
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
                    {"Choose Specialized"}
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
              {"Why Choose Interview Master?"}
            </h2>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
            {[
              {
                icon: <CompassOutlined />,
                title: "Big Factory Bank",
                desc: "Trained on real questions from top companies, refuse outdated materials",
                color: 'text-blue-500',
                bg: 'bg-blue-50',
              },
              {
                icon: <FlagOutlined />,
                title: "Personalized",
                desc: "Dynamically adjust difficulty based on your resume and ability",
                color: 'text-indigo-500',
                bg: 'bg-indigo-50',
              },
              {
                icon: <EyeOutlined />,
                title: "Deep Review",
                desc: "Detailed evaluation report and improvement suggestions for each interview",
                color: 'text-purple-500',
                bg: 'bg-purple-50',
              },
              {
                icon: <ThunderboltOutlined />,
                title: "Instant Feedback",
                desc: "No waiting, start interview anytime anywhere with real-time feedback",
                color: 'text-yellow-500',
                bg: 'bg-yellow-50',
              },
              {
                icon: <SmileOutlined />,
                title: "High Cost-Performance",
                desc: "Only 1/10 price of traditional tutors for 24h service",
                color: 'text-green-500',
                bg: 'bg-green-50',
              },
              {
                icon: <SwitcherOutlined />,
                title: "Controllable Difficulty",
                desc: "From entry to expert level, switch as you wish",
                color: 'text-pink-500',
                bg: 'bg-pink-50',
              },
              {
                icon: <SendOutlined />,
                title: "Real Combat",
                desc: "High-pressure environment simulation to overcome nervousness",
                color: 'text-cyan-500',
                bg: 'bg-cyan-50',
              },
              {
                icon: <TeamOutlined />,
                title: "Role Play",
                desc: "Simulate various interview styles to handle any situation",
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
            <h2 className="text-3xl md:text-4xl font-bold text-slate-900 mb-4">{"User Feedback"}</h2>
            <p className="text-lg text-slate-500">{"See how they got their dream offers through us"}</p>
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
            <h2 className="text-3xl font-bold text-slate-900">{"FAQ"}</h2>
          </div>
          <Collapse
            ghost
            expandIconPosition="end"
            items={[
              {
                key: '1',
                label: "Why use Interview Master instead of general AI like ChatGPT?",
                children: (
                  <p className="text-slate-500 pb-4">
                    {"We are deeply customized for interview scenarios. Our standard, follow-up logic, and evaluation system are much closer to real employer requirements."}
                  </p>
                ),
              },
              {
                key: '2',
                label: "What problem does it solve?",
                children: (
                  <p className="text-slate-500 pb-4">
                    {"It helps you find weaknesses before real interviews and provides structured evaluation and suggestions."}
                  </p>
                ),
              },
              {
                key: '3',
                label: "Who is it for?",
                children: (
                  <p className="text-slate-500 pb-4">
                    {"From campus recruitment to experienced professionals, for anyone seeking technical roles or career changes."}
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
