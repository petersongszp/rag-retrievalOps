'use client';

import { useEffect, useState } from 'react';
import { Card as AntCard, Row, Col, Tag, Button, Avatar, Spin, message, Divider } from 'antd';
import { useParams } from 'next/navigation';
import apiClient from '@/services/api/client';
import { useAuth } from '@/hooks/useAuth';
import { USER_API, INTERVIEW_API } from '@/config/api';
import {
  TrophyOutlined,
  ClockCircleOutlined,
  CalendarOutlined,
  EnvironmentOutlined,
  UserOutlined,
  BarChartOutlined,
  DownloadOutlined,
  RobotOutlined,
  CheckCircleOutlined,
  BulbOutlined,
} from '@ant-design/icons';

const difficultyMap: Record<string, string> = {
  easy: '简单',
  medium: '中等',
  hard: '困难',
  '\u7b80\u5355': '简单',
  '\u4e2d\u7b49': '中等',
  '\u56f0\u96be': '困难',
};

const interviewTypeMap: Record<string, string> = {
  technical: '技术面',
  hr: 'HR',
  behavioral: '行为面',
  '\u7efc\u5408': '综合面试',
  '\u6280\u672f\u9762': '技术面',
  '\u884c\u4e3a\u9762': '行为面',
  'hr\u9762': 'HR',
};

const dimensionNameMap: Record<string, string> = {
  '\u6c9f\u901a\u8868\u8fbe': '沟通表达',
  '\u903b\u8f91\u601d\u7ef4': '逻辑思维',
  '\u6280\u672f\u6df1\u5ea6': '技术深度',
  '\u4e1a\u52a1\u7406\u89e3': '业务理解',
  '\u5b66\u4e60\u80fd\u529b': '学习能力',
  '\u95ee\u9898\u89e3\u51b3': '问题解决',
  '\u56e2\u961f\u534f\u4f5c': '团队协作',
};

const toEnglishLabel = (value: string | undefined, mapping: Record<string, string>) => {
  if (!value) return '未知';
  return mapping[value] || value;
};

