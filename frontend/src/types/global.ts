// 全局类型定义

// API响应通用类型
export interface ApiResponse<T = any> {
  code: number;
  message: string;
  data: T;
}

// 分页参数
export interface PaginationParams {
  page: number;
  pageSize: number;
}

// 分页响应
export interface PaginationResponse<T> {
  list: T[];
  total: number;
  page: number;
  pageSize: number;
  hasMore: boolean;
}

// 用户相关类型
export interface User {
  id: string;
  name: string;
  email: string;
  phone?: string;
  avatar?: string;
  createdAt: string;
}

// 面试相关类型
export interface Interview {
  id: string;
  title: string;
  type: 'comprehensive' | 'resume' | 'specialized';
  duration: number;
  status: 'pending' | 'completed' | 'cancelled';
  createdAt: string;
  completedAt?: string;
}

// 简历相关类型
export interface Resume {
  id: string;
  name: string;
  content?: string;
  fileUrl?: string;
  createdAt: string;
  updatedAt: string;
}
