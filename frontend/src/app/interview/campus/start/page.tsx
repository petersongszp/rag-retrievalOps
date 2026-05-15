'use client';

import { useEffect, useRef, useState } from 'react';
import { useRouter } from 'next/navigation';
import { Typography, Button, Input, Avatar, Progress, message } from 'antd';
import { INTERVIEW_API } from '@/config/api';
import { CustomerServiceOutlined } from '@ant-design/icons';
import { useASRCapability } from '@/hooks/useASRCapability';
import { useSpeechAnswerInput } from '@/hooks/useSpeechAnswerInput';
import { InterviewFooterActions } from '@/components/interview/InterviewFooterActions';

const { Title } = Typography;

interface ConversationItem {
  type: 'question' | 'answer';
  content: string;
  index?: number;
  timestamp: number;
}

export default function CampusInterviewStartPage() {
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
  const chatContainerRef = useRef<HTMLDivElement>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const answerInputRef = useRef<HTMLTextAreaElement>(null);
  const asrCapability = useASRCapability();
  const speechInput = useSpeechAnswerInput({
    enabled: asrCapability.enabled,
    sessionId,
    interviewType: 'comprehensive',
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
      ? '语音识别不可用'
      : speechInput.status === 'recording'
        ? '正在录音，点击停止按钮结束'
        : speechInput.status === 'stopping'
          ? '正在停止录音，请稍候...'
          : speechInput.status === 'transcribing'
            ? '正在转写，请稍候...'
            : speechInput.status === 'error'
              ? '语音识别失败，请重试'
              : '支持语音输入，结果将填入输入框';

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
    if (!params || !params.resume_id) {
      message.error('缺少面试参数或简历，请从表单页重新进入');
      return;
    }
    setStarting(true);
    const sanitize = (s: string) => s.replace(/[<>&"'`]/g, '');
    const requestBody = {
      type: String(params.type || 'comprehensive'),
      domain: String(params.domain || 'campus'),
      difficulty: String(params.difficulty || 'simple'),
      position_name: String(params.position_name || ''),
      company_name: sanitize(String(params.company_name || '')),
      resume_id: Number(params.resume_id),
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
          message.error(
            `Cannot connect to backend service. Please confirm it is running on ${apiUrl}`
          );
          setStarting(false);
          console.error('[检测] 后端服务连接失败:', e);
          return;
        }

        // 使用JSON格式发送请求，包含resume_id
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
              content: '接口返回 404，请检查路由配置',
              duration: 10,
            });
          } else {
            message.error(`Interview start failed: ${response.status} ${response.statusText}`);
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
                    // 使用循环查找最后一条问题，增强兼容性
                    let lastQuestionIdx = -1;
                    for (let i = prev.length - 1; i >= 0; i--) {
                      if (prev[i].type === 'question') {
                        lastQuestionIdx = i;
                        break;
                      }
                    }

                    // 索引一致且是 chunk 或 question 的追加/校准
                    if (lastQuestionIdx !== -1 && prev[lastQuestionIdx].index === idx) {
                      const updated = [...prev];
                      if (payload?.type === 'chunk') {
                        // 流式追加
                        updated[lastQuestionIdx] = {
                          ...updated[lastQuestionIdx],
                          content: updated[lastQuestionIdx].content + q,
                        };
                      } else {
                        // 完整覆盖（类型为 question 时）
                        updated[lastQuestionIdx] = {
                          ...updated[lastQuestionIdx],
                          content: q,
                        };
                      }
                      setQuestionText(updated[lastQuestionIdx].content);
                      setQuestionIndex(idx);
                      return updated;
                    } else {
                      // 新题
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
                  message.success('面试已完成，正在跳转到记录页...');
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
    const frameId = window.requestAnimationFrame(() => {
      messagesEndRef.current?.scrollIntoView({ block: 'end', behavior: 'smooth' });
    });
    return () => window.cancelAnimationFrame(frameId);
  }, [conversationHistory, waitingNextQuestion, starting]);

  // 对话框解锁时自动聚焦输入框
  useEffect(() => {
    if (!waitingNextQuestion && !starting && answerInputRef.current) {
      answerInputRef.current.focus();
    }
  }, [waitingNextQuestion, starting]);

  const mm = String(Math.floor(elapsed / 60)).padStart(2, '0');
  const ss = String(elapsed % 60).padStart(2, '0');
  // 总共10道题，根据当前题目序号计算进度
  const percent = Math.min(100, Math.round((questionIndex / 10) * 100));

  const onSubmit = async (act?: 'next' | 'quit') => {
    if (!sessionId) {
      message.warning('会话已过期，请重新开始面试');
      return;
    }

    const action = act || 'next';

    // 如果是End Interview
    if (action === 'quit') {
      try {
        const token = localStorage.getItem('token');
        if (token) {
          // 调用后端接口End Interview
          // 更新为新的End Interview接口
          await fetch(`${INTERVIEW_API.END_INTERVIEW}`, {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
              Authorization: `Bearer ${token}`,
            },
            body: JSON.stringify({
              session_id: sessionId,
              // End Interview可能不需要answer字段，仅传session_id即可
            }),
            mode: 'cors',
          });
        }
      } catch (e) {
        console.error('[End Interview] 请求失败:', e);
      }
      try {
        abortControllerRef.current?.abort();
      } catch {}
      message.success('已结束面试，正在跳转...');
      router.push('/user/interviews');
      return;
    }

    // 验证答案不为空
    if (!answer.trim()) {
      message.warning('请先输入答案再提交');
      return;
    }

    setSubmitting(true);

    // 添加答案到对话历史
    setConversationHistory((prev) => [
      ...prev,
      {
        type: 'answer',
        content: answer,
        timestamp: Date.now(),
      },
    ]);

    setAnsweredCount((prev) => prev + 1);
    const currentAnswer = answer;
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

      console.log('[提交答案] 调用submit/answer接口:', {
        session_id: sessionId,
        answer: currentAnswer,
      });

      // 调用submit/answer接口提交答案
      const headers: Record<string, string> = {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
      };

      // 更新为新的提交答案接口
      const response = await fetch(`${INTERVIEW_API.SUBMIT_ANSWER}`, {
        method: 'POST',
        headers,
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
      <div className="fixed top-0 right-0 w-[500px] h-[500px] bg-green-100/40 rounded-full blur-[100px] -translate-y-1/2 translate-x-1/3 pointer-events-none z-0" />
      <div className="fixed bottom-0 left-0 w-[500px] h-[500px] bg-blue-100/40 rounded-full blur-[100px] translate-y-1/2 -translate-x-1/3 pointer-events-none z-0" />

      {/* Header */}
      <header className="sticky top-0 z-50 bg-white/80 backdrop-blur-md border-b border-slate-200/60 shadow-sm transition-all duration-300">
        <div className="max-w-4xl mx-auto px-4 h-16 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-8 h-8 bg-green-100 rounded-lg flex items-center justify-center text-green-600">
              <CustomerServiceOutlined />
            </div>
            <div>
              <h1 className="text-base font-bold text-slate-800 m-0 leading-tight">
                综合面试
              </h1>
              <p className="text-xs text-slate-500 m-0">校招简历面试</p>
            </div>
          </div>

          <div className="flex items-center gap-4">
            <div className="hidden md:flex items-center gap-3 bg-slate-50 px-3 py-1.5 rounded-full border border-slate-100">
              <div className="flex items-center gap-1.5">
                <div className="w-2 h-2 rounded-full bg-green-500 animate-pulse" />
                <span className="text-xs font-medium text-slate-600 font-mono">
                  {mm}:{ss}
                </span>
              </div>
              <div className="w-px h-3 bg-slate-200" />
              <div className="flex items-center gap-1.5">
                <span className="text-xs text-slate-500">进度 {percent}%</span>
                <div className="w-16 h-1.5 bg-slate-200 rounded-full overflow-hidden">
                  <div
                    className="h-full bg-green-500 transition-all duration-500"
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
        <div className="max-w-4xl mx-auto px-4 py-8 space-y-8 pb-40">
          {conversationHistory.length === 0 && !starting && (
            <div className="flex flex-col items-center justify-center py-20 opacity-0 animate-fade-in-up">
              <div className="w-16 h-16 bg-green-50 rounded-2xl flex items-center justify-center text-green-500 text-2xl mb-4 animate-bounce-subtle">
                <CustomerServiceOutlined />
              </div>
              <p className="text-slate-400 text-sm">正在分析简历并生成问题...</p>
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
                      Interviewer · Q{questionNumber}
                    </span>
                  )}

                  <div
                    className={`
                      relative px-6 py-4 text-[15px] leading-relaxed shadow-sm
                      ${
                        isQuestion
                          ? 'bg-white text-slate-700 rounded-2xl rounded-tl-none border border-slate-100'
                          : 'bg-gradient-to-br from-green-500 to-emerald-600 text-white rounded-2xl rounded-tr-none shadow-green-200'
                      }
                    `}
                  >
                    <div className="whitespace-pre-wrap break-words">{item.content}</div>
                  </div>

                  {!isQuestion && (
                    <span className="text-xs text-slate-400 mt-1.5 mr-1">
                      Me ·{' '}
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
                  {starting ? '正在生成第一题...' : '正在思考下一题...'}
                </span>
              </div>
            </div>
          )}

          <div ref={messagesEndRef} aria-hidden="true" className="h-px scroll-mb-44" />
        </div>
      </main>

      {/* Input Area */}
      <footer className="sticky bottom-0 z-50 bg-white/80 backdrop-blur-xl border-t border-slate-200/60 pb-6 pt-4">
        <div className="max-w-4xl mx-auto px-4">
          <div className="relative bg-white rounded-2xl border border-slate-200 shadow-lg shadow-slate-100/50 transition-all focus-within:shadow-xl focus-within:border-green-400 focus-within:ring-1 focus-within:ring-green-100">
            <Input.TextArea
              ref={answerInputRef}
              value={answer}
              onChange={(e) => setAnswer(e.target.value)}
              placeholder={
                speechInput.status === 'recording'
                  ? '正在录音，点击停止按钮结束...'
                  : speechInput.status === 'stopping'
                    ? '正在停止录音，请稍候...'
                    : speechInput.status === 'transcribing'
                      ? '正在转写，请稍候...'
                      : waitingNextQuestion
                        ? '面试官正在出题...'
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

            <InterviewFooterActions
              isRecording={speechInput.isRecording}
              isStopping={speechInput.isStopping}
              speechDisabled={speechDisabled}
              asrLoading={asrCapability.loading}
              submitting={submitting}
              sendDisabled={
                !sessionId ||
                waitingNextQuestion ||
                starting ||
                speechInput.isRecording ||
                speechInput.isStopping ||
                speechInput.isTranscribing ||
                !answer.trim()
              }
              onMicClick={speechInput.handleMicClick}
              onSubmit={() => onSubmit()}
              sendButtonClassName="!bg-green-500 hover:!bg-green-600 !shadow-green-200 !border-0"
            />
          </div>
          <div className="mt-2 px-2">
            <p className="text-xs text-slate-400">{speechHint}</p>
          </div>
          <div className="text-center mt-2">
            <p className="text-xs text-slate-300">Mianshiba</p>
          </div>
        </div>
      </footer>
    </div>
  );
}
