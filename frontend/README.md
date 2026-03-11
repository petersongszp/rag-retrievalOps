# 上岸面试通AI面试平台（前端）

## 项目简介

### 核心定位
上岸面试通AI面试平台前端是一个基于现代Web技术栈开发的智能面试系统界面，负责为用户提供AI面试交互、简历管理、面试报告展示等核心功能。本前端项目与后端服务紧密协作，通过API接口实现数据交互，为用户提供流畅、高效的面试体验。

### 技术栈概览
- **React 18.x**: 用于构建用户界面的JavaScript库，提供组件化开发能力
- **Next.js 14.x**: React框架，提供服务器端渲染(SSR)、静态站点生成(SSG)等优化功能
- **TypeScript**: 提供类型安全的JavaScript超集
- **Ant Design**: 企业级UI组件库，提供丰富的界面组件
- **Tailwind CSS**: 实用优先的CSS框架，用于快速构建响应式界面
- **Zustand**: 轻量级状态管理库
- **Axios**: HTTP客户端，用于API请求

## 环境准备

### 前置依赖
- **Node.js**: 18.0 或更高版本
- **npm**: 8.0 或更高版本，或 **yarn**: 1.22 或更高版本

#### 使用nvm安装Node.js（推荐）
```bash
# 安装nvm（Node Version Manager）
# Windows用户可访问 https://github.com/coreybutler/nvm-windows 下载安装
# 或使用PowerShell安装
winget install CoreyButler.NVMforWindows

# 安装Node.js 18
nvm install 18

# 使用Node.js 18
nvm use 18

# 验证安装
node -v
npm -v
```

### 环境变量配置
在 `frontend/` 目录下创建 `.env.local` 文件，添加以下配置：

```env
# API基础URL（指向后端服务）
NEXT_PUBLIC_API_BASE_URL=http://localhost:8000/api

# 开发环境配置（如需其他配置可在此添加）
```

## 安装与启动步骤

### 进入目录
```bash
cd 项目根目录/frontend
```

### 依赖安装

#### 使用npm
```bash
# 在 frontend 目录下执行
npm install
```

#### 使用yarn
```bash
# 在 frontend 目录下执行
yarn install
```

#### 常见依赖冲突解决方案
- 遇到版本冲突时，尝试删除 `node_modules` 和 `package-lock.json`/`yarn.lock` 文件后重新安装
- 如遇权限问题，在Windows上尝试以管理员身份运行终端

### 启动开发环境

#### 使用npm
```bash
# 在 frontend 目录下执行
npm run dev
```

#### 使用yarn
```bash
# 在 frontend 目录下执行
yarn dev
```

启动成功标志：控制台输出类似以下信息
```
▲ Next.js 14.0.4
- Local:        http://localhost:3000
✓ Ready in 2.5s
```

### 验证方式
在浏览器中访问 `http://localhost:3000`，如果首页正常展示，则表示启动成功。

## 项目结构速览

### 核心目录说明
```
frontend/src/
├── app/             # Next.js App Router目录（页面和路由）
│   ├── globals.css  # 全局样式
│   ├── layout.tsx   # 根布局组件
│   └── page.tsx     # 首页
├── components/      # 可复用组件
│   ├── common/      # 通用基础组件
│   ├── home/        # 首页相关组件
│   └── layout/      # 布局组件（导航栏、页脚等）
├── services/        # 服务层（API调用等）
│   └── api/         # API相关服务
├── store/           # 状态管理
├── types/           # TypeScript类型定义
├── hooks/           # 自定义React Hooks
└── utils/           # 工具函数
```

### 关键文件说明
- **app/page.tsx**: 应用首页入口，定义路由根页面
- **services/api/client.ts**: API请求客户端基础封装，配置Axios实例
- **store/authStore.ts**: 用户认证状态管理
- **hooks/useAuth.ts**: 认证相关自定义Hook
- **components/common/Card.tsx**: 通用卡片组件
- **types/global.ts**: 全局类型定义（API响应、用户、面试等类型）

## 开发规范与常用工具

### 代码规范
项目使用 ESLint + Prettier 进行代码质量控制和格式化。

#### IDE插件建议（VS Code）
- ESLint
- Prettier - Code formatter
- Tailwind CSS IntelliSense
- TypeScript React code snippets

#### 代码格式化命令
```bash
# 检查代码格式
npm run lint

# 自动修复格式问题
npm run lint:fix

# 使用Prettier格式化
npx prettier --write .
```

### Git提交规范
建议遵循以下提交信息格式：
```
类型: 简短描述

详细描述（可选）
```

类型包括：
- **feat**: 新增功能
- **fix**: 修复bug
- **docs**: 文档更新
- **style**: 代码风格调整
- **refactor**: 代码重构
- **test**: 测试相关
- **chore**: 构建过程或辅助工具变动

### 常用命令

| 命令 | 说明 | 执行目录 |
|------|------|----------|
| `npm run dev` / `yarn dev` | 启动开发服务器 | frontend/ |
| `npm run build` / `yarn build` | 构建生产版本 | frontend/ |
| `npm start` / `yarn start` | 启动生产服务器 | frontend/ |
| `npm run lint` / `yarn lint` | 运行ESLint检查 | frontend/ |
| `npm run lint:fix` / `yarn lint:fix` | 自动修复ESLint错误 | frontend/ |

## 常见问题排查

### 启动问题

#### 端口占用
**错误现象**：启动时报错 `listen EADDRINUSE: address already in use :::3000`

**解决方案**：
1. 查找占用3000端口的进程并关闭
   ```bash
   # Windows
   netstat -ano | findstr :3000
   # 然后使用taskkill终止进程
   taskkill /PID [进程ID] /F
   ```
2. 或修改Next.js使用其他端口
   ```bash
   npm run dev -- -p 3001
   ```

#### 'use client' 指令错误
**错误现象**：报错 `SyntaxError: Invalid or unexpected token`

**解决方案**：确保 `'use client'` 语句位于文件的第一行，且没有前导空格或注释。

### 样式问题

#### Tailwind样式不生效
**错误现象**：应用了Tailwind类但样式未生效

**解决方案**：
1. 检查 `tailwind.config.js` 配置是否正确
2. 确认类名拼写是否正确
3. 尝试重启开发服务器

#### Ant Design样式问题
**错误现象**：Ant Design组件样式异常或丢失

**解决方案**：
1. 确保在使用Ant Design组件的文件顶部添加 `'use client'` 指令
2. 检查是否正确导入了Ant Design组件

### 环境配置问题

#### 环境变量未加载
**错误现象**：`process.env.NEXT_PUBLIC_*` 返回undefined

**解决方案**：
1. 确认 `.env.local` 文件已创建在正确位置（frontend/目录下）
2. 变量名必须以 `NEXT_PUBLIC_` 开头（用于客户端访问）
3. 重启开发服务器以加载新的环境变量