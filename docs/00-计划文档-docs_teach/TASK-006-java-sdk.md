# TASK-006: Java SDK 开发详细实现教程

&gt; 🎯 **任务 ID**: TASK-006
&gt;
&gt; **功能名称**: Java SDK
&gt;
&gt; **预估工时**: 10 小时
&gt;
&gt; **难度**: ⭐⭐ (入门级)
&gt;
&gt; **技术栈**: Java、Maven/Gradle、OkHttp、Jackson
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

企业级应用和后端服务很多使用 Java 开发，缺少 Java SDK 会让 Java 开发者对接困难。

### 1.2 解决方案

开发 Java SDK，提供流畅的 Builder 模式 API，支持同步和异步调用。

### 1.3 功能需求

| 功能点 | 说明 |
|--------|------|
| 客户端初始化 | Builder 模式配置 BaseURL、API Key、超时 |
| 检索接口 | 封装 `/v1/retrieve` API |
| 类型安全 | POJO 定义请求/响应模型 |
| 错误处理 | 统一的异常类型 |
| 异步支持 | CompletableFuture 异步 API |
| 连接池 | OkHttp 连接池复用 |

---

## 二、为什么要做这个？

### 2.1 业务价值

| 指标 | 预期提升 |
|------|---------|
| Java 用户对接效率 | 从 2 小时降到 10 分钟 |
| 企业级采用率 | 提升 40% |

### 2.2 技术价值

- 学习 **Java 库设计模式**
- 掌握 **OkHttp 和 Jackson**
- 理解 **Builder 模式**
- 实践 **CompletableFuture 异步编程**

---

## 三、技术原理

### 3.1 系统架构

```
┌─────────────────┐
│  Java Service   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Java SDK       │ ← (1) 提供 Builder API
│  - Client       │
│  - OkHttp       │
│  - Jackson      │
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
└── java/
    ├── src/
    │   ├── main/
    │   │   └── java/
    │   │       └── com/
    │   │           └── rag/
    │   │               └── sdk/
    │   │                   ├── RagClient.java
    │   │                   ├── RagClientBuilder.java
    │   │                   ├── model/
    │   │                   │   ├── RetrieveRequest.java
    │   │                   │   ├── RetrieveResponse.java
    │   │                   │   ├── RetrieveItem.java
    │   │                   │   └── package-info.java
    │   │                   └── exception/
    │   │                       └── RagApiException.java
    │   └── test/
    │       └── java/
    │           └── com/
    │               └── rag/
    │                   └── sdk/
    │                       └── RagClientTest.java
    ├── examples/
    │   └── src/
    │       └── main/
    │           └── java/
    │               └── com/
    │                   └── rag/
    │                       └── sdk/
    │                           └── examples/
    │                               └── BasicUsage.java
    ├── pom.xml
    └── README.md
```

---

## 四、实现步骤

### Step 0: 创建目录结构

```bash
mkdir -p sdk/java/src/main/java/com/rag/sdk/{model,exception}
mkdir -p sdk/java/src/test/java/com/rag/sdk
mkdir -p sdk/java/examples/src/main/java/com/rag/sdk/examples
```

### Step 1: 定义异常类

**文件**: `sdk/java/src/main/java/com/rag/sdk/exception/RagApiException.java`

```java
package com.rag.sdk.exception;

public class RagApiException extends RuntimeException {
    private final int statusCode;
    private final String body;

    public RagApiException(int statusCode, String body) {
        super(String.format("RAG API returned %d: %s", statusCode, body));
        this.statusCode = statusCode;
        this.body = body;
    }

    public int getStatusCode() {
        return statusCode;
    }

    public String getBody() {
        return body;
    }
}
```

### Step 2: 定义模型类

**文件**: `sdk/java/src/main/java/com/rag/sdk/model/RetrieveItem.java`

