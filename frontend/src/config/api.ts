// API 配置文件
// 使用 NEXT_PUBLIC_API_BASE_URL 环境变量，默认为相对路径 /api
export const API_BASE_URL = process.env.NEXT_PUBLIC_API_BASE_URL || '/api';

// 面试相关接口
export const INTERVIEW_API = {
  // 启动面试流
  START_STREAM: `${API_BASE_URL}/interview/stream/start`,
  // 结束面试
  END_INTERVIEW: `${API_BASE_URL}/interview/interview/end`,
  // 提交答案
  SUBMIT_ANSWER: `${API_BASE_URL}/interview/answer/submit`,
  // 获取答题记录
  GET_ANSWER_RECORD: `${API_BASE_URL}/interview/answer-record`,
  // 获取评估报告
  GET_EVALUATION: `${API_BASE_URL}/interview/evaluation`,
  // 查询语音识别能力
  ASR_CAPABILITY: `${API_BASE_URL}/interview/asr/capability`,
  // 批量音频转写
  ASR_TRANSCRIBE: `${API_BASE_URL}/interview/asr/transcribe`,
  // axios client 路径
  ASR_CAPABILITY_PATH: '/interview/asr/capability',
  ASR_TRANSCRIBE_PATH: '/interview/asr/transcribe',
};

export const USER_API = {
  LOGIN: `${API_BASE_URL}/user/login`,
  REGISTER: `${API_BASE_URL}/user/register`,
  GET_PROFILE: `${API_BASE_URL}/user/profile`,
  SWITCH_MODEL: `${API_BASE_URL}/user/model/switch`,
  GITHUB_LOGIN: `${API_BASE_URL}/user/github/login`,
  GITHUB_CALLBACK: `${API_BASE_URL}/user/github/callback`,
};
