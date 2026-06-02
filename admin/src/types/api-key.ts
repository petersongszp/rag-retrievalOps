export interface APIKeyRecord {
  id: number;
  name: string;
  app_id: string;
  key_prefix: string;
  permissions: string[];
  status: string;
  last_used_at: string;
  expires_at: string;
  created_at: string;
}

export interface CreateAPIKeyPayload {
  name: string;
  app_id: string;
  permissions: string[];
  expires_in?: number;
}

export interface CreateAPIKeyResponse extends APIKeyRecord {
  key: string;
}

export interface RotateAPIKeyResponse {
  id: number;
  key: string;
  key_prefix: string;
  created_at: string;
}
