'use client';

import { useEffect, useRef, useState } from 'react';
import { useRouter } from 'next/navigation';
import { Typography, Button, Input, Avatar, Progress, message } from 'antd';
import { INTERVIEW_API } from '@/config/api';
import { useASRCapability } from '@/hooks/useASRCapability';
import { useSpeechAnswerInput } from '@/hooks/useSpeechAnswerInput';
import {
  AudioOutlined,
  SendOutlined,
  TeamOutlined,
  QuestionCircleOutlined,
  RobotOutlined,
  UserOutlined,
  SafetyCertificateOutlined,
  CodeOutlined,
  ProjectOutlined,
  StopOutlined,
} from '@ant-design/icons';
import {
  ConversationItem,
  RoleType,
  SSEStreamParser,
  detectRoleFromContent,
  getRoleConfig,
  StructuredMessage,
} from '@/types/message-schema';

const { Title } = Typography;

export default function MultiAgentInterviewStartPage() {
  const [elapsed, setElapsed] = useState(0);
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [questionText, setQuestionText] = useState('');
  const [questionIndex, setQuestionIndex] = useState<number>(0);
  const [answer, setAnswer] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const abortControllerRef = useRef<AbortController | null>(null);
  const [starting, setStarting] = useState(false);
  const [waitingNextQuestion, setWaitingNextQuestion] = useState(false);
  const [conversationHistory, setConversationHistory] = useState<ConversationItem[]>([]);
  const chatContainerRef = useRef<HTMLDivElement>(null);
  const asrCapability = useASRCapability();
  const speechInput = useSpeechAnswerInput({
    enabled: asrCapability.enabled,
    sessionId,
    interviewType: '多对一面试',
    questionText,
    onTranscript: (transcript) => {
      setAnswer((prev) => (prev.trim() ? `${prev.trim()}\n${transcript}` : transcript));
    },
  });
  const router = useRouter();
  const speechDisabled =
    !asrCapability.enabled ||
    !sessionId ||
    waitingNextQuestion ||
    starting ||
    speechInput.isTranscribing ||
    speechInput.isStopping;
  const speechHint =
    !asrCapability.loading && !asrCapability.enabled
      ? 'Speech recognition unavailable'
      : speechInput.status === 'recording'
        ? 'Recording, click stop button to end'
        : speechInput.status === 'stopping'
          ? 'Stopping recording, please wait...'
          : speechInput.status === 'transcribing'
            ? 'Transcribing, please wait...'
            : speechInput.status === 'error'
              ? 'Speech recognition failed, try again'
              : 'Voice input supported, results will fill the input box';

  useEffect(() => {
    const timer = setInterval(() => setElapsed((prev) => prev + 1), 1000);
    return () => clearInterval(timer);
  }, []);

  useEffect(() => {
    let params = (window as any).__interviewParams;
    if (!params) {
      try {
        params = JSON.parse(sessionStorage.getItem('interviewParams') || 'null');
      } catch {
        params = null;
      }
    }
    if (!params) {
      message.error('Missing parameters, please reconfigure');
      router.push('/interview/multi');
      return;
    }

    setStarting(true);
    const abortController = new AbortController();
    abortControllerRef.current = abortController;

    const startInterview = async () => {
      try {
        const token = localStorage.getItem('token');
        if (!token) {
          message.error('Please login first');
          setStarting(false);
          return;
        }

        const response = await fetch(`${INTERVIEW_API.START_STREAM}`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${token}`,
          },
          body: JSON.stringify({
            type: params.type,
            domain: params.domain,
            difficulty: params.difficulty,
            position_name: params.position_name,
            company_name: params.company_name,
            resume_id: params.resume_id,
          }),
          signal: abortController.signal,
        });

        if (!response.ok) {
          throw new Error(`HTTP ${response.status}`);
        }

        if (!response.body) return;

        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        const parser = new SSEStreamParser();

        while (true) {
          const { done, value } = await reader.read();
          if (done) {
            setStarting(false);
            break;
          }

          const chunk = decoder.decode(value, { stream: true });
          const events = parser.parse(chunk);

          for (const payload of events) {
            console.log('[SSE接收]', payload.type, payload);

            if (payload?.type === 'session_id' || payload?.type === 'start') {
              const sid = payload.session_id || payload.data?.session_id || '';
              if (sid) setSessionId(sid);
            } else if (payload?.type === 'structured_message') {
              // 处理结构化消息（新协议）
              const role: RoleType = payload.role || 'main_interviewer';
              const roleConfig = getRoleConfig(role);
              const content = payload.content || '';
              const idx = payload.index || questionIndex;

              setStarting(false);
              setWaitingNextQuestion(false);
              setQuestionIndex(idx);

              if (payload.action_type === 'thinking') {
                // 思考状态，不添加到历史
                continue;
              }

              setConversationHistory((prev) => {
                const lastIdx = prev.length - 1;

                // 如果是同一个角色的消息，追加内容
                if (
                  lastIdx >= 0 &&
                  prev[lastIdx].type === 'question' &&
                  prev[lastIdx].role === role
                ) {
                  const updated = [...prev];
                  updated[lastIdx] = {
                    ...updated[lastIdx],
                    content:
                      payload.status === 'streaming' ? updated[lastIdx].content + content : content,
                  };
                  setQuestionText(updated[lastIdx].content);
                  return updated;
                }

                // 新消息
                if (content) {
                  setQuestionText(content);
                  return [
                    ...prev,
                    {
                      type: 'question',
                      content,
                      role,
                      roleConfig,
                      index: idx,
                      timestamp: Date.now(),
                      actionType: payload.action_type || 'speaking',
                    },
                  ];
                }

                return prev;
              });
            } else if (payload?.type === 'chunk' || payload?.type === 'question') {
              // 兼容旧协议
              const q = payload.data?.question_text || payload.content || '';
              const idx = payload.index || payload.data?.index || 0;

              // 检测角色
              let role: RoleType = 'main_interviewer';
              let roleConfig = getRoleConfig(role);

              if (payload.role) {
                role = payload.role;
                roleConfig = getRoleConfig(role);
              } else if (q) {
                role = detectRoleFromContent(q);
                roleConfig = getRoleConfig(role);
              }

              setStarting(false);
              setWaitingNextQuestion(false);
              setQuestionIndex(idx);

              setConversationHistory((prev) => {
                let lastQuestionIdx = -1;
                for (let i = prev.length - 1; i >= 0; i--) {
                  if (prev[i].type === 'question') {
                    lastQuestionIdx = i;
                    break;
                  }
                }

                // 索引一致且是 chunk，追加
                if (
                  payload?.type === 'chunk' &&
                  lastQuestionIdx !== -1 &&
                  prev[lastQuestionIdx].index === idx
                ) {
                  const updated = [...prev];
                  updated[lastQuestionIdx] = {
                    ...updated[lastQuestionIdx],
                    content: updated[lastQuestionIdx].content + q,
                    role,
                    roleConfig,
                  };
                  setQuestionText(updated[lastQuestionIdx].content);
                  return updated;
                }

                // 索引一致且是完整 question，覆盖校准
                if (
                  payload?.type === 'question' &&
                  lastQuestionIdx !== -1 &&
                  prev[lastQuestionIdx].index === idx
                ) {
                  const updated = [...prev];
                  updated[lastQuestionIdx] = {
                    ...updated[lastQuestionIdx],
                    content: q,
                    role,
                    roleConfig,
                  };
                  setQuestionText(q);
                  return updated;
                }

                // 新题
                if (q) {
                  setQuestionText(q);
                  return [
                    ...prev,
                    {
                      type: 'question',
                      content: q,
                      role,
                      roleConfig,
                      index: idx,
                      timestamp: Date.now(),
                    },
                  ];
                }

                return prev;
              });
            } else if (payload?.type === 'thinking') {
              // 思考状态
              setWaitingNextQuestion(true);
            } else if (payload?.type === 'ready_for_answer') {
              setWaitingNextQuestion(false);
              setStarting(false);
            } else if (payload?.type === 'end' || payload?.type === 'complete') {
              message.success('Interview completed, redirecting to records...');
              setStarting(false);
              setWaitingNextQuestion(false);
              // 延迟跳转，让用户看到提示消息
              setTimeout(() => {
                router.push('/user/interviews');
              }, 1500);
            }
          }
        }
      } catch (error: any) {
        if (error.name !== 'AbortError') {
          message.error('Failed to connect to interview server');
        }
        setStarting(false);
      }
    };

    startInterview();

    return () => {
      abortController.abort();
      abortControllerRef.current = null;
    };
  }, []);

  useEffect(() => {
    if (chatContainerRef.current) {
      chatContainerRef.current.scrollTop = chatContainerRef.current.scrollHeight;
    }
  }, [conversationHistory, waitingNextQuestion, starting]);

  const onSubmit = async (act?: 'next' | 'quit') => {
    if (!sessionId) return;
    const action = act || 'next';

    if (action === 'quit') {
      try {
        const token = localStorage.getItem('token');
        await fetch(`${INTERVIEW_API.END_INTERVIEW}`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${token}`,
          },
          body: JSON.stringify({ session_id: sessionId }),
        });
      } catch (e) {}
      router.push('/user/interviews');
      return;
    }

    if (!answer.trim() || submitting) return;

    setSubmitting(true);
    const currentAnswer = answer;
    setConversationHistory((prev) => [
      ...prev,
      { type: 'answer', content: currentAnswer, timestamp: Date.now() },
    ]);
    setAnswer('');
    setWaitingNextQuestion(true);

    try {
      const token = localStorage.getItem('token');
      const res = await fetch(`${INTERVIEW_API.SUBMIT_ANSWER}`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ session_id: sessionId, answer: currentAnswer }),
      });

      if (!res.ok) throw new Error('Submission failed');
    } catch (error) {
      message.error('答案Submission failed');
      setWaitingNextQuestion(false);
    } finally {
      setSubmitting(false);
    }
  };

  // 快捷切换面试官
  const switchInterviewer = async (interviewerType: 'tech' | 'project' | 'main') => {
    if (!sessionId || !answer.trim() || submitting) return;

    const interviewerMap = {
      tech: 'Tech Interviewer',
      project: 'Project Interviewer',
      main: 'Lead Interviewer',
    };

    const switchRequest = `I would like to discuss this with ${interviewerMap[interviewerType]}: ${answer}`;

    setSubmitting(true);
    const currentAnswer = switchRequest;
    setConversationHistory((prev) => [
      ...prev,
      { type: 'answer', content: currentAnswer, timestamp: Date.now() },
    ]);
    setAnswer('');
    setWaitingNextQuestion(true);

    try {
      const token = localStorage.getItem('token');
      const res = await fetch(`${INTERVIEW_API.SUBMIT_ANSWER}`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ session_id: sessionId, answer: currentAnswer }),
      });

      if (!res.ok) throw new Error('Submission failed');
    } catch (error) {
      message.error('Failed to switch interviewer');
      setWaitingNextQuestion(false);
    } finally {
      setSubmitting(false);
    }
  };

  const mm = String(Math.floor(elapsed / 60)).padStart(2, '0');
  const ss = String(elapsed % 60).padStart(2, '0');
  const percent = Math.min(100, Math.round((questionIndex / 10) * 100));

  return (
    <div className="min-h-screen bg-slate-50 relative flex flex-col font-sans -my-8">
      <div className="fixed top-0 right-0 w-[500px] h-[500px] bg-orange-100/30 rounded-full blur-[100px] -translate-y-1/2 translate-x-1/3 pointer-events-none z-0" />
      <div className="fixed bottom-0 left-0 w-[400px] h-[400px] bg-amber-100/30 rounded-full blur-[100px] translate-y-1/2 -translate-x-1/3 pointer-events-none z-0" />

      <header className="sticky top-0 z-50 bg-white/80 backdrop-blur-md border-b border-slate-200/60 shadow-sm">
        <div className="max-w-4xl mx-auto px-4 h-16 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 bg-orange-100 rounded-xl flex items-center justify-center text-orange-600 shadow-inner">
              <TeamOutlined style={{ fontSize: '20px' }} />
            </div>
            <div>
              <h1 className="text-base font-bold text-slate-800 m-0 leading-tight">
                Multi-Agent Mock Interview (Eino)
              </h1>
              <p className="text-[10px] text-slate-400 m-0 uppercase tracking-widest font-semibold mt-0.5">
                Expert Panel Interview
              </p>
            </div>
          </div>

          <div className="flex items-center gap-4">
            <div className="hidden md:flex items-center gap-3 bg-slate-50 px-4 py-2 rounded-full border border-slate-100">
              <span className="text-xs font-bold text-slate-600 font-mono tracking-tighter w-12 text-center">
                {mm}:{ss}
              </span>
              <div className="w-px h-3 bg-slate-200" />
              <div className="flex items-center gap-2">
                <span className="text-[10px] text-slate-400 font-bold uppercase">Progress</span>
                <div className="w-20 h-1.5 bg-slate-200 rounded-full overflow-hidden">
                  <div
                    className="h-full bg-orange-500 transition-all duration-500"
                    style={{ width: `${percent}%` }}
                  />
                </div>
              </div>
            </div>
            <Button
              danger
              ghost
              size="small"
              className="!rounded-full !px-4 hover:!bg-red-50 border-red-200 font-medium"
              onClick={() => onSubmit('quit')}
            >
              Exit Early
            </Button>
          </div>
        </div>
      </header>

      <main className="flex-1 overflow-y-auto relative z-10 scroll-smooth" ref={chatContainerRef}>
        <div className="max-w-4xl mx-auto px-4 py-10 space-y-8 pb-32">
          {conversationHistory.length === 0 && !starting && (
            <div className="flex flex-col items-center justify-center py-20 text-center animate-pulse">
              <RobotOutlined style={{ fontSize: '40px', color: '#cbd5e1' }} />
              <p className="text-slate-400 text-sm mt-4 font-medium">
                Interviewer panel is preparing the first question...
              </p>
            </div>
          )}

          {conversationHistory.map((item, index) => {
            const isQuestion = item.type === 'question';
            const questionSeq = conversationHistory
              .slice(0, index + 1)
              .filter((i) => i.type === 'question').length;

            // 优先使用 roleConfig，回退到内容检测
            let speaker = 'Lead Interviewer';
            let speakerColor = 'text-orange-500';
            let speakerBorder = 'border-orange-200';
            let avatarSeed = 'interviewer-main-v2';

            if (item.roleConfig) {
              // 使用结构化的角色配置（新协议）
              speaker = item.roleConfig.roleName;
              speakerColor = item.roleConfig.colorClass;
              speakerBorder = item.roleConfig.borderClass;
              avatarSeed = item.roleConfig.avatarSeed;
            } else {
              // 回退到基于内容的角色检测（兼容旧协议）
              const role = item.role || detectRoleFromContent(item.content);
              const config = getRoleConfig(role);
              speaker = config.roleName;
              speakerColor = config.colorClass;
              speakerBorder = config.borderClass;
              avatarSeed = config.avatarSeed;
            }

            return (
              <div
                key={index}
                className={`flex gap-4 ${isQuestion ? 'items-start' : 'items-end flex-row-reverse'} animate-fade-in-up`}
              >
                <Avatar
                  src={`https://api.dicebear.com/7.x/avataaars/svg?seed=${isQuestion ? avatarSeed : 'user-candidate-v2'}`}
                  size={44}
                  className={`border-2 shadow-sm shrink-0 bg-white ${isQuestion ? speakerBorder : 'border-blue-100'}`}
                  icon={isQuestion ? <RobotOutlined /> : <UserOutlined />}
                />

                <div
                  className={`flex flex-col max-w-[85%] md:max-w-[75%] ${isQuestion ? 'items-start' : 'items-end'}`}
                >
                  {isQuestion && (
                    <div className="flex items-center gap-2 mb-1.5 ml-1">
                      <span
                        className={`text-[10px] font-bold uppercase tracking-widest ${speakerColor}`}
                      >
                        {speaker}
                      </span>
                      <div className="w-1 h-1 bg-slate-300 rounded-full" />
                      <span className="text-[10px] text-slate-400 font-bold uppercase">
                        Q{questionSeq}
                      </span>
                    </div>
                  )}

                  <div
                    className={`relative px-6 py-4 text-[15px] leading-relaxed shadow-sm transition-all
                                        ${
                                          isQuestion
                                            ? `bg-white text-slate-700 rounded-2xl rounded-tl-none border ${speakerBorder} hover:border-orange-300`
                                            : 'bg-gradient-to-br from-orange-500 to-amber-600 text-white rounded-2xl rounded-tr-none shadow-orange-100'
                                        }`}
                  >
                    <div className="whitespace-pre-wrap break-words">{item.content}</div>
                  </div>

                  {!isQuestion && (
                    <span className="text-[10px] text-slate-400 mt-2 mr-1 font-bold uppercase tracking-tighter">
                      Candidate ·{' '}
                      {new Date(item.timestamp).toLocaleTimeString([], {
                        hour: '2-digit',
                        minute: '2-digit',
                      })}
                    </span>
                  )}
                </div>
              </div>
            );
          })}

          {(waitingNextQuestion || starting) && (
            <div className="flex gap-4 items-start animate-fade-in-up">
              <Avatar
                src="https://api.dicebear.com/7.x/avataaars/svg?seed=interviewer-thinking"
                size={44}
                className="border-2 border-orange-50 shadow-sm shrink-0 bg-white animate-pulse"
              />
              <div className="bg-white px-6 py-4 rounded-2xl rounded-tl-none border border-slate-100 shadow-sm flex items-center gap-3">
                <div className="flex gap-1">
                  <div
                    className="w-1.5 h-1.5 bg-orange-400 rounded-full animate-bounce"
                    style={{ animationDelay: '0s' }}
                  />
                  <div
                    className="w-1.5 h-1.5 bg-orange-400 rounded-full animate-bounce"
                    style={{ animationDelay: '0.2s' }}
                  />
                  <div
                    className="w-1.5 h-1.5 bg-orange-400 rounded-full animate-bounce"
                    style={{ animationDelay: '0.4s' }}
                  />
                </div>
                <span className="text-xs font-bold text-slate-400 uppercase tracking-widest">
                  {starting ? 'Interview panel coming online...' : 'Interviewers are conferring...'}
                </span>
              </div>
            </div>
          )}
        </div>
      </main>

      <footer className="sticky bottom-0 z-50 bg-white/80 backdrop-blur-xl border-t border-slate-200/60 pb-8 pt-5">
        <div className="max-w-4xl mx-auto px-4">
          {/* 快捷切换面试官按钮组 */}
          {sessionId && !waitingNextQuestion && !starting && answer.trim() && (
            <div className="mb-3 flex items-center gap-2 animate-fade-in-up">
              <span className="text-xs font-bold text-slate-400 uppercase tracking-wider">
                Quick Switch:
              </span>
              <Button
                size="small"
                icon={<TeamOutlined />}
                className="!rounded-full !text-xs !border-orange-200 !text-orange-600 hover:!bg-orange-50"
                onClick={() => switchInterviewer('main')}
              >
                Lead Interviewer
              </Button>
              <Button
                size="small"
                icon={<CodeOutlined />}
                className="!rounded-full !text-xs !border-blue-200 !text-blue-600 hover:!bg-blue-50"
                onClick={() => switchInterviewer('tech')}
              >
                Tech Interviewer
              </Button>
              <Button
                size="small"
                icon={<ProjectOutlined />}
                className="!rounded-full !text-xs !border-emerald-200 !text-emerald-600 hover:!bg-emerald-50"
                onClick={() => switchInterviewer('project')}
              >
                Project Interviewer
              </Button>
            </div>
          )}

          <div className="relative group">
            <div className="absolute -inset-1 bg-gradient-to-r from-orange-500 to-amber-500 rounded-3xl blur opacity-20 group-focus-within:opacity-40 transition-opacity duration-500" />
            <div className="relative bg-white rounded-2xl border border-slate-200 shadow-lg shadow-slate-100/50 transition-all focus-within:border-orange-400">
              <Input.TextArea
                value={answer}
                onChange={(e) => setAnswer(e.target.value)}
                placeholder={
                  speechInput.status === 'recording'
                    ? 'Recording, click stop button to end...'
                    : speechInput.status === 'stopping'
                      ? 'Stopping recording, please wait...'
                      : speechInput.status === 'transcribing'
                        ? 'Transcribing, please wait...'
                        : waitingNextQuestion
                          ? 'Interviewers are discussing...'
                          : 'Please enter your answer, press Enter to send...'
                }
                disabled={waitingNextQuestion || starting}
                readOnly={speechInput.isRecording || speechInput.isStopping}
                autoSize={{ minRows: 1, maxRows: 6 }}
                className="!border-0 !shadow-none !bg-transparent !text-base !px-5 !py-4 !resize-none placeholder:text-slate-300 font-medium"
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && !e.shiftKey) {
                    e.preventDefault();
                    if (answer.trim()) onSubmit();
                  }
                }}
              />

              <div className="relative z-10 flex justify-between items-center px-3 pb-3 pt-1 border-t border-slate-50/50">
                <div className="flex gap-1">
                  {speechInput.isRecording || speechInput.isStopping ? (
                    <Button
                      danger
                      shape="round"
                      size="small"
                      icon={<StopOutlined />}
                      disabled={speechInput.isStopping}
                      onClick={speechInput.handleMicClick}
                      className="!h-9 !px-4 !font-medium"
                    >
                      {speechInput.isStopping ? 'Stopping...' : 'Stop Rec.'}
                    </Button>
                  ) : (
                    <Button
                      type="text"
                      size="small"
                      icon={<AudioOutlined className="text-slate-300" />}
                      disabled={speechDisabled || asrCapability.loading}
                      onClick={speechInput.handleMicClick}
                      className="!text-slate-300 !w-9 !h-9"
                    />
                  )}
                  <Button
                    type="text"
                    size="small"
                    icon={<SafetyCertificateOutlined className="text-slate-300" />}
                  />
                  <Button
                    type="text"
                    size="small"
                    icon={<QuestionCircleOutlined className="text-slate-300" />}
                  />
                </div>
                <div className="flex items-center gap-3">
                  <span className="text-[10px] font-bold text-slate-300 hidden sm:inline-block tracking-widest uppercase">
                    Shift + Enter for new line
                  </span>
                  <Button
                    type="primary"
                    shape="round"
                    icon={<SendOutlined />}
                    loading={submitting}
                    disabled={
                      !sessionId ||
                      waitingNextQuestion ||
                      starting ||
                      speechInput.isRecording ||
                      speechInput.isStopping ||
                      speechInput.isTranscribing ||
                      !answer.trim()
                    }
                    onClick={() => onSubmit()}
                    className="!bg-orange-500 hover:!bg-orange-600 !shadow-orange-100 !border-0 font-bold px-6"
                  >
                    Send Answer
                  </Button>
                </div>
              </div>
            </div>
          </div>
          <div className="mt-2 px-2">
            <p className="text-xs text-slate-400">{speechHint}</p>
          </div>
        </div>
      </footer>
    </div>
  );
}
