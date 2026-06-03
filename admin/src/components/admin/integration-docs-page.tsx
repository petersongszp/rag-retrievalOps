'use client';

import { Alert, Card, Space, Typography } from 'antd';

const { Paragraph, Text, Title } = Typography;

const loginCurl = `curl -X POST "$RAG_BASE_URL/v1/auth/login" \\
  -H "Content-Type: application/json" \\
  -d '{
    "email": "owner@example.com",
    "password": "your-password"
  }'`;

const createKeyCurl = `curl -X POST "$RAG_BASE_URL/v1/api-keys" \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer $RAG_JWT" \\
  -d '{
    "name": "production-agent",
    "app_id": "support-bot",
    "permissions": ["retrieve"],
    "expires_in": 0
  }'`;

const retrieveCurl = `curl -X POST "$RAG_BASE_URL/v1/retrieve" \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer $RAG_API_KEY" \\
  -d '{
    "query": "知识库里关于 Go 并发的内容是什么？",
    "kb_ids": [1],
    "top_k": 5
  }'`;

const pythonExample = `import os
import requests

resp = requests.post(
    f"{os.environ['RAG_BASE_URL']}/v1/retrieve",
    headers={
        "Content-Type": "application/json",
        "Authorization": f"Bearer {os.environ['RAG_API_KEY']}",
    },
    json={
        "query": "知识库里关于 Go 并发的内容是什么？",
        "kb_ids": [1],
        "top_k": 5,
    },
    timeout=30,
)

print(resp.status_code)
print(resp.text)`;

const goExample = `client := ragsdk.NewClient(ragsdk.ClientConfig{
  BaseURL: "http://localhost:8081",
  APIKey:  "rag_xxxxxxxxxxxx",
})

resp, err := client.Retrieve(ctx, ragsdk.RetrieveRequest{
  Query: "知识库里关于 Go 并发的内容是什么？",
  KBIDs: []uint64{1},
  TopK:  5,
})`;

function CodeBlock({ title, code }: { title: string; code: string }) {
  return (
    <Card size="small" title={title}>
      <pre className="overflow-x-auto whitespace-pre-wrap text-xs leading-6 text-slate-900">{code}</pre>
    </Card>
  );
}

export function IntegrationDocsPage() {
  return (
    <div className="space-y-6">
      <div>
        <Title level={2} style={{ marginBottom: 8 }}>
          接入文档
        </Title>
        <Paragraph type="secondary" style={{ marginBottom: 0 }}>
          这里给出从 JWT 登录、创建 API Key 到服务端检索的最小接入路径。完整文档同步维护在
          `backend/docs/zhuhu/agent-integration-guide.md`。
        </Paragraph>
      </div>

      <Alert
        type="info"
        showIcon
        message="认证边界"
        description="Admin UI 使用 JWT；Agent/SDK 使用 API Key。终端用户不直接持有 API Key。legacy app_id 仅保留兼容旧链路。"
      />

      <Space direction="vertical" size={16} className="w-full">
        <CodeBlock title="1. 登录获取 JWT" code={loginCurl} />
        <CodeBlock title="2. 创建 API Key" code={createKeyCurl} />
        <CodeBlock title="3. 使用 API Key 调 /v1/retrieve" code={retrieveCurl} />
        <CodeBlock title="Python requests 示例" code={pythonExample} />
        <CodeBlock title="Go SDK 示例" code={goExample} />
      </Space>

      <Card title="常见错误码">
        <Space direction="vertical" size={8} className="w-full">
          <Text>`401 invalid_api_key` / `api_key_revoked` / `api_key_expired`</Text>
          <Text>`403 forbidden` / `tenant_suspended`</Text>
          <Text>`404 not_found`</Text>
          <Text>`429 quota_exceeded` / `rate_limited`</Text>
        </Space>
      </Card>
    </div>
  );
}