const formatDuration = (totalSeconds: number | undefined) => {
  if (!totalSeconds && totalSeconds !== 0) return '未知';
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}m ${seconds}s`;
};

const shortDimensionName = (name: string) => {
  const translated = dimensionNameMap[name] || name;
  return translated.length > 18 ? `${translated.slice(0, 15)}...` : translated;
};

function ExpandableMobileText({
  content,
  textClassName,
  lines = 3,
  threshold = 120,
}: {
  content: unknown;
  textClassName?: string;
  lines?: number;
  threshold?: number;
}) {
  const [expanded, setExpanded] = useState(false);
  const value = content == null ? '' : String(content);

  if (!value) return null;

  const shouldClamp = value.length > threshold;
  const clampClassByLines: Record<number, string> = {
    2: '[-webkit-line-clamp:2]',
    3: '[-webkit-line-clamp:3]',
    4: '[-webkit-line-clamp:4]',
    5: '[-webkit-line-clamp:5]',
  };
  const clampClass = clampClassByLines[lines] || clampClassByLines[3];

  return (
    <div className="min-w-0">
      <div
        className={[
          textClassName || '',
          'break-words [overflow-wrap:anywhere]',
          !expanded && shouldClamp
            ? `overflow-hidden [display:-webkit-box] [-webkit-box-orient:vertical] ${clampClass} sm:overflow-visible sm:[display:block] sm:[-webkit-line-clamp:unset]`
            : '',
        ]
          .filter(Boolean)
          .join(' ')}
      >
        {value}
      </div>
      {shouldClamp && (
        <button
          type="button"
          onClick={() => setExpanded((prev) => !prev)}
          className="mt-2 text-xs font-medium text-blue-600 hover:text-blue-500 sm:hidden"
        >
          {expanded ? '收起' : '展开'}
        </button>
      )}
    </div>
  );
}

function RadarChart({
  items,
  size = 520,
}: {
  items: { dimension_name: string; score: number }[];
  size?: number;
}) {
  const radius = size * 0.35; // Use 35% of size for radius to leave room for labels
  const cx = size / 2;
  const cy = size / 2;
  const points = items.map((it, i) => {
    const angle = (2 * Math.PI * i) / items.length - Math.PI / 2;
    const r = (Math.max(0, Math.min(100, Number(it.score))) / 100) * radius;
    return [cx + r * Math.cos(angle), cy + r * Math.sin(angle)];
  });
  const axis = items.map((_, i) => {
    const angle = (2 * Math.PI * i) / items.length - Math.PI / 2;
    return [cx + radius * Math.cos(angle), cy + radius * Math.sin(angle)];
  });
  const poly = points.map((p) => p.join(',')).join(' ');

  return (
    <div
      style={{ width: '100%', maxWidth: size, height: size }}
      className="mx-auto relative min-w-0"
    >
      <svg
        width="100%"
        height="100%"
        viewBox={`0 0 ${size} ${size}`}
        style={{ overflow: 'visible' }}
      >
        <defs>
          <radialGradient id="radarGradient" cx="50%" cy="50%" r="50%" fx="50%" fy="50%">
            <stop offset="0%" stopColor="rgba(82,196,26,0.4)" />
            <stop offset="100%" stopColor="rgba(82,196,26,0.1)" />
          </radialGradient>
        </defs>
        {/* Background circles */}
        {[1, 0.8, 0.6, 0.4, 0.2].map((scale, i) => (
          <circle
            key={i}
            cx={cx}
            cy={cy}
            r={radius * scale}
            fill={i === 0 ? '#f6ffed' : 'none'}
            stroke="#d9f7be"
            strokeDasharray={i === 0 ? 'none' : '4 4'}
          />
        ))}

        {/* Axis lines */}
        {axis.map((p, i) => (
          <line key={i} x1={cx} y1={cy} x2={p[0]} y2={p[1]} stroke="#e8e8e8" />
        ))}

        {/* Data polygon */}
        <polygon points={poly} fill="url(#radarGradient)" stroke="#52c41a" strokeWidth={2} />

        {/* Data points */}
        {points.map((p, i) => (
          <circle key={i} cx={p[0]} cy={p[1]} r={4} fill="#fff" stroke="#52c41a" strokeWidth={2} />
        ))}

        {/* Labels */}
        {axis.map((p, i) => {
          // Calculate offset based on angle to push labels away from center
          const angle = (2 * Math.PI * i) / items.length - Math.PI / 2;
          const labelDist = 14; // Distance from the end of the axis
          const lx = p[0] + labelDist * Math.cos(angle);
          const ly = p[1] + labelDist * Math.sin(angle);

          // Determine text anchor based on x position relative to center
          let textAnchor = 'middle';
          if (Math.abs(lx - cx) > 10) {
            textAnchor = lx > cx ? 'start' : 'end';
          }

          // Determine dominant baseline based on y position
          let dominantBaseline = 'middle';
          if (Math.abs(ly - cy) > 10) {
            dominantBaseline = ly > cy ? 'hanging' : 'baseline';
          }

          return (
            <text
              key={i}
              x={lx}
              y={ly}
              textAnchor={textAnchor}
              dominantBaseline={dominantBaseline}
              fontSize={11}
              fontWeight={600}
              fill="#64748b"
            >
              {shortDimensionName(items[i].dimension_name)}
            </text>
          );
        })}
      </svg>
    </div>
  );
}

function ScoreGauge({ score }: { score: number }) {
  const size = 260;
  const cx = size / 2;
  const cy = size / 2;
  const r = 100;
  const start = Math.PI;
  const angle = (Math.max(0, Math.min(100, score)) / 100) * Math.PI;
  const arcPath = (ang: number, color: string) => {
    const sx = cx + r * Math.cos(start);
    const sy = cy + r * Math.sin(start);
    const ex = cx + r * Math.cos(start + ang);
    const ey = cy + r * Math.sin(start + ang);
    return (
      <path
        d={`M ${sx} ${sy} A ${r} ${r} 0 0 1 ${ex} ${ey}`}
        stroke={color}
        strokeWidth={14}
        fill="none"
        strokeLinecap="round"
      />
    );
  };

  let color = '#52c41a'; // Green for good
  if (score < 60)
    color = '#ff4d4f'; // Red for bad
  else if (score < 80) color = '#faad14'; // Yellow for average

  return (
    <div style={{ width: size, height: size / 1.4 }} className="mx-auto relative">
      <svg width={size} height={size / 1.4}>
        <path
          d={`M ${cx - r} ${cy} A ${r} ${r} 0 0 1 ${cx + r} ${cy}`}
          stroke="#f0f0f0"
          strokeWidth={14}
          fill="none"
          strokeLinecap="round"
        />
        {arcPath(angle, color)}
        <text x={cx} y={cy - 15} textAnchor="middle" fontSize={14} fill="#8c8c8c">
          面试得分
        </text>
        <text x={cx} y={cy + 30} textAnchor="middle" fontSize={48} fontWeight={700} fill={color}>
          {score}
        </text>
      </svg>
    </div>
  );
}

export default function InterviewResultDetailPage() {
  const params = useParams() as any;
  const id = Number(params?.id || 0);
  const { user, login } = useAuth();

  const [loading, setLoading] = useState(true);
  const [interviewInfo, setInterviewInfo] = useState<any>(null);
  const [evaluation, setEvaluation] = useState<any>(null);
  const [answerRecords, setAnswerRecords] = useState<any[]>([]);

  const fetchData = async () => {
    if (!id) return;
    setLoading(true);
    try {
      // 0. Fetch User Profile if missing
      if (!user) {
        try {
          const userRes: any = await apiClient.get(USER_API.GET_PROFILE);
          if (userRes && userRes.username) {
            login({
              id: String(userRes.id),
              name: userRes.username,
              email: userRes.email,
              avatar: userRes.avatar,
            });
          }
        } catch (e) {
          console.error('获取用户信息失败', e);
        }
      }

      // 1. Fetch Interview Info (from list)
      const listRes: any = await apiClient.get(INTERVIEW_API.GET_RECORDS, {
        params: { page: 1, page_size: 1000 },
      });
      const listData = listRes?.records || [];
      const info = listData.find((item: any) => item.id === id);
      setInterviewInfo(info);

      // 2. Fetch Evaluation Report
      const evalRes: any = await apiClient.get(INTERVIEW_API.GET_EVALUATION, {
        params: { report_id: id },
      });
      setEvaluation(evalRes);

      // 3. Fetch Answer Records
      const recordRes: any = await apiClient.get(INTERVIEW_API.GET_ANSWER_RECORD, {
        params: { report_id: id },
      });
      if (recordRes && recordRes.records) {
        setAnswerRecords(recordRes.records);
      } else if (Array.isArray(recordRes)) {
        setAnswerRecords(recordRes);
      } else {
        setAnswerRecords(recordRes?.records || []);
      }
    } catch (e: any) {
      console.error(e);
      message.error('加载面试详情失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  if (loading) {
    return (
      <div className="flex justify-center items-center min-h-screen bg-slate-50">
        <Spin size="large" tip="正在生成详细分析..." />
      </div>
    );
  }

  if (!evaluation) {
    return (
      <div className="min-h-screen bg-slate-50 flex items-center justify-center p-4">
        <AntCard className="rounded-3xl shadow-xl border-0 text-center p-10 max-w-md w-full">
          <div className="mb-4 text-6xl">📊</div>
          <h3 className="text-xl font-bold text-slate-700 mb-2">暂无分析报告</h3>
          <p className="text-slate-500 mb-6">
            面试可能仍在进行中，或数据正在处理中。
          </p>
          <Button
            type="primary"
            onClick={() => window.history.back()}
            className="bg-blue-600 rounded-xl h-10 px-6"
          >
            返回列表
          </Button>
        </AntCard>
      </div>
    );
  }

  const basic = {
    candidate: user?.name || '未知用户',
    resume: 'N/A',
    type: toEnglishLabel(interviewInfo?.type, interviewTypeMap),
    score: evaluation.score,
    difficulty: toEnglishLabel(interviewInfo?.difficulty, difficultyMap),
    company: interviewInfo?.company_name || '未填写',
    position: interviewInfo?.position_name || '未填写',
    duration: formatDuration(interviewInfo?.duration),
    time: interviewInfo?.created_at
      ? new Date(interviewInfo.created_at).toLocaleString('en-US')
      : '未知',
  };

  return (
    <div className="min-h-screen relative font-sans bg-slate-50/50 pb-20">
      {/* Decorative Background */}
      <div className="fixed top-0 left-0 w-full h-[400px] bg-gradient-to-b from-blue-50/80 to-transparent pointer-events-none z-0" />
      <div className="fixed top-0 right-0 w-[600px] h-[600px] bg-indigo-50/60 rounded-full blur-[120px] -translate-y-1/2 translate-x-1/3 pointer-events-none z-0" />

      <div className="container mx-auto px-4 relative z-10 pt-8">
        <div className="mb-8 animate-fade-in-up flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-extrabold text-slate-900 tracking-tight flex items-center gap-3 break-words [overflow-wrap:anywhere]">
              <TrophyOutlined className="text-yellow-500" />
              面试结果分析
            </h1>
            <p className="text-slate-500 mt-2 ml-11 break-words [overflow-wrap:anywhere]">
              基于 AI 洞察，对你的面试表现进行全方位复盘。
            </p>
          </div>
          <Button
            onClick={() => window.history.back()}
            className="rounded-xl border-slate-200 hover:border-blue-400 hover:text-blue-600"
          >
            返回列表
          </Button>
        </div>

        {/* Header Info Card */}
        <div className="bg-white rounded-3xl p-8 border border-slate-100 shadow-xl shadow-slate-200/50 animate-fade-in-up mb-8">
          <Row gutter={[48, 24]} align="middle">
            <Col xs={24} lg={14}>
              <div className="flex items-start gap-6 min-w-0">
                <Avatar
                  size={80}
                  className="bg-blue-100 text-blue-600 font-bold text-2xl border-4 border-white shadow-lg"
                >
                  {basic.candidate.substring(0, 2).toUpperCase()}
                </Avatar>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-3 mb-2 min-w-0">
                    <h2 className="text-2xl font-bold text-slate-800 m-0 break-words [overflow-wrap:anywhere]">
                      {basic.candidate}
                    </h2>
                    <Tag
                      color="blue"
                      className="rounded-full px-3 border-0 bg-blue-50 text-blue-700 font-medium max-w-full break-words [overflow-wrap:anywhere]"
                    >
                      {basic.type}
                    </Tag>
                  </div>

                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-y-3 gap-x-8 text-slate-600 mt-4">
                    <div className="flex items-center gap-2">
                      <EnvironmentOutlined className="text-slate-400" />
                      <span className="break-words [overflow-wrap:anywhere] min-w-0">
                        公司：{' '}
                        <span className="font-medium text-slate-800 break-words [overflow-wrap:anywhere]">
                          {basic.company}
                        </span>
                      </span>
                    </div>
                    <div className="flex items-center gap-2">
                      <UserOutlined className="text-slate-400" />
                      <span className="break-words [overflow-wrap:anywhere] min-w-0">
                        岗位：{' '}
                        <span className="font-medium text-slate-800 break-words [overflow-wrap:anywhere]">
                          {basic.position}
                        </span>
                      </span>
                    </div>
                    <div className="flex items-center gap-2">
                      <BarChartOutlined className="text-slate-400" />
                      <span>
                        难度：{' '}
                        <span className="font-medium text-slate-800">{basic.difficulty}</span>
                      </span>
                    </div>
                    <div className="flex items-center gap-2">
                      <ClockCircleOutlined className="text-slate-400" />
                      <span>
                        时长：{' '}
                        <span className="font-medium text-slate-800">{basic.duration}</span>
                      </span>
                    </div>
                    <div className="flex items-center gap-2 sm:col-span-2">
                      <CalendarOutlined className="text-slate-400" />
                      <span>
                        日期：<span className="font-medium text-slate-800">{basic.time}</span>
                      </span>
                    </div>
                  </div>
                </div>
              </div>
            </Col>
            <Col xs={24} lg={10}>
              <div className="bg-slate-50 rounded-2xl px-6 py-10 flex flex-col xl:flex-row items-center justify-between gap-6 border border-slate-100">
                <div className="flex-1 text-center xl:border-r border-slate-200 xl:pr-6 w-full xl:w-auto">
                  <ScoreGauge score={basic.score} />
                </div>
                <div className="flex flex-col gap-3 w-full xl:w-[180px]">
                  <Button
                    type="primary"
                    icon={<DownloadOutlined />}
                    className="bg-blue-600 hover:bg-blue-500 h-10 rounded-xl shadow-lg shadow-blue-200 border-0 w-full"
                  >
                    下载报告
                  </Button>
                  <Button
                    icon={<RobotOutlined />}
                    className="h-10 rounded-xl border-blue-200 text-blue-600 hover:bg-blue-50 hover:border-blue-300 w-full"
                  >
                    AI 改进建议
                  </Button>
                </div>
              </div>
            </Col>
          </Row>
        </div>

        {/* Analysis Content */}
        <Row gutter={[24, 24]}>
          <Col xs={24} lg={16}>
            {/* Interviewer Comment */}
            <div
              className="bg-white rounded-3xl p-8 border border-slate-100 shadow-lg shadow-slate-200/50 animate-fade-in-up h-full"
              style={{ animationDelay: '0.1s' }}
            >
              <div className="flex items-center gap-3 mb-6">
                <div className="bg-indigo-100 p-2 rounded-lg text-indigo-600">
                  <CheckCircleOutlined className="text-xl" />
                </div>
                <h3 className="text-xl font-bold text-slate-800 m-0">
                  面试官综合反馈
                </h3>
              </div>
              <div className="bg-indigo-50/50 p-6 rounded-2xl border border-indigo-50 text-slate-700 leading-relaxed text-lg">
                <ExpandableMobileText
                  content={evaluation.comment}
                  lines={4}
                  threshold={160}
                  textClassName="text-slate-700 leading-relaxed text-lg"
                />
              </div>

              <Divider className="my-8" />

              <div className="flex items-center gap-3 mb-6">
                <div className="bg-emerald-100 p-2 rounded-lg text-emerald-600">
                  <BarChartOutlined className="text-xl" />
                </div>
                <h3 className="text-xl font-bold text-slate-800 m-0">
                  维度详细分析
                </h3>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {evaluation.dimensions?.map((d: any, i: number) => (
                  <div
                    key={i}
                    className="bg-white border border-slate-200 rounded-xl p-5 hover:shadow-md transition-shadow duration-300"
                  >
                    <div className="flex justify-between items-center gap-3 mb-3 min-w-0">
                      <Tag
                        color="cyan"
                        className="rounded-md px-2 py-0.5 text-sm font-medium m-0 border-0 bg-cyan-50 text-cyan-700 max-w-full break-words [overflow-wrap:anywhere]"
                      >
                        {dimensionNameMap[d.dimension_name] || d.dimension_name}
                      </Tag>
                      <span className="font-bold text-slate-800 text-lg">
                        {d.score} <span className="text-xs text-slate-400 font-normal">/ 100</span>
                      </span>
                    </div>
                    <ExpandableMobileText
                      content={d.evaluation}
                      lines={3}
                      threshold={120}
                      textClassName="text-slate-600 text-sm leading-relaxed m-0"
                    />
                  </div>
                ))}
              </div>
            </div>
          </Col>

          <Col xs={24} lg={8}>
            {/* Radar Chart */}
            <div
              className="bg-white rounded-3xl p-8 border border-slate-100 shadow-lg shadow-slate-200/50 animate-fade-in-up h-full"
              style={{ animationDelay: '0.2s' }}
            >
              <div className="flex items-center gap-3 mb-6">
                <div className="bg-purple-100 p-2 rounded-lg text-purple-600">
                  <BulbOutlined className="text-xl" />
                </div>
                <h3 className="text-xl font-bold text-slate-800 m-0">能力雷达图</h3>
              </div>
              <div className="flex justify-center items-center py-4">
                {evaluation.dimensions && evaluation.dimensions.length > 0 ? (
                  <RadarChart items={evaluation.dimensions} size={320} />
                ) : (
                  <div className="text-slate-400 py-10">暂无维度数据</div>
                )}
              </div>
              <div className="text-center text-slate-500 text-sm mt-4">
                基于本次面试生成的五维能力模型。
              </div>
            </div>
          </Col>
        </Row>

        {/* Q&A Records */}
        <div
          className="mt-8 bg-white rounded-3xl p-8 border border-slate-100 shadow-xl shadow-slate-200/50 animate-fade-in-up"
          style={{ animationDelay: '0.3s' }}
        >
          <div className="flex items-center gap-3 mb-8">
            <div className="bg-orange-100 p-2 rounded-lg text-orange-600">
              <ClockCircleOutlined className="text-xl" />
            </div>
            <h3 className="text-xl font-bold text-slate-800 m-0">完整问答复盘</h3>
          </div>

          {answerRecords && answerRecords.length > 0 ? (
            <div className="space-y-8">
              {answerRecords.map((rec: any, index) => (
                <div key={index} className="group">
                  <div className="flex items-center gap-3 mb-4">
                    <div className="bg-slate-800 text-white w-8 h-8 rounded-lg flex items-center justify-center font-bold shadow-md">
                      {rec.order}
                    </div>
                    <ExpandableMobileText
                      content={rec.content}
                      lines={2}
                      threshold={80}
                      textClassName="text-lg font-bold text-slate-800 group-hover:text-blue-600 transition-colors"
                    />
                  </div>

                  <div className="border-l-2 border-slate-200 ml-4 pl-8 pb-8 space-y-6">
                    {/* QA Pairs */}
                    {rec.message?.map((m: any, mIdx: number) => (
                      <div
                        key={mIdx}
                        className="bg-slate-50 rounded-2xl p-6 border border-slate-100"
                      >
                        <div className="mb-4">
                          <span className="inline-block bg-blue-100 text-blue-700 text-xs font-bold px-2 py-1 rounded mb-2">
                            面试官提问
                          </span>
                          <ExpandableMobileText
                            content={m.question}
                            lines={3}
                            threshold={100}
                            textClassName="text-slate-800 font-medium text-lg"
                          />
                        </div>
                        <div className="bg-white rounded-xl p-4 border border-slate-100 shadow-sm">
                          <span className="inline-block bg-green-100 text-green-700 text-xs font-bold px-2 py-1 rounded mb-2">
                            你的回答
                          </span>
                          <ExpandableMobileText
                            content={m.answer}
                            lines={4}
                            threshold={140}
                            textClassName="text-slate-600 leading-relaxed"
                          />
                        </div>
                      </div>
                    ))}

                    {/* AI Comment for this Question */}
                    {rec.comment && (
                      <div className="bg-gradient-to-r from-orange-50 to-rose-50 rounded-2xl p-6 border border-orange-100">
                        <div className="flex items-center gap-2 mb-4 text-orange-700 font-bold">
                          <RobotOutlined /> AI 深度点评
                        </div>

                        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                          <div className="bg-white/60 rounded-xl p-4">
                            <div className="text-xs text-slate-400 uppercase tracking-wider font-bold mb-1">
                              单题得分
                            </div>
                            <div className="text-2xl font-bold text-orange-600">
                              {rec.comment.score}{' '}
                              <span className="text-sm text-slate-400">分</span>
                            </div>
                          </div>
                          <div className="bg-white/60 rounded-xl p-4">
                            <div className="text-xs text-slate-400 uppercase tracking-wider font-bold mb-1">
                              难度
                            </div>
                            <div className="text-lg font-bold text-slate-700">
                              {toEnglishLabel(rec.comment.difficulty, difficultyMap)}
                            </div>
                          </div>
                        </div>

                        <div className="mt-6 space-y-4">
                          {[
                            { label: '要点', val: rec.comment.key_points },
                            {
                              label: '优势',
                              val: rec.comment.strengths,
                              color: 'text-green-700',
                            },
                            {
                              label: '不足',
                              val: rec.comment.weaknesses,
                              color: 'text-red-600',
                            },
                            {
                              label: '建议',
                              val: rec.comment.suggestion,
                              color: 'text-blue-600',
                            },
                            { label: '推荐思路', val: rec.comment.thinking },
                            { label: '参考答案', val: rec.comment.reference },
                          ].map(
                            (item, idx) =>
                              item.val && (
                                <div
                                  key={idx}
                                  className="flex flex-col sm:flex-row gap-2 sm:gap-4 text-sm"
                                >
                                  <div className="min-w-[120px] font-bold text-slate-500 sm:text-right">
                                    {item.label}
                                  </div>
                                  <div
                                    className={`flex-1 ${item.color || 'text-slate-700'} leading-relaxed bg-white/40 p-2 rounded-lg`}
                                  >
                                    <ExpandableMobileText
                                      content={item.val}
                                      lines={4}
                                      threshold={140}
                                      textClassName={`${item.color || 'text-slate-700'} leading-relaxed`}
                                    />
                                  </div>
                                </div>
                              )
                          )}
                        </div>
                      </div>
                    )}
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="text-center py-12">
              <div className="text-6xl mb-4 opacity-20">📝</div>
              <div className="text-slate-400">暂无答题记录</div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

