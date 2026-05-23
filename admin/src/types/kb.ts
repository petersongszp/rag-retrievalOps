// 知识库类型定义
export interface KnowledgeBase {
  id: number;
  name: string;
  description?: string;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface KBDocument {
  id: number;
  kb_id: number;
  file_name: string;
  file_type: string;
  file_size: number;
  file_hash: string;
  storage_path: string;
  status: 'pending' | 'processing' | 'completed' | 'failed';
  chunk_count: number;
  error_msg?: string;
  deleted: number;
  created_at: string;
  updated_at: string;
}

export interface KBIngestJob {
  id: number;
  kb_id: number;
  document_id: number;
  status: 'pending' | 'processing' | 'completed' | 'failed' | 'retrying' | 'dead' | 'canceled';
  retry_count: number;
  error_msg?: string;
  started_at?: string;
  finished_at?: string;
  created_at: string;
  updated_at: string;
}

export interface RetrieveItem {
  content: string;
  score: number;
  citation: {
    kb_id: number;
    document_id: number;
    chunk_id: string;
    file_name: string;
    chunk_index: number;
  };
  source: {
    route: string;
    collection: string;
    retriever_version: string;
  };
}

export interface RetrieveResponse {
  request_id: string;
  items: RetrieveItem[];
}

export interface ListResponse<T> {
  items: T[];
  total: number;
  page: number;
  page_size: number;
}
