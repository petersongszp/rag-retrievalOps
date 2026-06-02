export interface TenantSummary {
  tenant_id: number;
  name: string;
  slug?: string;
  plan?: string;
  status?: string;
}

export interface TenantDetail extends TenantSummary {
  created_at?: string;
  updated_at?: string;
  max_kb_count: number;
  max_doc_count: number;
  max_storage_mb: number;
  max_api_calls_per_day: number;
}

export interface TenantUsageLimits {
  max_kb_count: number;
  max_doc_count: number;
  max_storage_mb: number;
  max_api_calls_per_day: number;
}

export interface TenantUsage {
  api_calls_today: number;
  api_calls_this_month?: number;
  kb_count: number;
  doc_count: number;
  storage_mb: number;
  limits: TenantUsageLimits;
}
