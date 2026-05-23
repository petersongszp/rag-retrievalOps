# 面试吧 - 知识库管理后台

这是面试吧的知识库管理后台项目，独立运行在 3001 端口。

## 功能特性

- 知识库管理：创建、查看知识库
- 文档管理：上传、删除文档
- 任务监控：查看文档处理任务状态
- 检索测试：测试知识库检索功能

## 技术栈

- Next.js 14
- React 18
- TypeScript
- Ant Design 5
- Tailwind CSS

## 开发

```bash
# 安装依赖
npm install

# 开发模式（运行在 3001 端口）
npm run dev

# 构建
npm run build

# 生产模式运行
npm start
```

## 环境变量

创建 `.env.local` 文件：

```
NEXT_PUBLIC_API_BASE_URL=http://localhost:8899/api
```

## 项目结构

```
admin/
├── src/
│   ├── app/
│   │   ├── layout.tsx          # 根布局
│   │   └── page.tsx            # 主页面
│   ├── config/
│   │   └── api.ts              # API 配置
│   ├── services/
│   │   └── api/
│   │       └── client.ts       # API 客户端
│   ├── types/
│   │   └── kb.ts               # 类型定义
│   └── styles/
│       └── globals.css         # 全局样式
├── package.json
└── next.config.js
```
