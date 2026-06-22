# TASK-005: Python SDK 开发详细实现教程

> 🎯 **任务 ID**: TASK-005
>
> **功能名称**: Python SDK
>
> **预估工时**: 8 小时
>
> **难度**: ⭐⭐ (入门级)
>
> **技术栈**: Python、httpx、pydantic
>
> **推荐人数**: 1-2 人

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

RAG 中台已经有 Go SDK，但上游 Agent 可能使用 Python 开发（LangChain、AutoGen 等主流 Agent 框架都是 Python 生态）。缺少 Python SDK 会增加对接成本。

### 1.2 解决方案

开发 Python SDK，提供简洁易用的 API，让 Python 开发者可以快速对接 RAG 中台。

### 1.3 功能需求

| 功能点 | 说明 |
|--------|------|
| 客户端初始化 | 支持 BaseURL、API Key、超时配置 |
| 检索接口 | 封装 `/v1/retrieve` API |
| 类型安全 | 使用 Pydantic 定义请求/响应模型 |
| 错误处理 | 统一的异常类型，包含状态码和响应体 |
| 异步支持 | 同时提供同步和异步 API |
| 重试机制 | 可选的自动重试 |

---

## 二、为什么要做这个？

### 2.1 业务价值

| 指标 | 预期提升 |
|------|---------|
| Python 用户对接效率 | 从 2 小时降到 10 分钟 |
| 错误率 | 降低 60%（类型安全避免拼写错误） |

### 2.2 技术价值

- 学习 **SDK 设计模式**
- 掌握 **Pydantic 类型系统**
- 理解 **HTTP 客户端封装**
- 实践 **同步/异步 API 设计**

---

## 三、技术原理

### 3.1 系统架构

```
┌─────────────────┐
│  Python Agent   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Python SDK     │ ← (1) 提供简洁 API
│  - Client       │
│  - Pydantic     │
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
└── python/
    ├── rag_sdk/
    │   ├── __init__.py
    │   ├── client.py          # 客户端实现
    │   ├── models.py          # Pydantic 模型
    │   └── exceptions.py      # 异常定义
    ├── examples/
    │   ├── basic_usage.py     # 基础使用示例
    │   └── async_usage.py     # 异步使用示例
    ├── tests/
    │   └── test_client.py     # 单元测试
    ├── pyproject.toml         # 项目配置
    └── README.md
```

---

## 四、实现步骤

### Step 0: 创建目录结构

```bash
# 在项目根目录下创建
mkdir -p sdk/python/rag_sdk
mkdir -p sdk/python/examples
mkdir -p sdk/python/tests
```

### Step 1: 定义异常类

**文件**: `sdk/python/rag_sdk/exceptions.py`

```python
class RAGAPIError(Exception):
    """RAG API 错误"""

    def __init__(self, status_code: int, body: str):
        self.status_code = status_code
        self.body = body
        super().__init__(f"RAG API returned {status_code}: {body}")
```

### Step 2: 定义 Pydantic 模型

**文件**: `sdk/python/rag_sdk/models.py`

```python
from typing import List, Optional, Dict, Any
from pydantic import BaseModel, Field


class RetrieveRequest(BaseModel):
    query: str = Field(..., description="查询文本")
    kb_ids: Optional[List[int]] = Field(None, description="知识库 ID 列表")
    top_k: Optional[int] = Field(None, description="返回结果数量")
    strategy_profile: Optional[str] = Field(None, description="策略配置")
    metadata_filter: Optional[Dict[str, Any]] = Field(None, description="元数据过滤")


class RetrieveItem(BaseModel):
    content: str = Field(..., description="内容")
    score: float = Field(..., description="相似度分数")
    citation: Any = Field(..., description="引用信息")
    source: Any = Field(..., description="来源信息")


class RetrieveResponse(BaseModel):
    request_id: str = Field(..., description="请求 ID")
    items: List[RetrieveItem] = Field(..., description="检索结果列表")
```

### Step 3: 实现客户端

**文件**: `sdk/python/rag_sdk/client.py`

