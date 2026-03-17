'use client';

import { useEffect, useRef, useState } from 'react';
import { useRouter } from 'next/navigation';
import { INTERVIEW_API } from '@/config/api';
import { Typography, Button, Input, Avatar, message } from 'antd';
import FailoverModal, { FailoverData } from '@/components/FailoverModal';
import {
  AudioOutlined,
  CustomerServiceOutlined,
  QuestionCircleOutlined,
  SendOutlined,
  StopOutlined,
} from '@ant-design/icons';
import { useASRCapability } from '@/hooks/useASRCapability';
import { useSpeechAnswerInput } from '@/hooks/useSpeechAnswerInput';

interface ConversationItem {
  type: 'question' | 'answer';
  content: string;
  index?: number;
  timestamp: number;
}

export default function SpecialInterviewStartPage() {
  const [elapsed, setElapsed] = useState(0);
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [questionText, setQuestionText] = useState<string>('');
  const [questionIndex, setQuestionIndex] = useState<number>(0);
  const [answeredCount, setAnsweredCount] = useState(0);
  const [answer, setAnswer] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const abortControllerRef = useRef<AbortController | null>(null);
  const [starting, setStarting] = useState(false);
  const [waitingNextQuestion, setWaitingNextQuestion] = useState(false);
  const [conversationHistory, setConversationHistory] = useState<ConversationItem[]>([]);
  const [failoverData, setFailoverData] = useState<FailoverData | null>(null);
  const [showFailoverModal, setShowFailoverModal] = useState(false);
  const chatContainerRef = useRef<HTMLDivElement>(null);
  const [currentDomain, setCurrentDomain] = useState<string>('');
  const asrCapability = useASRCapability();
  const speechInput = useSpeechAnswerInput({
    enabled: asrCapability.enabled,
    sessionId,
    interviewType: '专项面试',
    domain: currentDomain,
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
      ? '语音识别暂不可用'
      : speechInput.status === 'recording'
        ? '正在录音，点击停止录音按钮结束'
        : speechInput.status === 'stopping'
          ? '正在结束录音，请稍候...'
          : speechInput.status === 'transcribing'
            ? '语音识别中，请稍候...'
            : speechInput.status === 'error'
              ? '语音识别失败，可重新尝试'
              : '支持语音输入，识别结果会回填到输入框';

  useEffect(() => {
    const timer = setInterval(() => setElapsed((prev) => prev + 1), 1000);
    return () => clearInterval(timer);
  }, []);

  useEffect(() => {
    const params =
      (window as any).__interviewParams ||
      (() => {
        try {
          return JSON.parse(sessionStorage.getItem('interviewParams') || 'null');
        } catch {
          return null;
        }
      })();
    if (!params) {
      message.error('缺少面试参数，请从表单页重新进入');
      return;
    }
    setCurrentDomain(params.domain || '');
    setStarting(true);

    // 接口传参格式必须严格遵循以下JSON结构
    const requestBody = {
      type: '专项面试',
      domain: String(params.domain || ''),
      difficulty: String(params.difficulty || ''),
    };

    const abortController = new AbortController();
    abortControllerRef.current = abortController;

    const startInterview = async () => {
      try {
        const token = localStorage.getItem('token');
        if (!token) {
          message.error('请先登录后再开始面试');
          setStarting(false);
          return;
        }

        // 先测试后端服务是否可达
        console.log('[检测] 测试后端服务连接...');
        try {
          const testResponse = await fetch(`${INTERVIEW_API.START_STREAM}`, {
            method: 'OPTIONS',
            mode: 'cors',
          });
          console.log('[检测] 后端服务连接正常');
        } catch (e) {
          const apiUrl = process.env.NEXT_PUBLIC_API_BASE_URL || '/api';
          message.error(`无法连接到后端服务，请确认后端服务是否运行在 ${apiUrl}`);
          setStarting(false);
          console.error('[检测] 后端服务连接失败:', e);
          return;
        }

        // 使用JSON格式发送请求
        let response;
        console.log('[面试启动] 使用JSON格式发送请求');
        console.log('[面试启动] 请求参数:', requestBody);

        try {
          response = await fetch(`${INTERVIEW_API.START_STREAM}`, {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
              Authorization: `Bearer ${token}`,
            },
            body: JSON.stringify(requestBody),
            signal: abortController.signal,
            mode: 'cors',
          });
        } catch (headerError) {
          // 如果Authorization header方式失败，尝试使用URL参数
          console.log('[面试启动] 方案1失败，尝试方案2: 使用URL参数传递token');
          const urlWithToken = `${INTERVIEW_API.START_STREAM}?token=${encodeURIComponent(token)}`;

          response = await fetch(urlWithToken, {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
            },
            body: JSON.stringify(requestBody),
            signal: abortController.signal,
            mode: 'cors',
          });
        }

        console.log('[面试启动] 收到响应:', response.status, response.statusText);

        if (!response.ok) {
          if (response.status === 401) {
            message.error('登录已过期，请重新登录');
          } else if (response.status === 404) {
            console.error('[面试启动] 404错误 - 接口不存在');
            message.error({
              content:
                '接口返回404，请在后端 middleware.go 中将 /api/interview/stream/start 添加到 jwtPublicRoutes',
              duration: 10,
            });
          } else {
            message.error(`面试启动失败：${response.status} ${response.statusText}`);
          }
          setStarting(false);
          return;
        }

        if (!response.body) {
          message.error('无法读取响应流');
          setStarting(false);
          return;
        }

        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';

        while (true) {
          const { done, value } = await reader.read();
          if (done) {
            setStarting(false);
            break;
          }

          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split('\n\n');
          buffer = lines.pop() || '';

          for (const block of lines) {
            // SSE 格式可能是 "event: xxx\ndata: {...}" 或仅 "data: {...}"
            const dataMatch = block.match(/^data:\s*(.+)$/m);
            if (dataMatch) {
              const json = dataMatch[1];
              try {
                const payload = JSON.parse(json);
                console.log('[SSE数据]', payload);
                if (payload?.type === 'session_id') {
                  const sid = payload.session_id || payload.data?.session_id || '';
                  console.log('[会话ID]', sid);
                  setSessionId(sid);
                  setStarting(false);
                } else if (payload?.type === 'start') {
                  const sid = payload.session_id || '';
                  console.log('[面试开始] session_id:', sid);
                  setSessionId(sid);
                  setStarting(false);
                } else if (payload?.type === 'model_failover_required') {
                  console.error('[模型失效]', payload.data);
                  setFailoverData(payload.data);
                  setShowFailoverModal(true);
                  setStarting(false);
                  setSubmitting(false);
                  setWaitingNextQuestion(false);
                  break; // Interrupt the stream loop
                } else if (
                  payload?.type === 'chunk' ||
                  payload?.type === 'question' ||
                  payload?.type === 'follow_up_question'
                ) {
                  // 兼容新旧两种格式
                  // 新格式: { type: "chunk", content: "...", role: "..." }
                  // 旧格式: { type: "chunk", data: { question_text: "..." } }
                  const q = payload.content || payload.data?.question_text || '';
                  const idx = payload.index || payload.data?.index || questionIndex || 1;

                  setStarting(false);
                  setWaitingNextQuestion(false);

                  setConversationHistory((prev) => {
                    let lastQuestionIdx = -1;
                    for (let i = prev.length - 1; i >= 0; i--) {
                      if (prev[i].type === 'question') {
                        lastQuestionIdx = i;
                        break;
                      }
                    }

                    if (lastQuestionIdx !== -1 && prev[lastQuestionIdx].index === idx) {
                      const updated = [...prev];
                      if (payload?.type === 'chunk') {
                        updated[lastQuestionIdx] = {
                          ...updated[lastQuestionIdx],
                          content: updated[lastQuestionIdx].content + q,
                        };
                      } else {
                        updated[lastQuestionIdx] = {
                          ...updated[lastQuestionIdx],
                          content: q,
                        };
                      }
                      setQuestionText(updated[lastQuestionIdx].content);
                      setQuestionIndex(idx);
                      return updated;
                    } else {
                      if (!q) return prev;
                      setQuestionText(q);
                      setQuestionIndex(idx);
                      return [
                        ...prev,
                        {
                          type: 'question',
                          content: q,
                          index: idx,
                          timestamp: Date.now(),
                        },
                      ];
                    }
                  });
                } else if (payload?.type === 'end' || payload?.type === 'complete') {
                  console.log('[面试结束]', payload);
                  message.success('面试已完成，正在跳转到面试记录...');
                  setStarting(false);
                  setWaitingNextQuestion(false);
                  // 延迟跳转，让用户看到提示消息
                  setTimeout(() => {
                    router.push('/user/interviews');
                  }, 1500);
                }
              } catch (e) {
                console.error('解析SSE数据失败:', json, e);
              }
            }
          }
        }
      } catch (error: any) {
        if (error.name !== 'AbortError') {
          message.error('面试启动失败：网络错误');
          console.error('启动面试错误:', error);
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

  // 自动滚动到底部
  useEffect(() => {
    if (chatContainerRef.current) {
      chatContainerRef.current.scrollTop = chatContainerRef.current.scrollHeight;
    }
  }, [conversationHistory, waitingNextQuestion]);

  const mm = String(Math.floor(elapsed / 60)).padStart(2, '0');
  const ss = String(elapsed % 60).padStart(2, '0');
  // 总共10道题，根据当前题目序号计算进度
  const percent = Math.min(100, Math.round((questionIndex / 10) * 100));

  const onSubmit = async (act?: 'next' | 'quit') => {
    if (!sessionId) {
      message.warning('会话已失效，请重新开始面试');
      return;
    }

    const action = act || 'next';

    // 如果是结束面试
    if (action === 'quit') {
      try {
        const token = localStorage.getItem('token');
        if (token) {
          await fetch(`${INTERVIEW_API.END_INTERVIEW}`, {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
              Authorization: `Bearer ${token}`,
            },
            body: JSON.stringify({
              session_id: sessionId,
            }),
            mode: 'cors',
          });
        }
      } catch (e) {
        console.error('[结束面试] 请求失败:', e);
      }
      try {
        abortControllerRef.current?.abort();
      } catch {}
      message.success('面试已结束，正在跳转...');
      router.push('/user/interviews');
      return;
    }

    // 验证答案不为空
    if (!answer.trim()) {
      message.warning('请输入答案后再提交');
      return;
    }

    setSubmitting(true);
    setAnsweredCount((prev) => prev + 1);
    const currentAnswer = answer;

    // Add answer to conversation history immediately
    setConversationHistory((prev) => [
      ...prev,
      {
        type: 'answer',
        content: currentAnswer,
        timestamp: Date.now(),
      },
    ]);

    setAnswer('');
    setWaitingNextQuestion(true);

    try {
      const token = localStorage.getItem('token');
      if (!token) {
        message.error('登录已过期，请重新登录');
        setSubmitting(false);
        setWaitingNextQuestion(false);
        return;
      }

      const response = await fetch(`${INTERVIEW_API.SUBMIT_ANSWER}`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          session_id: sessionId,
          answer: currentAnswer,
        }),
        mode: 'cors',
      });

      console.log('[提交答案] 响应状态:', response.status);

      if (!response.ok) {
        const errorText = await response.text();
        console.error('[提交答案] 错误响应:', errorText);
        message.error('答案提交失败，请重试');
        setWaitingNextQuestion(false);
        return;
      }

      // 提交成功，等待SSE流推送下一题
      console.log('[提交答案] 提交成功，等待SSE推送下一题');
      message.success('答案已提交，正在生成下一题...');
    } catch (error: any) {
      console.error('[提交答案] 异常:', error);
      message.error('答案提交失败：网络错误');
      setWaitingNextQuestion(false);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="min-h-screen bg-slate-50 relative flex flex-col font-sans -my-8">
      {/* Decorative Background */}
      <div className="fixed top-0 right-0 w-[500px] h-[500px] bg-purple-100/40 rounded-full blur-[100px] -translate-y-1/2 translate-x-1/3 pointer-events-none z-0" />
      <div className="fixed bottom-0 left-0 w-[500px] h-[500px] bg-pink-100/40 rounded-full blur-[100px] translate-y-1/2 -translate-x-1/3 pointer-events-none z-0" />

      {/* Header */}
      <header className="sticky top-0 z-50 bg-white/80 backdrop-blur-md border-b border-slate-200/60 shadow-sm transition-all duration-300">
        <div className="max-w-4xl mx-auto px-4 h-16 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-8 h-8 bg-purple-100 rounded-lg flex items-center justify-center text-purple-600">
              <CustomerServiceOutlined />
            </div>
            <div>
              <h1 className="text-base font-bold text-slate-800 m-0 leading-tight">专项面试</h1>
              <p className="text-xs text-slate-500 m-0">{currentDomain || '技能专项练习'}</p>
            </div>
          </div>

          <div className="flex items-center gap-4">
            <div className="hidden md:flex items-center gap-3 bg-slate-50 px-3 py-1.5 rounded-full border border-slate-100">
              <div className="flex items-center gap-1.5">
                <div className="w-2 h-2 rounded-full bg-purple-500 animate-pulse" />
                <span className="text-xs font-medium text-slate-600 font-mono">
                  {mm}:{ss}
                </span>
              </div>
              <div className="w-px h-3 bg-slate-200" />
              <div className="flex items-center gap-1.5">
                <span className="text-xs text-slate-500">进度 {percent}%</span>
                <div className="w-16 h-1.5 bg-slate-200 rounded-full overflow-hidden">
                  <div
                    className="h-full bg-purple-500 transition-all duration-500"
                    style={{ width: `${percent}%` }}
                  />
                </div>
              </div>
            </div>

            <Button
              danger
              ghost
              size="small"
              className="!rounded-full !px-4 hover:!bg-red-50 border-red-200"
              onClick={() => onSubmit('quit')}
            >
              结束面试
            </Button>
          </div>
        </div>
      </header>

      {/* Chat Area */}
      <main className="flex-1 overflow-y-auto relative z-10" ref={chatContainerRef}>
        <div className="max-w-4xl mx-auto px-4 py-8 space-y-8 pb-32">
          {conversationHistory.length === 0 && !starting && (
            <div className="flex flex-col items-center justify-center py-20 opacity-0 animate-fade-in-up">
              <div className="w-16 h-16 bg-purple-50 rounded-2xl flex items-center justify-center text-purple-500 text-2xl mb-4 animate-bounce-subtle">
                <CustomerServiceOutlined />
              </div>
              <p className="text-slate-400 text-sm">正在准备专项面试题...</p>
            </div>
          )}

          {conversationHistory.map((item, index) => {
            const isQuestion = item.type === 'question';
            const questionNumber = conversationHistory
              .slice(0, index + 1)
              .filter((i) => i.type === 'question').length;

            return (
              <div
                key={index}
                className={`flex gap-4 ${isQuestion ? 'items-start' : 'items-end flex-row-reverse'} animate-fade-in-up`}
              >
                <Avatar
                  src={`https://api.dicebear.com/7.x/avataaars/svg?seed=${isQuestion ? 'interviewer' : 'user'}`}
                  size={40}
                  className="border-2 border-white shadow-sm shrink-0"
                />

                <div
                  className={`flex flex-col max-w-[85%] md:max-w-[75%] ${isQuestion ? 'items-start' : 'items-end'}`}
                >
                  {isQuestion && (
                    <span className="text-xs text-slate-400 mb-1.5 ml-1">
                      面试官 · 第 {questionNumber} 题
                    </span>
                  )}

                  <div
                    className={`
                      relative px-6 py-4 text-[15px] leading-relaxed shadow-sm
                      ${
                        isQuestion
                          ? 'bg-white text-slate-700 rounded-2xl rounded-tl-none border border-slate-100'
                          : 'bg-gradient-to-br from-purple-500 to-fuchsia-600 text-white rounded-2xl rounded-tr-none shadow-purple-200'
                      }
                    `}
                  >
                    <div className="whitespace-pre-wrap break-words">{item.content}</div>
                  </div>

                  {!isQuestion && (
                    <span className="text-xs text-slate-400 mt-1.5 mr-1">
                      我 ·{' '}
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
                src="https://api.dicebear.com/7.x/avataaars/svg?seed=interviewer"
                size={40}
                className="border-2 border-white shadow-sm"
              />
              <div className="bg-white px-5 py-4 rounded-2xl rounded-tl-none border border-slate-100 shadow-sm flex items-center gap-2">
                <div className="flex gap-1">
                  <div
                    className="w-1.5 h-1.5 bg-slate-400 rounded-full animate-bounce"
                    style={{ animationDelay: '0s' }}
                  />
                  <div
                    className="w-1.5 h-1.5 bg-slate-400 rounded-full animate-bounce"
                    style={{ animationDelay: '0.2s' }}
                  />
                  <div
                    className="w-1.5 h-1.5 bg-slate-400 rounded-full animate-bounce"
                    style={{ animationDelay: '0.4s' }}
                  />
                </div>
                <span className="text-sm text-slate-400 ml-2">
                  {starting ? '正在生成首题...' : '正在思考下一题...'}
                </span>
              </div>
            </div>
          )}
        </div>
      </main>

      {/* Input Area */}
      <footer className="sticky bottom-0 z-50 bg-white/80 backdrop-blur-xl border-t border-slate-200/60 pb-6 pt-4">
        <div className="max-w-4xl mx-auto px-4">
          <div className="relative bg-white rounded-2xl border border-slate-200 shadow-lg shadow-slate-100/50 transition-all focus-within:shadow-xl focus-within:border-purple-400 focus-within:ring-1 focus-within:ring-purple-100">
            <Input.TextArea
              value={answer}
              onChange={(e) => setAnswer(e.target.value)}
              placeholder={
                speechInput.status === 'recording'
                  ? '正在录音，点击停止录音按钮结束...'
                  : speechInput.status === 'stopping'
                    ? '正在结束录音，请稍候...'
                    : speechInput.status === 'transcribing'
                      ? '语音识别中，请稍候...'
                      : waitingNextQuestion
                        ? '面试官正在提问...'
                        : '请输入你的回答...'
              }
              disabled={waitingNextQuestion || starting}
              readOnly={speechInput.isRecording || speechInput.isStopping}
              autoSize={{ minRows: 1, maxRows: 6 }}
              className="!border-0 !shadow-none !bg-transparent !text-base !px-4 !py-3 !resize-none placeholder:text-slate-400 focus:!shadow-none"
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey) {
                  e.preventDefault();
                  if (answer.trim()) onSubmit();
                }
              }}
            />

            <div className="relative z-10 flex justify-between items-center px-2 pb-2 pt-1 border-t border-slate-50">
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
                    {speechInput.isStopping ? '结束中...' : '停止录音'}
                  </Button>
                ) : (
                  <Button
                    type="text"
                    size="small"
                    icon={<AudioOutlined className="text-slate-400" />}
                    disabled={speechDisabled || asrCapability.loading}
                    onClick={speechInput.handleMicClick}
                    className="!text-slate-400 !w-9 !h-9"
                  />
                )}
                <Button
                  type="text"
                  size="small"
                  icon={<QuestionCircleOutlined className="text-slate-400" />}
                  className="!text-slate-400"
                />
              </div>
              <div className="flex items-center gap-3">
                <span className="text-xs text-slate-300 hidden sm:inline-block">Enter 发送</span>
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
                  className="!bg-purple-500 hover:!bg-purple-600 !shadow-purple-200 !border-0"
                >
                  发送
                </Button>
              </div>
            </div>
          </div>
          <div className="mt-2 px-2">
            <p className="text-xs text-slate-400">{speechHint}</p>
          </div>
          <div className="text-center mt-2">
            <p className="text-xs text-slate-300">面试吧</p>
          </div>
        </div>
      </footer>

      <FailoverModal
        open={showFailoverModal}
        data={failoverData}
        onSuccess={() => {
          setShowFailoverModal(false);
          setFailoverData(null);
          // Auto resubmit the previous action if possible
          if (answer.trim()) {
            onSubmit();
          } else {
            // In case it died generating the first question, just reload the page or force a restart.
            // For simplicity:
            if (conversationHistory.length === 0) {
              window.location.reload();
            } else {
              // Tell backend to keep going from current DB state if applicable.
              // It's safer to just resubmit an empty string or standard 'continue' action but backend
              // supports "continue" as an arbitrary action. Let's send an empty submission since
              // we're waiting for the question generation. Note: user might have to just refresh.
              onSubmit('continue' as any);
            }
          }
        }}
        onCancel={() => {
          setShowFailoverModal(false);
          router.push('/');
        }}
      />
    </div>
  );
}