```java
package com.rag.sdk.model;

import com.fasterxml.jackson.annotation.JsonProperty;

public class RetrieveItem {
    @JsonProperty("content")
    private String content;

    @JsonProperty("score")
    private double score;

    @JsonProperty("citation")
    private Object citation;

    @JsonProperty("source")
    private Object source;

    public String getContent() {
        return content;
    }

    public void setContent(String content) {
        this.content = content;
    }

    public double getScore() {
        return score;
    }

    public void setScore(double score) {
        this.score = score;
    }

    public Object getCitation() {
        return citation;
    }

    public void setCitation(Object citation) {
        this.citation = citation;
    }

    public Object getSource() {
        return source;
    }

    public void setSource(Object source) {
        this.source = source;
    }
}
```

**文件**: `sdk/java/src/main/java/com/rag/sdk/model/RetrieveRequest.java`

```java
package com.rag.sdk.model;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;

import java.util.List;
import java.util.Map;

@JsonInclude(JsonInclude.Include.NON_NULL)
public class RetrieveRequest {
    @JsonProperty("query")
    private String query;

    @JsonProperty("kb_ids")
    private List&lt;Long&gt; kbIds;

    @JsonProperty("top_k")
    private Integer topK;

    @JsonProperty("strategy_profile")
    private String strategyProfile;

    @JsonProperty("metadata_filter")
    private Map&lt;String, Object&gt; metadataFilter;

    public String getQuery() {
        return query;
    }

    public void setQuery(String query) {
        this.query = query;
    }

    public List&lt;Long&gt; getKbIds() {
        return kbIds;
    }

    public void setKbIds(List&lt;Long&gt; kbIds) {
        this.kbIds = kbIds;
    }

    public Integer getTopK() {
        return topK;
    }

    public void setTopK(Integer topK) {
        this.topK = topK;
    }

    public String getStrategyProfile() {
        return strategyProfile;
    }

    public void setStrategyProfile(String strategyProfile) {
        this.strategyProfile = strategyProfile;
    }

    public Map&lt;String, Object&gt; getMetadataFilter() {
        return metadataFilter;
    }

    public void setMetadataFilter(Map&lt;String, Object&gt; metadataFilter) {
        this.metadataFilter = metadataFilter;
    }

    public static Builder builder() {
        return new Builder();
    }

    public static class Builder {
        private String query;
        private List&lt;Long&gt; kbIds;
        private Integer topK;
        private String strategyProfile;
        private Map&lt;String, Object&gt; metadataFilter;

        public Builder query(String query) {
            this.query = query;
            return this;
        }

        public Builder kbIds(List&lt;Long&gt; kbIds) {
            this.kbIds = kbIds;
            return this;
        }

        public Builder topK(Integer topK) {
            this.topK = topK;
            return this;
        }

        public Builder strategyProfile(String strategyProfile) {
            this.strategyProfile = strategyProfile;
            return this;
        }

        public Builder metadataFilter(Map&lt;String, Object&gt; metadataFilter) {
            this.metadataFilter = metadataFilter;
            return this;
        }

        public RetrieveRequest build() {
            RetrieveRequest request = new RetrieveRequest();
            request.query = this.query;
            request.kbIds = this.kbIds;
            request.topK = this.topK;
            request.strategyProfile = this.strategyProfile;
            request.metadataFilter = this.metadataFilter;
            return request;
        }
    }
}
```

**文件**: `sdk/java/src/main/java/com/rag/sdk/model/RetrieveResponse.java`

```java
package com.rag.sdk.model;

import com.fasterxml.jackson.annotation.JsonProperty;

import java.util.List;

public class RetrieveResponse {
    @JsonProperty("request_id")
    private String requestId;

    @JsonProperty("items")
    private List&lt;RetrieveItem&gt; items;

    public String getRequestId() {
        return requestId;
    }

    public void setRequestId(String requestId) {
        this.requestId = requestId;
    }

    public List&lt;RetrieveItem&gt; getItems() {
        return items;
    }

    public void setItems(List&lt;RetrieveItem&gt; items) {
        this.items = items;
    }
}
```

