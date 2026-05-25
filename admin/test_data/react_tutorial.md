# React 快速上手指南

## 什么是 React？

React 是一个用于构建用户界面的 JavaScript 库，由 Facebook 开发并维护。

## React 的核心概念

1. **组件** - React 应用的基本构建块
2. **JSX** - 一种语法扩展，可以在 JavaScript 中写 HTML
3. **Props** - 组件的输入，类似 HTML 标签的属性
4. **State** - 组件内部状态，用于存储和管理数据

## 示例：创建一个简单的组件

```jsx
function Hello({ name }) {
  return <h1>Hello, {name}!</h1>;
}

function App() {
  return (
    <div>
      <Hello name="Alice" />
      <Hello name="Bob" />
    </div>
  );
}
```