```python
import httpx
from typing import Optional, Dict, Any
from .models import RetrieveRequest, RetrieveResponse
from .exceptions import RAGAPIError


class Client:
    def __init__(
        self,
        base_url: str,
        api_key: Optional[str] = None,
        timeout: float = 10.0,
    ):
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key
        self.timeout = timeout
        self._sync_client: Optional[httpx.Client] = None
        self._async_client: Optional[httpx.AsyncClient] = None

    def _get_headers(self) -> Dict[str, str]:
        headers = {"Content-Type": "application/json"}
        if self.api_key:
            headers["Authorization"] = f"Bearer {self.api_key}"
        return headers

    def retrieve(self, request: RetrieveRequest) -> RetrieveResponse:
        """同步检索"""
        if self._sync_client is None:
            self._sync_client = httpx.Client(timeout=self.timeout)

        url = f"{self.base_url}/v1/retrieve"
        response = self._sync_client.post(
            url,
            json=request.model_dump(exclude_none=True),
            headers=self._get_headers(),
        )

        if response.status_code != 200:
            raise RAGAPIError(response.status_code, response.text)

        return RetrieveResponse.model_validate(response.json())

    async def aretrieve(self, request: RetrieveRequest) -> RetrieveResponse:
        """异步检索"""
        if self._async_client is None:
            self._async_client = httpx.AsyncClient(timeout=self.timeout)

        url = f"{self.base_url}/v1/retrieve"
        response = await self._async_client.post(
            url,
            json=request.model_dump(exclude_none=True),
            headers=self._get_headers(),
        )

        if response.status_code != 200:
            raise RAGAPIError(response.status_code, response.text)

        return RetrieveResponse.model_validate(response.json())

    def close(self):
        """关闭同步客户端"""
        if self._sync_client:
            self._sync_client.close()
            self._sync_client = None

    async def aclose(self):
        """关闭异步客户端"""
        if self._async_client:
            await self._async_client.aclose()
            self._async_client = None

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.close()

    async def __aenter__(self):
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        await self.aclose()
```

### Step 4: 创建 `__init__.py`

**文件**: `sdk/python/rag_sdk/__init__.py`

```python
from .client import Client
from .models import RetrieveRequest, RetrieveResponse, RetrieveItem
from .exceptions import RAGAPIError

__all__ = [
    "Client",
    "RetrieveRequest",
    "RetrieveResponse",
    "RetrieveItem",
    "RAGAPIError",
]
```

### Step 5: 配置 pyproject.toml

**文件**: `sdk/python/pyproject.toml`

```toml
[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"

[project]
name = "rag-sdk"
version = "0.1.0"
description = "RAG Platform Python SDK"
requires-python = "&gt;=3.9"
authors = [
    { name = "Your Team" },
]
dependencies = [
    "httpx&gt;=0.24.0",
    "pydantic&gt;=2.0.0",
]

[project.optional-dependencies]
dev = [
    "pytest&gt;=7.0.0",
    "pytest-asyncio&gt;=0.21.0",
    "black&gt;=23.0.0",
]
```

### Step 6: 编写使用示例

**文件**: `sdk/python/examples/basic_usage.py`

```python
from rag_sdk import Client, RetrieveRequest, RAGAPIError


def main():
    client = Client(
        base_url="http://localhost:8081",
        api_key="rag_xxxxxxxxxxxx",
    )

    try:
        request = RetrieveRequest(
            query="知识库里关于 Go 并发的内容是什么？",
            kb_ids=[1],
            top_k=5,
        )

        response = client.retrieve(request)

        print(f"request_id: {response.request_id}")
        for item in response.items:
            print(f"[{item.score:.2f}] {item.content}")

    except RAGAPIError as e:
        print(f"Error: {e}")
        print(f"Status code: {e.status_code}")
        print(f"Body: {e.body}")
    finally:
        client.close()


if __name__ == "__main__":
    main()
```

**文件**: `sdk/python/examples/async_usage.py`

```python
import asyncio
from rag_sdk import Client, RetrieveRequest


async def main():
    async with Client(
        base_url="http://localhost:8081",
        api_key="rag_xxxxxxxxxxxx",
    ) as client:
        request = RetrieveRequest(
            query="知识库里关于 Go 并发的内容是什么？",
            kb_ids=[1],
            top_k=5,
        )

        response = await client.aretrieve(request)

        print(f"request_id: {response.request_id}")
        for item in response.items:
            print(f"[{item.score:.2f}] {item.content}")


if __name__ == "__main__":
    asyncio.run(main())
```

### Step 7: 编写单元测试

**文件**: `sdk/python/tests/test_client.py`