### Step 3: 实现客户端 Builder

**文件**: `sdk/java/src/main/java/com/rag/sdk/RagClientBuilder.java`

```java
package com.rag.sdk;

import java.time.Duration;

public class RagClientBuilder {
    private String baseUrl;
    private String apiKey;
    private Duration timeout = Duration.ofSeconds(10);

    public RagClientBuilder baseUrl(String baseUrl) {
        this.baseUrl = baseUrl;
        return this;
    }

    public RagClientBuilder apiKey(String apiKey) {
        this.apiKey = apiKey;
        return this;
    }

    public RagClientBuilder timeout(Duration timeout) {
        this.timeout = timeout;
        return this;
    }

    public RagClient build() {
        if (baseUrl == null || baseUrl.isEmpty()) {
            throw new IllegalArgumentException("baseUrl is required");
        }
        return new RagClient(baseUrl, apiKey, timeout);
    }
}
```

### Step 4: 实现客户端

**文件**: `sdk/java/src/main/java/com/rag/sdk/RagClient.java`

```java
package com.rag.sdk;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.rag.sdk.exception.RagApiException;
import com.rag.sdk.model.RetrieveRequest;
import com.rag.sdk.model.RetrieveResponse;
import okhttp3.*;

import java.io.IOException;
import java.time.Duration;
import java.util.concurrent.CompletableFuture;

public class RagClient implements AutoCloseable {
    private static final MediaType JSON = MediaType.get("application/json; charset=utf-8");

    private final String baseUrl;
    private final String apiKey;
    private final OkHttpClient httpClient;
    private final ObjectMapper objectMapper;

    RagClient(String baseUrl, String apiKey, Duration timeout) {
        this.baseUrl = baseUrl.endsWith("/") ? baseUrl.substring(0, baseUrl.length() - 1) : baseUrl;
        this.apiKey = apiKey;
        this.httpClient = new OkHttpClient.Builder()
                .connectTimeout(timeout)
                .readTimeout(timeout)
                .writeTimeout(timeout)
                .build();
        this.objectMapper = new ObjectMapper();
    }

    public static RagClientBuilder builder() {
        return new RagClientBuilder();
    }

    public RetrieveResponse retrieve(RetrieveRequest request) throws RagApiException {
        try {
            String jsonBody = objectMapper.writeValueAsString(request);
            Request httpRequest = buildRequest("/v1/retrieve", jsonBody);

            try (Response response = httpClient.newCall(httpRequest).execute()) {
                if (!response.isSuccessful()) {
                    String body = response.body() != null ? response.body().string() : "";
                    throw new RagApiException(response.code(), body);
                }

                String responseBody = response.body() != null ? response.body().string() : "";
                return objectMapper.readValue(responseBody, RetrieveResponse.class);
            }
        } catch (IOException e) {
            throw new RuntimeException("Failed to execute request", e);
        }
    }

    public CompletableFuture&lt;RetrieveResponse&gt; retrieveAsync(RetrieveRequest request) {
        CompletableFuture&lt;RetrieveResponse&gt; future = new CompletableFuture&lt;&gt;();

        try {
            String jsonBody = objectMapper.writeValueAsString(request);
            Request httpRequest = buildRequest("/v1/retrieve", jsonBody);

            httpClient.newCall(httpRequest).enqueue(new Callback() {
                @Override
                public void onFailure(Call call, IOException e) {
                    future.completeExceptionally(e);
                }

                @Override
                public void onResponse(Call call, Response response) {
                    try {
                        if (!response.isSuccessful()) {
                            String body = response.body() != null ? response.body().string() : "";
                            future.completeExceptionally(new RagApiException(response.code(), body));
                            return;
                        }

                        String responseBody = response.body() != null ? response.body().string() : "";
                        RetrieveResponse result = objectMapper.readValue(responseBody, RetrieveResponse.class);
                        future.complete(result);
                    } catch (IOException e) {
                        future.completeExceptionally(e);
                    } finally {
                        response.close();
                    }
                }
            });
        } catch (IOException e) {
            future.completeExceptionally(e);
        }

        return future;
    }

    private Request buildRequest(String path, String jsonBody) {
        Request.Builder builder = new Request.Builder()
                .url(baseUrl + path)
                .post(RequestBody.create(jsonBody, JSON));

        if (apiKey != null &amp;&amp; !apiKey.isEmpty()) {
            builder.header("Authorization", "Bearer " + apiKey);
        }

        return builder.build();
    }

    @Override
    public void close() {
        httpClient.dispatcher().executorService().shutdown();
        httpClient.connectionPool().evictAll();
    }
}
```

