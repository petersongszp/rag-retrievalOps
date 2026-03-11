# Kubernetes 部署文档

## 1. 概述
本项目支持使用 Kubernetes 进行容器化编排部署。以下清单文件位于 `k8s/` 目录下。

## 2. 资源清单说明

### 2.1 基础服务
- `mysql.yaml`: 部署单节点 MySQL 8.0，包含 PersistentVolumeClaim (PVC) 以持久化数据。
- `redis.yaml`: 部署单节点 Redis 7，配置密码访问。

### 2.2 核心业务
- `backend.yaml`:
    - **ConfigMap**: 包含 `config.yaml` 配置，挂载到容器内。
    - **Deployment**: 后端 API 服务，环境变量注入数据库连接信息。
    - **Service**: NodePort 类型 (30888)，暴露 API 端口。
- `frontend.yaml`:
    - **Deployment**: Next.js 前端应用。
    - **Service**: NodePort 类型 (30000)，暴露 Web 访问端口。

## 3. 部署步骤

### 3.1 前置条件
- 已安装 kubectl
- 已配置 Kubernetes 集群 (如 Minikube, K3s, 或云厂商集群)
- 本地已有 Docker 镜像 (如果使用 Minikube，需 `minikube image load` 或使用本地 registry)

### 3.2 执行部署
```bash
# 1. 部署基础服务
kubectl apply -f k8s/mysql.yaml
kubectl apply -f k8s/redis.yaml

# 2. 等待数据库启动...
kubectl get pods

# 3. 部署业务服务
kubectl apply -f k8s/backend.yaml
kubectl apply -f k8s/frontend.yaml
```

### 3.3 验证
访问 `http://<NODE_IP>:30000` 查看前端页面。
访问 `http://<NODE_IP>:30888/ping` 验证后端 API。