```python
import pytest
from unittest.mock import Mock, patch
from rag_sdk import Client, RetrieveRequest, RetrieveResponse, RAGAPIError


def test_retrieve_success():
    client = Client(base_url="http://test", api_key="test_key")

    mock_response = Mock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "request_id": "test-123",
        "items": [
            {"content": "test content", "score": 0.95, "citation": {}, "source": {}}
        ],
    }

    with patch.object(client._sync_client, "post", return_value=mock_response):
        request = RetrieveRequest(query="test", kb_ids=[1])
        response = client.retrieve(request)

        assert response.request_id == "test-123"
        assert len(response.items) == 1
        assert response.items[0].content == "test content"


def test_retrieve_error():
    client = Client(base_url="http://test", api_key="test_key")

    mock_response = Mock()
    mock_response.status_code = 401
    mock_response.text = "Invalid API key"

    with patch.object(client._sync_client, "post", return_value=mock_response):
        request = RetrieveRequest(query="test", kb_ids=[1])

        with pytest.raises(RAGAPIError) as exc_info:
            client.retrieve(request)

        assert exc_info.value.status_code == 401
        assert "Invalid API key" in exc_info.value.body


@pytest.mark.asyncio
async def test_async_retrieve_success():
    client = Client(base_url="http://test", api_key="test_key")

    mock_response = Mock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "request_id": "test-123",
        "items": [],
    }

    with patch.object(client._async_client, "post", return_value=mock_response):
        request = RetrieveRequest(query="test", kb_ids=[1])
        response = await client.aretrieve(request)

        assert response.request_id == "test-123"
```

### Step 8: 创建 README

**文件**: `sdk/python/README.md`

```markdown
# RAG Platform Python SDK

## 安装

```bash
pip install rag-sdk
```

或从源码安装：

```bash
cd sdk/python
pip install -e .
```

## 快速开始

```python
from rag_sdk import Client, RetrieveRequest

client = Client(
    base_url="http://localhost:8081",
    api_key="rag_xxxxxxxxxxxx",
)

request = RetrieveRequest(
    query="知识库里关于 Go 并发的内容是什么？",
    kb_ids=[1],
    top_k=5,
)

response = client.retrieve(request)

print(f"request_id: {response.request_id}")
for item in response.items:
    print(f"[{item.score:.2f}] {item.content}")

client.close()
```

## 异步使用

```python
import asyncio
from rag_sdk import Client, RetrieveRequest

async def main():
    async with Client(
        base_url="http://localhost:8081",
        api_key="rag_xxxxxxxxxxxx",
    ) as client:
        request = RetrieveRequest(query="...", kb_ids=[1])
        response = await client.aretrieve(request)
        # ...

asyncio.run(main())
```

## 错误处理

```python
from rag_sdk import RAGAPIError

try:
    response = client.retrieve(request)
except RAGAPIError as e:
    print(f"Status: {e.status_code}")
    print(f"Body: {e.body}")
```
```

---

## 五、如何验证？

### 5.1 单元测试

```bash
cd sdk/python
pip install -e ".[dev]"
pytest tests/ -v
```

### 5.2 手动测试

#### 步骤 1: 安装 SDK

```bash
cd sdk/python
pip install -e .
```

#### 步骤 2: 运行示例

```bash
python examples/basic_usage.py
```

#### 步骤 3: 测试异步

```bash
python examples/async_usage.py
```

### 5.3 验收标准

| 验收项 | 标准 |
|--------|------|
| 单元测试 | 100% 通过 |
| 类型安全 | Pydantic 模型正确验证 |
| 同步 API | 正常调用并返回结果 |
| 异步 API | 正常调用并返回结果 |
| 错误处理 | 正确抛出 RAGAPIError |

---

## 六、代码提交流程

### 6.1 提交代码

```bash
git checkout -b feature/TASK-005-python-sdk

git add sdk/python/

git commit -m "feat: TASK-005 实现 Python SDK

- 实现同步和异步 Client
- Pydantic 类型定义
- 完整错误处理
- 使用示例和单元测试"

git push origin feature/TASK-005-python-sdk
```

### 6.2 创建 Pull Request

**标题**: `feat: TASK-005 实现 Python SDK`

**内容**:

```markdown
## 任务说明
- 任务 ID：TASK-005
- 功能：Python SDK
- 实现人：[你的名字]

## 实现方案
- 使用 httpx 作为 HTTP 客户端
- Pydantic v2 类型定义
- 同时支持同步和异步 API
- 上下文管理器支持

## 验证结果
- [x] 单元测试通过
- [x] 手动测试通过
- [x] 示例代码可运行

## 相关文档
- 教程文档：./docs/TASK-005-python-sdk.md
- SDK README：./sdk/python/README.md
```

---

## 🎉 恭喜！

完成这个任务后，你将：
- ✅ 掌握 SDK 设计最佳实践
- ✅ 熟练使用 Pydantic
- ✅ 理解同步/异步 API 设计
- ✅ 学会编写 Python 库

**下一步**: 去做 TASK-006（Java SDK），难度差不多！