### Step 5: 配置 pom.xml

**文件**: `sdk/java/pom.xml`

```xml
&lt;?xml version="1.0" encoding="UTF-8"?&gt;
&lt;project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd"&gt;
    &lt;modelVersion&gt;4.0.0&lt;/modelVersion&gt;

    &lt;groupId&gt;com.rag&lt;/groupId&gt;
    &lt;artifactId&gt;rag-sdk&lt;/artifactId&gt;
    &lt;version&gt;0.1.0&lt;/version&gt;
    &lt;packaging&gt;jar&lt;/packaging&gt;

    &lt;name&gt;RAG SDK&lt;/name&gt;
    &lt;description&gt;RAG Platform Java SDK&lt;/description&gt;

    &lt;properties&gt;
        &lt;maven.compiler.source&gt;11&lt;/maven.compiler.source&gt;
        &lt;maven.compiler.target&gt;11&lt;/maven.compiler.target&gt;
        &lt;project.build.sourceEncoding&gt;UTF-8&lt;/project.build.sourceEncoding&gt;
    &lt;/properties&gt;

    &lt;dependencies&gt;
        &lt;dependency&gt;
            &lt;groupId&gt;com.squareup.okhttp3&lt;/groupId&gt;
            &lt;artifactId&gt;okhttp&lt;/artifactId&gt;
            &lt;version&gt;4.12.0&lt;/version&gt;
        &lt;/dependency&gt;

        &lt;dependency&gt;
            &lt;groupId&gt;com.fasterxml.jackson.core&lt;/groupId&gt;
            &lt;artifactId&gt;jackson-databind&lt;/artifactId&gt;
            &lt;version&gt;2.16.0&lt;/version&gt;
        &lt;/dependency&gt;

        &lt;dependency&gt;
            &lt;groupId&gt;org.junit.jupiter&lt;/groupId&gt;
            &lt;artifactId&gt;junit-jupiter&lt;/artifactId&gt;
            &lt;version&gt;5.10.0&lt;/version&gt;
            &lt;scope&gt;test&lt;/scope&gt;
        &lt;/dependency&gt;
    &lt;/dependencies&gt;

    &lt;build&gt;
        &lt;plugins&gt;
            &lt;plugin&gt;
                &lt;groupId&gt;org.apache.maven.plugins&lt;/groupId&gt;
                &lt;artifactId&gt;maven-compiler-plugin&lt;/artifactId&gt;
                &lt;version&gt;3.11.0&lt;/version&gt;
                &lt;configuration&gt;
                    &lt;source&gt;11&lt;/source&gt;
                    &lt;target&gt;11&lt;/target&gt;
                &lt;/configuration&gt;
            &lt;/plugin&gt;
        &lt;/plugins&gt;
    &lt;/build&gt;
&lt;/project&gt;
```

### Step 6: 编写使用示例

**文件**: `sdk/java/examples/src/main/java/com/rag/sdk/examples/BasicUsage.java`

