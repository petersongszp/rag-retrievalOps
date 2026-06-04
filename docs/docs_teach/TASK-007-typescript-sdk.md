# TASK-007: TypeScript SDK 开发详细实现教程

&gt; 🎯 **任务 ID**: TASK-007
&gt;
&gt; **功能名称**: TypeScript SDK
&gt;
&gt; **预估工时**: 8 小时
&gt;
&gt; **难度**: ⭐⭐ (入门级)
&gt;
&gt; **技术栈**: TypeScript、fetch API、Vitest
&gt;
&gt; **推荐人数**: 1-2 人

---

## 📋 目录

- [一、需求是什么？](#一需求是什么)
- [二、为什么要做这个？](#二为什么要做这个)
- [三、技术原理](#三技术原理)
- [四、实现步骤](#四实现步骤)
- [五、如何验证？](#五如何验证)
- [六、代码提交流程](#六代码提交流程)

---

## 一、需求是什么？

### 1.1 问题背景

前端应用和 Node.js 后端需要对接 RAG 中台，缺少 TypeScript SDK 会增加开发成本。

### 1.2 解决方案

开发 TypeScript SDK，提供类型安全的 Promise 风格 API，支持浏览器和 Node.js 环境。

### 1.3 功能需求

| 功能点 | 说明 |
|--------|------|
| 客户端初始化 | 支持 BaseURL、API Key、超时配置 |
| 检索接口 | 封装 `/v1/retrieve` API |
| 类型安全 | TypeScript 类型定义 |
| 错误处理 | 统一的错误类 |
| 跨环境 | 支持浏览器和 Node.js |
| 零依赖 | 使用原生 fetch API |

---

## 二、为什么要做这个？

### 2.1 业务价值

| 指标 | 预期提升 |
|------|---------|
| 前端对接效率 | 从 2 小时降到 10 分钟 |
| TypeScript 开发者体验 | 显著提升 |

### 2.2 技术价值

- 学习 **TypeScript 库设计**
- 掌握 **fetch API 封装**
- 理解 **泛型和类型推断**
- 实践 **Vitest 测试**

---

## 三、技术原理

### 3.1 系统架构

```
┌─────────────────┐
│  前端 / Node.js │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  TypeScript SDK │ ← (1) 提供类型安全 API
│  - Client       │
│  - Types        │
│  - Error Handle │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  RAG 中台 API   │ ← (2) HTTP 调用
└─────────────────┘
```

### 3.2 目录结构

```
sdk/
└── typescript/
    ├── src/
    │   ├── index.ts
    │   ├── client.ts
    │   ├── types.ts
    │   └── errors.ts
    ├── examples/
    │   ├── browser.html
    │   └── node.ts
    ├── tests/
    │   └── client.test.ts
    ├── package.json
    ├── tsconfig.json
    └── README.md
```

---

## 四、实现步骤

### Step 0: 创建目录结构

```bash
mkdir -p sdk/typescript/src
mkdir -p sdk/typescript/examples
mkdir -p sdk/typescript/tests
```

### Step 1: 定义错误类

**文件**: `sdk/typescript/src/errors.ts`

```typescript
export class RAGApiError extends Error {
  statusCode: number;
  body: string;

  constructor(statusCode: number, body: string) {
    super(`RAG API returned ${statusCode}: ${body}`);
    this.name = 'RAGApiError';
    this.statusCode = statusCode;
    this.body = body;
  }
}
```

### Step 2: 定义类型

**文件**: `sdk/typescript/src/types.ts`

```typescript
export interface RetrieveRequest {
  query: string;
  kb_ids?: number[];
  top_k?: number;
  strategy_profile?: string;
  metadata_filter?: Record&lt;string, unknown&gt;;
}

export interface RetrieveItem {
  content: string;
  score: number;
  citation: unknown;
  source: unknown;
}

export interface RetrieveResponse {
  request_id: string;
  items: RetrieveItem[];
}

export interface ClientConfig {
  baseUrl: string;
  apiKey?: string;
  timeout?: number;
}
```

### Step 3: 实现客户端

**文件**: `sdk/typescript/src/client.ts`

```typescript
import { RAGApiError } from './errors';
import type { ClientConfig, RetrieveRequest, RetrieveResponse } from './types';

export class RAGClient {
  private baseUrl: string;
  private apiKey?: string;
  private timeout: number;

  constructor(config: ClientConfig) {
    this.baseUrl = config.baseUrl.replace(/\/$/, '');
    this.apiKey = config.apiKey;
    this.timeout = config.timeout ?? 10000;
  }

  private buildHeaders(): Record&lt;string, string&gt; {
    const headers: Record&lt;string, string&gt; = {
      'Content-Type': 'application/json',
    };
    if (this.apiKey) {
      headers['Authorization'] = `Bearer ${this.apiKey}`;
    }
    return headers;
  }

  async retrieve(request: RetrieveRequest): Promise&lt;RetrieveResponse&gt; {
    const controller = new AbortController();
    const timeoutId = setTimeout(() =&gt; controller.abort(), this.timeout);

    try {
      const response = await fetch(`${this.baseUrl}/v1/retrieve`, {
        method: 'POST',
        headers: this.buildHeaders(),
        body: JSON.stringify(request),
        signal: controller.signal,
      });

      if (!response.ok) {
        const body = await response.text();
        throw new RAGApiError(response.status, body);
      }

      return await response.json() as RetrieveResponse;
    } finally {
      clearTimeout(timeoutId);
    }
  }
}
```

### Step 4: 创建 index.ts

**文件**: `sdk/typescript/src/index.ts`

```typescript
export { RAGClient } from './client';
export { RAGApiError } from './errors';
export type {
  ClientConfig,
  RetrieveRequest,
  RetrieveResponse,
  RetrieveItem,
} from './types';
```

### Step 5: 配置 package.json

**文件**: `sdk/typescript/package.json`

```json
{
  "name": "rag-sdk",
  "version": "0.1.0",
  "description": "RAG Platform TypeScript SDK",
  "type": "module",
  "main": "./dist/index.cjs",
  "module": "./dist/index.js",
  "types": "./dist/index.d.ts",
  "exports": {
    ".": {
      "import": "./dist/index.js",
      "require": "./dist/index.cjs",
      "types": "./dist/index.d.ts"
    }
  },
  "files": [
    "dist"
  ],
  "scripts": {
    "build": "tsup src/index.ts --format cjs,esm --dts",
    "dev": "tsup src/index.ts --format cjs,esm --dts --watch",
    "test": "vitest",
    "test:run": "vitest run",
    "lint": "tsc --noEmit"
  },
  "devDependencies": {
    "@types/node": "^20.0.0",
    "tsup": "^8.0.0",
    "typescript": "^5.0.0",
    "vitest": "^1.0.0"
  }
}
```

### Step 6: 配置 tsconfig.json

**文件**: `sdk/typescript/tsconfig.json`

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "strict": true,
    "declaration": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
    "outDir": "./dist",
    "rootDir": "./src"
  },
  "include": ["src/**/*"],
  "exclude": ["node_modules", "dist", "tests", "examples"]
}
```

### Step 7: 编写使用示例

**文件**: `sdk/typescript/examples/node.ts`

```typescript
import { RAGClient, RAGApiError } from '../src';

async function main() {
  const client = new RAGClient({
    baseUrl: 'http://localhost:8081',
    apiKey: 'rag_xxxxxxxxxxxx',
  });

  try {
    const response = await client.retrieve({
      query: '知识库里关于 Go 并发的内容是什么？',
      kb_ids: [1],
      top_k: 5,
    });

    console.log('request_id:', response.request_id);
    response.items.forEach(item =&gt; {
      console.log(`[${item.score.toFixed(2)}] ${item.content}`);
    });
  } catch (error) {
    if (error instanceof RAGApiError) {
      console.error('Error:', error.message);
      console.error('Status code:', error.statusCode);
      console.error('Body:', error.body);
    } else {
      console.error('Unexpected error:', error);
    }
  }
}

main();
```

**文件**: `sdk/typescript/examples/browser.html`

```html
&lt;!DOCTYPE html&gt;
&lt;html lang="zh-CN"&gt;
&lt;head&gt;
  &lt;meta charset="UTF-8"&gt;
  &lt;meta name="viewport" content="width=device-width, initial-scale=1.0"&gt;
  &lt;title&gt;RAG SDK Browser Example&lt;/title&gt;
&lt;/head&gt;
&lt;body&gt;
  &lt;h1&gt;RAG SDK Browser Example&lt;/h1&gt;
  &lt;div id="result"&gt;&lt;/div&gt;

  &lt;script type="module"&gt;
    import { RAGClient, RAGApiError } from '../dist/index.js';

    const resultDiv = document.getElementById('result');

    async function run() {
      const client = new RAGClient({
        baseUrl: 'http://localhost:8081',
        apiKey: 'rag_xxxxxxxxxxxx',
      });

      try {
        const response = await client.retrieve({
          query: '知识库里关于 Go 并发的内容是什么？',
          kb_ids: [1],
          top_k: 5,
        });

        let html = `&lt;p&gt;request_id: ${response.request_id}&lt;/p&gt;`;
        response.items.forEach(item =&gt; {
          html += `&lt;p&gt;[${item.score.toFixed(2)}] ${item.content}&lt;/p&gt;`;
        });
        resultDiv.innerHTML = html;
      } catch (error) {
        if (error instanceof RAGApiError) {
          resultDiv.innerHTML = `
            &lt;p&gt;Error: ${error.message}&lt;/p&gt;
            &lt;p&gt;Status code: ${error.statusCode}&lt;/p&gt;
            &lt;p&gt;Body: ${error.body}&lt;/p&gt;
          `;
        } else {
          resultDiv.innerHTML = `&lt;p&gt;Unexpected error: ${error}&lt;/p&gt;`;
        }
      }
    }

    run();
  &lt;/script&gt;
&lt;/body&gt;
&lt;/html&gt;
```

### Step 8: 编写单元测试

**文件**: `sdk/typescript/tests/client.test.ts`

```typescript
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { RAGClient, RAGApiError } from '../src';

describe('RAGClient', () =&gt; {
  let client: RAGClient;

  beforeEach(() =&gt; {
    client = new RAGClient({
      baseUrl: 'http://test',
      apiKey: 'test-key',
    });
  });

  it('should retrieve successfully', async () =&gt; {
    const mockResponse = {
      request_id: 'test-123',
      items: [
        {
          content: 'test content',
          score: 0.95,
          citation: {},
          source: {},
        },
      ],
    };

    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () =&gt; Promise.resolve(mockResponse),
    });

    const response = await client.retrieve({
      query: 'test',
      kb_ids: [1],
    });

    expect(response.request_id).toBe('test-123');
    expect(response.items).toHaveLength(1);
    expect(response.items[0].content).toBe('test content');
  });

  it('should throw RAGApiError on error', async () =&gt; {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 401,
      text: () =&gt; Promise.resolve('Invalid API key'),
    });

    await expect(
      client.retrieve({ query: 'test', kb_ids: [1] })
    ).rejects.toThrow(RAGApiError);

    try {
      await client.retrieve({ query: 'test', kb_ids: [1] });
    } catch (error) {
      expect(error).toBeInstanceOf(RAGApiError);
      if (error instanceof RAGApiError) {
        expect(error.statusCode).toBe(401);
        expect(error.body).toContain('Invalid API key');
      }
    }
  });
});
```

### Step 9: 创建 README

**文件**: `sdk/typescript/README.md`

```markdown
# RAG Platform TypeScript SDK

