# TASK-008: 多语言 SDK 使用示例与集成指南

&gt; 🎯 **任务 ID**: TASK-008
&gt;
&gt; **功能名称**: SDK 集成示例
&gt;
&gt; **预估工时**: 6 小时
&gt;
&gt; **难度**: ⭐ (入门级)
&gt;
&gt; **推荐人数**: 1 人

---

## 📋 目录

- [一、需求是什么？](#一需求是什么)
- [二、实现内容](#二实现内容)
- [三、目录结构](#三目录结构)
- [四、实现步骤](#四实现步骤)
- [五、验收标准](#五验收标准)
- [六、代码提交流程](#六代码提交流程)

---

## 一、需求是什么？

为了让开发者更方便地使用多语言 SDK，需要提供完整的集成示例，包括：
- 与主流 Agent 框架的集成
- 常见使用场景示例
- 最佳实践文档

---

## 二、实现内容

### 2.1 示例列表

| 示例 | 语言 | 说明 |
|------|------|------|
| LangChain 集成 | Python | LangChain RAG 应用 |
| AutoGen 集成 | Python | 多 Agent 协作 |
| Spring Boot 集成 | Java | 企业级应用 |
| React 组件 | TypeScript | 前端问答组件 |
| Next.js API Route | TypeScript | 服务端 API |

---

## 三、目录结构

```
sdk/
└── examples/
    ├── python/
    │   ├── langchain_example/
    │   │   ├── main.py
    │   │   ├── requirements.txt
    │   │   └── README.md
    │   └── autogen_example/
    │       ├── main.py
    │       ├── requirements.txt
    │       └── README.md
    ├── java/
    │   └── spring-boot-example/
    │       ├── pom.xml
    │       └── src/
    │           └── main/
    │               └── java/
    │                   └── com/
    │                       └── example/
    │                           └── Application.java
    └── typescript/
        ├── react-example/
        │   ├── App.tsx
        │   └── package.json
        └── nextjs-example/
            ├── app/
            │   └── api/
            │       └── rag/
            │           └── route.ts
            └── package.json
```

---

## 四、实现步骤

### Step 1: Python + LangChain 示例

**文件**: `sdk/examples/python/langchain_example/main.py`

```python
import os
from typing import List
from langchain.llms import OpenAI
from langchain.chains import RetrievalQA
from langchain.schema import Document
from rag_sdk import Client, RetrieveRequest


class RAGRetriever:
    def __init__(self, base_url: str, api_key: str, kb_ids: List[int]):
        self.client = Client(base_url=base_url, api_key=api_key)
        self.kb_ids = kb_ids

    def get_relevant_documents(self, query: str) -&gt; List[Document]:
        request = RetrieveRequest(
            query=query,
            kb_ids=self.kb_ids,
            top_k=5,
        )
        response = self.client.retrieve(request)

        return [
            Document(
                page_content=item.content,
                metadata={"score": item.score, "source": item.source}
            )
            for item in response.items
        ]


def main():
    retriever = RAGRetriever(
        base_url=os.getenv("RAG_BASE_URL", "http://localhost:8081"),
        api_key=os.getenv("RAG_API_KEY", ""),
        kb_ids=[1],
    )

    llm = OpenAI(temperature=0)

    qa_chain = RetrievalQA.from_chain_type(
        llm=llm,
        chain_type="stuff",
        retriever=retriever,
        return_source_documents=True,
    )

    query = "知识库里关于 Go 并发的内容是什么？"
    result = qa_chain({"query": query})

    print("Answer:", result["result"])
    print("\nSources:")
    for doc in result["source_documents"]:
        print(f"- [{doc.metadata['score']:.2f}] {doc.page_content[:100]}...")


if __name__ == "__main__":
    main()
```

**文件**: `sdk/examples/python/langchain_example/requirements.txt`

```
rag-sdk
langchain
openai
python-dotenv
```

**文件**: `sdk/examples/python/langchain_example/README.md`

```markdown
# LangChain + RAG SDK 示例

## 安装

```bash
pip install -r requirements.txt
```

## 运行

```bash
export RAG_BASE_URL=http://localhost:8081
export RAG_API_KEY=rag_xxxxxxxxxxxx
export OPENAI_API_KEY=sk-xxxxxxxxxxxx

python main.py
```
```

### Step 2: Python + AutoGen 示例

**文件**: `sdk/examples/python/autogen_example/main.py`

```python
import os
import autogen
from rag_sdk import Client, RetrieveRequest


class RAGAssistant:
    def __init__(self, base_url: str, api_key: str, kb_ids: list[int]):
        self.client = Client(base_url=base_url, api_key=api_key)
        self.kb_ids = kb_ids

    def retrieve(self, query: str) -&gt; str:
        request = RetrieveRequest(query=query, kb_ids=self.kb_ids, top_k=5)
        response = self.client.retrieve(request)

        context = "\n".join([
            f"[{item.score:.2f}] {item.content}"
            for item in response.items
        ])
        return f"Context:\n{context}"


def main():
    rag = RAGAssistant(
        base_url=os.getenv("RAG_BASE_URL", "http://localhost:8081"),
        api_key=os.getenv("RAG_API_KEY", ""),
        kb_ids=[1],
    )

    assistant = autogen.AssistantAgent(
        name="assistant",
        llm_config={"config_list": [{"model": "gpt-4", "api_key": os.getenv("OPENAI_API_KEY")}]},
    )

    user_proxy = autogen.UserProxyAgent(
        name="user_proxy",
        human_input_mode="NEVER",
        max_consecutive_auto_reply=1,
        code_execution_config={"work_dir": "coding"},
    )

    @user_proxy.register_for_execution()
    @assistant.register_for_llm(name="retrieve_from_kb", description="Retrieve information from knowledge base")
    def retrieve_from_kb(query: str) -&gt; str:
        return rag.retrieve(query)

    user_proxy.initiate_chat(
        assistant,
        message="请查询知识库中关于 Go 并发的内容，然后总结给我。",
    )


if __name__ == "__main__":
    main()
```

**文件**: `sdk/examples/python/autogen_example/requirements.txt`

```
rag-sdk
pyautogen
python-dotenv
```

### Step 3: Java + Spring Boot 示例

**文件**: `sdk/examples/java/spring-boot-example/pom.xml`

```xml
&lt;?xml version="1.0" encoding="UTF-8"?&gt;
&lt;project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd"&gt;
    &lt;modelVersion&gt;4.0.0&lt;/modelVersion&gt;

    &lt;parent&gt;
        &lt;groupId&gt;org.springframework.boot&lt;/groupId&gt;
        &lt;artifactId&gt;spring-boot-starter-parent&lt;/artifactId&gt;
        &lt;version&gt;3.2.0&lt;/version&gt;
    &lt;/parent&gt;

    &lt;groupId&gt;com.example&lt;/groupId&gt;
    &lt;artifactId&gt;rag-spring-boot-example&lt;/artifactId&gt;
    &lt;version&gt;1.0.0&lt;/version&gt;

    &lt;dependencies&gt;
        &lt;dependency&gt;
            &lt;groupId&gt;org.springframework.boot&lt;/groupId&gt;
            &lt;artifactId&gt;spring-boot-starter-web&lt;/artifactId&gt;
        &lt;/dependency&gt;
        &lt;dependency&gt;
            &lt;groupId&gt;com.rag&lt;/groupId&gt;
            &lt;artifactId&gt;rag-sdk&lt;/artifactId&gt;
            &lt;version&gt;0.1.0&lt;/version&gt;
        &lt;/dependency&gt;
    &lt;/dependencies&gt;
&lt;/project&gt;
```

**文件**: `sdk/examples/java/spring-boot-example/src/main/java/com/example/Application.java`

```java
package com.example;

import com.rag.sdk.RagClient;
import com.rag.sdk.model.RetrieveRequest;
import com.rag.sdk.model.RetrieveResponse;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.context.annotation.Bean;
import org.springframework.web.bind.annotation.*;

import java.util.Arrays;
import java.util.List;

@SpringBootApplication
@RestController
@RequestMapping("/api")
public class Application {

    @Value("${rag.base-url}")
    private String baseUrl;

    @Value("${rag.api-key}")
    private String apiKey;

    public static void main(String[] args) {
        SpringApplication.run(Application.class, args);
    }

    @Bean
    public RagClient ragClient() {
        return RagClient.builder()
                .baseUrl(baseUrl)
                .apiKey(apiKey)
                .build();
    }

    @PostMapping("/query")
    public QueryResponse query(@RequestBody QueryRequest request) {
        RetrieveRequest ragRequest = RetrieveRequest.builder()
                .query(request.query())
                .kbIds(Arrays.asList(1L))
                .topK(request.topK())
                .build();

        RetrieveResponse ragResponse = ragClient().retrieve(ragRequest);

        List&lt;QueryResultItem&gt; items = ragResponse.getItems().stream()
                .map(item -&gt; new QueryResultItem(item.getContent(), item.getScore()))
                .toList();

        return new QueryResponse(ragResponse.getRequestId(), items);
    }

    record QueryRequest(String query, Integer topK) {}
    record QueryResultItem(String content, double score) {}
    record QueryResponse(String requestId, List&lt;QueryResultItem&gt; items) {}
}
```

**文件**: `sdk/examples/java/spring-boot-example/src/main/resources/application.properties`

```properties
rag.base-url=http://localhost:8081
rag.api-key=rag_xxxxxxxxxxxx
server.port=8080
```

### Step 4: TypeScript + React 示例

**文件**: `sdk/examples/typescript/react-example/package.json`

```json
{
  "name": "rag-react-example",
  "private": true,
  "dependencies": {
    "react": "^18.2.0",
    "react-dom": "^18.2.0",
    "rag-sdk": "^0.1.0"
  },
  "devDependencies": {
    "@types/react": "^18.2.0",
    "@types/react-dom": "^18.2.0",
    "@vitejs/plugin-react": "^4.2.0",
    "typescript": "^5.3.0",
    "vite": "^5.0.0"
  }
}
```

**文件**: `sdk/examples/typescript/react-example/src/App.tsx`

```tsx
import { useState } from 'react';
import { RAGClient, RAGApiError } from 'rag-sdk';

const client = new RAGClient({
  baseUrl: import.meta.env.VITE_RAG_BASE_URL || 'http://localhost:8081',
  apiKey: import.meta.env.VITE_RAG_API_KEY,
});

export default function App() {
  const [query, setQuery] = useState('');
  const [loading, setLoading] = useState(false);
  const [results, setResults] = useState&lt;Array&lt;{ content: string; score: number }&gt;&gt;([]);
  const [error, setError] = useState&lt;string | null&gt;(null);

  const handleSearch = async () =&gt; {
    if (!query) return;

    setLoading(true);
    setError(null);

    try {
      const response = await client.retrieve({
        query,
        kb_ids: [1],
        top_k: 5,
      });

      setResults(response.items.map(item =&gt; ({
        content: item.content,
        score: item.score,
      })));
    } catch (err) {
      if (err instanceof RAGApiError) {
        setError(`Error: ${err.statusCode} - ${err.body}`);
      } else {
        setError('An unexpected error occurred');
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    &lt;div style={{ padding: '2rem', maxWidth: '800px', margin: '0 auto' }}&gt;
      &lt;h1&gt;RAG Search&lt;/h1&gt;

      &lt;div style={{ marginBottom: '2rem' }}&gt;
        &lt;input
          type="text"
          value={query}
          onChange={(e) =&gt; setQuery(e.target.value)}
          placeholder="Enter your query..."
          style={{ width: '100%', padding: '0.5rem', fontSize: '1rem' }}
          onKeyPress={(e) =&gt; e.key === 'Enter' &amp;&amp; handleSearch()}
        /&gt;
        &lt;button
          onClick={handleSearch}
          disabled={loading}
          style={{ marginTop: '0.5rem', padding: '0.5rem 1rem' }}
        &gt;
          {loading ? 'Searching...' : 'Search'}
        &lt;/button&gt;
      &lt;/div&gt;

      {error &amp;&amp; (
        &lt;div style={{ color: 'red', marginBottom: '1rem' }}&gt;
          {error}
        &lt;/div&gt;
      )}

      &lt;div&gt;
        {results.map((result, index) =&gt; (
          &lt;div key={index} style={{ marginBottom: '1rem', padding: '1rem', border: '1px solid #ddd' }}&gt;
            &lt;div style={{ fontSize: '0.875rem', color: '#666' }}&gt;
              Score: {result.score.toFixed(2)}
            &lt;/div&gt;
            &lt;div&gt;{result.content}&lt;/div&gt;
          &lt;/div&gt;
        ))}
      &lt;/div&gt;
    &lt;/div&gt;
  );
}
```

### Step 5: TypeScript + Next.js 示例

**文件**: `sdk/examples/typescript/nextjs-example/package.json`

```json
{
  "name": "rag-nextjs-example",
  "private": true,
  "dependencies": {
    "next": "14.0.0",
    "react": "^18.2.0",
    "react-dom": "^18.2.0",
    "rag-sdk": "^0.1.0"
  },
  "devDependencies": {
    "@types/node": "^20.0.0",
    "@types/react": "^18.2.0",
    "@types/react-dom": "^18.2.0",
    "typescript": "^5.3.0"
  }
}
```

**文件**: `sdk/examples/typescript/nextjs-example/app/api/rag/route.ts`

```typescript
import { NextResponse } from 'next/server';
import { RAGClient, RAGApiError } from 'rag-sdk';

const client = new RAGClient({
  baseUrl: process.env.RAG_BASE_URL || 'http://localhost:8081',
  apiKey: process.env.RAG_API_KEY,
});

export async function POST(request: Request) {
  try {
    const { query } = await request.json();

    const response = await client.retrieve({
      query,
      kb_ids: [1],
      top_k: 5,
    });

    return NextResponse.json(response);
  } catch (error) {
    if (error instanceof RAGApiError) {
      return NextResponse.json(
        { error: error.message, statusCode: error.statusCode },
        { status: error.statusCode }
      );
    }
    return NextResponse.json(
      { error: 'Internal server error' },
      { status: 500 }
    );
  }
}
```

**文件**: `sdk/examples/typescript/nextjs-example/app/page.tsx`

```tsx
'use client';

import { useState } from 'react';

export default function Home() {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState&lt;any[]&gt;([]);

  const handleSearch = async () =&gt; {
    const res = await fetch('/api/rag', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ query }),
    });
    const data = await res.json();
    setResults(data.items);
  };

  return (
    &lt;div style={{ padding: '2rem' }}&gt;
      &lt;h1&gt;RAG Search&lt;/h1&gt;
      &lt;input
        value={query}
        onChange={(e) =&gt; setQuery(e.target.value)}
        placeholder="Query..."
      /&gt;
      &lt;button onClick={handleSearch}&gt;Search&lt;/button&gt;
      &lt;ul&gt;
        {results.map((item, i) =&gt; (
          &lt;li key={i}&gt;[{item.score.toFixed(2)}] {item.content}&lt;/li&gt;
        ))}
      &lt;/ul&gt;
    &lt;/div&gt;
  );
}
```

### Step 6: 创建主 README

**文件**: `sdk/examples/README.md`

```markdown
# RAG SDK 集成示例

这是 RAG 中台多语言 SDK 的集成示例集合。

## 示例列表

- [Python + LangChain](./python/langchain_example/)
- [Python + AutoGen](./python/autogen_example/)
- [Java + Spring Boot](./java/spring-boot-example/)
- [TypeScript + React](./typescript/react-example/)
- [TypeScript + Next.js](./typescript/nextjs-example/)

## 前置条件

1. 启动 RAG 中台服务
2. 获取 API Key

## 快速开始

选择你感兴趣的示例，进入对应目录查看详细说明。
```

---

## 五、验收标准

| 验收项 | 标准 |
|--------|------|
| 所有示例 | 可正常运行 |
| 文档完整 | 每个示例都有 README |
| 代码规范 | 符合各语言最佳实践 |

---

## 六、代码提交流程

```bash
git checkout -b feature/TASK-008-sdk-examples

git add sdk/examples/

git commit -m "feat: TASK-008 添加 SDK 集成示例

- LangChain 集成示例
- AutoGen 集成示例
- Spring Boot 集成示例
- React 组件示例
- Next.js API Route 示例"

git push origin feature/TASK-008-sdk-examples
```

---

## 🎉 恭喜！

完成这个任务后，你将：
- ✅ 了解主流 Agent 框架
- ✅ 掌握 SDK 最佳实践
- ✅ 能够帮助其他开发者快速上手