```java
package com.rag.sdk.examples;

import com.rag.sdk.RagClient;
import com.rag.sdk.exception.RagApiException;
import com.rag.sdk.model.RetrieveRequest;
import com.rag.sdk.model.RetrieveResponse;

import java.util.Arrays;
import java.util.concurrent.CompletableFuture;

public class BasicUsage {
    public static void main(String[] args) {
        try (RagClient client = RagClient.builder()
                .baseUrl("http://localhost:8081")
                .apiKey("rag_xxxxxxxxxxxx")
                .build()) {

            syncExample(client);
            asyncExample(client);

        } catch (Exception e) {
            e.printStackTrace();
        }
    }

    private static void syncExample(RagClient client) {
        System.out.println("=== Sync Example ===");

        try {
            RetrieveRequest request = RetrieveRequest.builder()
                    .query("知识库里关于 Go 并发的内容是什么？")
                    .kbIds(Arrays.asList(1L))
                    .topK(5)
                    .build();

            RetrieveResponse response = client.retrieve(request);

            System.out.println("request_id: " + response.getRequestId());
            response.getItems().forEach(item -&gt; {
                System.out.printf("[%.2f] %s%n", item.getScore(), item.getContent());
            });

        } catch (RagApiException e) {
            System.err.println("Error: " + e.getMessage());
            System.err.println("Status code: " + e.getStatusCode());
            System.err.println("Body: " + e.getBody());
        }
    }

    private static void asyncExample(RagClient client) {
        System.out.println("\n=== Async Example ===");

        RetrieveRequest request = RetrieveRequest.builder()
                .query("知识库里关于 Go 并发的内容是什么？")
                .kbIds(Arrays.asList(1L))
                .topK(5)
                .build();

        CompletableFuture&lt;RetrieveResponse&gt; future = client.retrieveAsync(request);

        future.thenAccept(response -&gt; {
            System.out.println("request_id: " + response.getRequestId());
            response.getItems().forEach(item -&gt; {
                System.out.printf("[%.2f] %s%n", item.getScore(), item.getContent());
            });
        }).exceptionally(e -&gt; {
            e.printStackTrace();
            return null;
        }).join();
    }
}
```

### Step 7: 编写单元测试

**文件**: `sdk/java/src/test/java/com/rag/sdk/RagClientTest.java`

```java
package com.rag.sdk;

import com.rag.sdk.exception.RagApiException;
import com.rag.sdk.model.RetrieveRequest;
import com.rag.sdk.model.RetrieveResponse;
import okhttp3.mockwebserver.MockResponse;
import okhttp3.mockwebserver.MockWebServer;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.util.Arrays;

import static org.junit.jupiter.api.Assertions.*;

class RagClientTest {
    private MockWebServer mockWebServer;
    private RagClient client;

    @BeforeEach
    void setUp() throws IOException {
        mockWebServer = new MockWebServer();
        mockWebServer.start();
        client = RagClient.builder()
                .baseUrl(mockWebServer.url("/").toString())
                .apiKey("test-key")
                .build();
    }

    @AfterEach
    void tearDown() throws IOException {
        client.close();
        mockWebServer.shutdown();
    }

    @Test
    void testRetrieveSuccess() {
        String jsonResponse = """
            {
                "request_id": "test-123",
                "items": [
                    {
                        "content": "test content",
                        "score": 0.95,
                        "citation": {},
                        "source": {}
                    }
                ]
            }
            """;

        mockWebServer.enqueue(new MockResponse()
                .setBody(jsonResponse)
                .addHeader("Content-Type", "application/json"));

        RetrieveRequest request = RetrieveRequest.builder()
                .query("test")
                .kbIds(Arrays.asList(1L))
                .build();

        RetrieveResponse response = client.retrieve(request);

        assertEquals("test-123", response.getRequestId());
        assertEquals(1, response.getItems().size());
        assertEquals("test content", response.getItems().get(0).getContent());
    }

    @Test
    void testRetrieveError() {
        mockWebServer.enqueue(new MockResponse()
                .setResponseCode(401)
                .setBody("Invalid API key"));

        RetrieveRequest request = RetrieveRequest.builder()
                .query("test")
                .kbIds(Arrays.asList(1L))
                .build();

        RagApiException exception = assertThrows(RagApiException.class, () -&gt; {
            client.retrieve(request);
        });

        assertEquals(401, exception.getStatusCode());
        assertTrue(exception.getBody().contains("Invalid API key"));
    }
}
```

