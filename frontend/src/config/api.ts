// API 配置文件
// 使用 NEXT_PUBLIC_API_BASE_URL 环境变量，默认为相对路径 /api
export const API_BASE_URL = process.env.NEXT_PUBLIC_API_BASE_URL || '/api';

// 用户模块
// export const USER_API = {
//   LOGIN: `${API_BASE_URL}/user/login`,
//   REGISTER: `${API_BASE_URL}/user/register`,
//   GET_PROFILE: `${API_BASE_URL}/user/profile`,
//   GITHUB_LOGIN: `${API_BASE_URL}/user/github/login`,
//   GITHUB_CALLBACK: `${API_BASE_URL}/user/github/callback`,
//   SWITCH_MODEL: `${API_BASE_URL}/user/model/switch`,
// };

export const USER_API = {
  LOGIN: `${API_BASE_URL}/user/login`,
  REGISTER: `${API_BASE_URL}/user/register`,
  GET_PROFILE: `${API_BASE_URL}/user/profile`,
  SWITCH_MODEL: `${API_BASE_URL}/user/model/switch`,
  GITHUB_LOGIN: `${API_BASE_URL}/user/github/login`,
  GITHUB_CALLBACK: `${API_BASE_URL}/user/github/callback`,
  GOOGLE_LOGIN: `${API_BASE_URL}/user/google/login`,
  GOOGLE_CALLBACK: `${API_BASE_URL}/user/google/callback`,
};

// 面试核心模块
export const INTERVIEW_API = {
  // 启动面试流
  START_STREAM: `${API_BASE_URL}/interview/stream/start`,
  // 结束面试
  END_INTERVIEW: `${API_BASE_URL}/interview/interview/end`,
  // 提交答案
  SUBMIT_ANSWER: `${API_BASE_URL}/interview/answer/submit`,
  // 获取历史记录
  GET_RECORDS: `${API_BASE_URL}/interview/records`,
  // 获取答题记录
  GET_ANSWER_RECORD: `${API_BASE_URL}/interview/answer-record`,
  // 获取评估报告
  GET_EVALUATION: `${API_BASE_URL}/interview/evaluation`,
  // ASR 相关
  ASR_CAPABILITY: `${API_BASE_URL}/interview/asr/capability`,
  ASR_TRANSCRIBE: `${API_BASE_URL}/interview/asr/transcribe`,
};

// 简历管理模块
export const RESUME_API = {
  LIST: `${API_BASE_URL}/resume/list`,
  UPLOAD: `${API_BASE_URL}/resume/upload`,
  DETAIL: (id: string | number) => `${API_BASE_URL}/resume/${id}`,
  DELETE: (id: string | number) => `${API_BASE_URL}/resume/${id}`,
  SET_DEFAULT: `${API_BASE_URL}/resume/set-default`,
};



// AI 模型配置模块
export const MODEL_API = {
  CHECK: `${API_BASE_URL}/user/model/check`,
  LIST: `${API_BASE_URL}/user/model/list`,
  DETAILS: (id: string | number) => `${API_BASE_URL}/user/model/details/${id}`,
  CREATE: `${API_BASE_URL}/user/create/model`,
  UPDATE: (id: string | number) => `${API_BASE_URL}/user/model/update/${id}`,
  DELETE: (id: string | number) => `${API_BASE_URL}/user/model/delete/${id}`,
};

// 其他功能 (预测/押题)
export const PREDICTION_API = {
  LIST: `${API_BASE_URL}/prediction/list`,
  DETAIL: (id: string | number) => `${API_BASE_URL}/prediction/${id}`,
  START: `${API_BASE_URL}/prediction/start`,
  DELETE: `${API_BASE_URL}/prediction/delete`,
};