## 安装

```bash
npm install rag-sdk
# or
yarn add rag-sdk
# or
pnpm add rag-sdk
```

## 快速开始

```typescript
import { RAGClient, RAGApiError } from 'rag-sdk';

const client = new RAGClient({
  baseUrl: 'http://localhost:8081',
  apiKey: 'rag_xxxxxxxxxxxx',
});

async function main() {
  try {
    const response = await client.retrieve({
      query: '知识库里关于 Go 并发的内容是什么？',
      kb_ids: [1],
      top_k: 5,
    });

    console.log('request_id:', response.request_id);
    response.items.forEach(item =&gt; {
      console.log(`[${item.score.toFixed(2)}] ${item.content}`);
    });
  } catch (error) {
    if (error instanceof RAGApiError) {
      console.error('Status:', error.statusCode);
      console.error('Body:', error.body);
    }
  }
}

main();
```

## 错误处理

```typescript
import { RAGApiError } from 'rag-sdk';

try {
  const response = await client.retrieve(request);
} catch (error) {
  if (error instanceof RAGApiError) {
    console.log('Status:', error.statusCode);
    console.log('Body:', error.body);
  }
}
```
```

---

## 五、如何验证？

### 5.1 单元测试

```bash
cd sdk/typescript
npm install
npm run test:run
```

### 5.2 手动测试

```bash
cd sdk/typescript
npm install
npm run build
npx tsx examples/node.ts
```

### 5.3 验收标准

| 验收项 | 标准 |
|--------|------|
| 单元测试 | 100% 通过 |
| 类型安全 | TypeScript 类型正确 |
| 异步 API | Promise 正常工作 |
| 错误处理 | 正确抛出 RAGApiError |
| 构建产物 | ESM 和 CJS 双格式 |

---

## 六、代码提交流程

### 6.1 提交代码

```bash
git checkout -b feature/TASK-007-typescript-sdk

git add sdk/typescript/

git commit -m "feat: TASK-007 实现 TypeScript SDK

- TypeScript 类型定义
- fetch API 封装
- 浏览器和 Node.js 支持
- 完整单元测试"

git push origin feature/TASK-007-typescript-sdk
```

### 6.2 创建 Pull Request

**标题**: `feat: TASK-007 实现 TypeScript SDK`

---

## 🎉 恭喜！

完成这个任务后，你将：
- ✅ 掌握 TypeScript 库设计
- ✅ 熟练使用 fetch API
- ✅ 理解 TypeScript 类型系统
- ✅ 学会 Vitest 测试框架

**下一步**: 去做 TASK-008（SDK 使用示例）！