### Step 8: 创建 README

**文件**: `sdk/java/README.md`

```markdown
# RAG Platform Java SDK

## 安装

### Maven

```xml
&lt;dependency&gt;
    &lt;groupId&gt;com.rag&lt;/groupId&gt;
    &lt;artifactId&gt;rag-sdk&lt;/artifactId&gt;
    &lt;version&gt;0.1.0&lt;/version&gt;
&lt;/dependency&gt;
```

### Gradle

```groovy
implementation 'com.rag:rag-sdk:0.1.0'
```

## 快速开始

```java
import com.rag.sdk.RagClient;
import com.rag.sdk.model.RetrieveRequest;
import com.rag.sdk.model.RetrieveResponse;
import java.util.Arrays;

try (RagClient client = RagClient.builder()
        .baseUrl("http://localhost:8081")
        .apiKey("rag_xxxxxxxxxxxx")
        .build()) {

    RetrieveRequest request = RetrieveRequest.builder()
            .query("知识库里关于 Go 并发的内容是什么？")
            .kbIds(Arrays.asList(1L))
            .topK(5)
            .build();

    RetrieveResponse response = client.retrieve(request);

    System.out.println("request_id: " + response.getRequestId());
    response.getItems().forEach(item -&gt; {
        System.out.printf("[%.2f] %s%n", item.getScore(), item.getContent());
    });
}
```

## 异步使用

```java
CompletableFuture&lt;RetrieveResponse&gt; future = client.retrieveAsync(request);

future.thenAccept(response -&gt; {
    System.out.println("request_id: " + response.getRequestId());
}).exceptionally(e -&gt; {
    e.printStackTrace();
    return null;
});
```

## 错误处理

```java
import com.rag.sdk.exception.RagApiException;

try {
    RetrieveResponse response = client.retrieve(request);
} catch (RagApiException e) {
    System.out.println("Status: " + e.getStatusCode());
    System.out.println("Body: " + e.getBody());
}
```
```

---

## 五、如何验证？

### 5.1 单元测试

```bash
cd sdk/java
mvn test
```

### 5.2 手动测试

```bash
cd sdk/java
mvn install
cd examples
mvn compile exec:java -Dexec.mainClass="com.rag.sdk.examples.BasicUsage"
```

### 5.3 验收标准

| 验收项 | 标准 |
|--------|------|
| 单元测试 | 100% 通过 |
| Builder 模式 | 流畅的 API 设计 |
| 同步 API | 正常调用并返回结果 |
| 异步 API | CompletableFuture 正常工作 |
| 资源管理 | AutoCloseable 正确释放资源 |

---

## 六、代码提交流程

### 6.1 提交代码

```bash
git checkout -b feature/TASK-006-java-sdk

git add sdk/java/

git commit -m "feat: TASK-006 实现 Java SDK

- Builder 模式客户端
- OkHttp + Jackson 实现
- 同步和异步 API
- 完整单元测试"

git push origin feature/TASK-006-java-sdk
```

### 6.2 创建 Pull Request

**标题**: `feat: TASK-006 实现 Java SDK`

---

## 🎉 恭喜！

完成这个任务后，你将：
- ✅ 掌握 Java 库设计最佳实践
- ✅ 熟练使用 OkHttp 和 Jackson
- ✅ 理解 Builder 模式
- ✅ 学会 CompletableFuture 异步编程

**下一步**: 去做 TASK-007（TypeScript SDK）！
