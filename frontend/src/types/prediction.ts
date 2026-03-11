export interface PredictionRecordItem {
  id: number;
  created_at: string;
  job_title: string;
  difficulty: string;
  company: string;
  prediction_type: string;
  language: string;
}

export interface ListPredictionResponse {
  list: PredictionRecordItem[];
  total: number;
  page: number;
  size: number;
}

export interface PredictionQuestion {
  id: number;
  question: string;
  content: string;
  focus: string;
  thinking_path: string;
  reference_answer: string;
  follow_up: string; // JSON string representation of string[]
  sort: number;
}

export interface GetPredictionDetailResponse {
  id: number;
  questions: PredictionQuestion[];
}
