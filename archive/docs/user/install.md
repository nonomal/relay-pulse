# 安装指南

> **⚠️ 历史文档 / Deprecated** — Last verified: 2026-04-17
> 本文内容已过期，保留作为背景参考。现行文档请见仓库根 `README.md` 与 `docs/user/`。

> **Audience**: 用户 | **Last reviewed**: 2025-11-21

> 💡 **在线演示**: [https://relaypulse.top](https://relaypulse.top) - 体验完整功能后再部署

本文档介绍如何在不同环境下安装和部署 Relay Pulse。

## 前置要求

### Docker 部署（推荐）
- Docker 20.10+
- Docker Compose v2.0+

### 手动部署
- Go 1.24+
- Node.js 20+ (仅前端构建)
- SQLite 或 PostgreSQL

## 快速开始（5分钟）

### 1. 下载配置文件

```bash
# 创建项目目录
mkdir relay-pulse && cd relay-pulse

# 下载 docker-compose.yaml
curl -O https://raw.githubusercontent.com/prehisle/relay-pulse/main/docker-compose.yaml

# 下载配置模板
curl -O https://raw.githubusercontent.com/prehisle/relay-pulse/main/config.yaml.example
```

### 2. 准备配置

```bash
# 复制配置模板
cp config.yaml.example config.yaml

# 编辑配置（填入你的 API Key）
vi config.yaml
```

**最小配置示例**：

```yaml
interval: "1m"
slow_latency: "5s"

monitors:
  - provider: "openai"
    service: "gpt-4"
    category: "commercial"  # 必填
    sponsor: "团队自有"      # 必填
    url: "https://api.openai.com/v1/chat/completions"
    method: "POST"
    api_key: "sk-your-api-key-here"
    headers:
      Authorization: "Bearer {{API_KEY}}"
      Content-Type: "application/json"
    body: |
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "hello"}],
        "max_tokens": 10
      }
```

### 3. 启动服务

```bash
docker compose up -d
```

### 4. 访问服务

- **Web 界面**: http://localhost:8080
- **API 端点**: http://localhost:8080/api/status
- **健康检查**: http://localhost:8080/health

完成！🎉

## Docker 部署

### 使用预构建镜像

```bash
# 拉取最新镜像
docker pull ghcr.io/prehisle/relay-pulse:latest

# 使用 Docker Compose（推荐）
docker compose up -d

# 或手动启动
docker run -d \
  --name relaypulse-monitor \
  -p 8080:8080 \
  -v $(pwd)/config.yaml:/app/config.yaml:ro \
  -v relay-pulse-data:/data \
  -e MONITOR_SQLITE_PATH=/data/monitor.db \
  ghcr.io/prehisle/relay-pulse:latest
```

### 本地构建镜像

```bash
# 克隆仓库
git clone https://github.com/prehisle/relay-pulse.git
cd relay-pulse

# 构建镜像
docker build -t relay-pulse:latest .

# 启动
docker compose up -d
```

### Docker Compose 常用命令

```bash
# 查看运行状态
docker compose ps

# 查看实时日志
docker compose logs -f monitor

# 重启服务（配置更新后）
docker compose restart

# 停止服务
docker compose down

# 更新到最新版本
docker compose pull
docker compose up -d

# 备份数据库
docker compose exec monitor cp /data/monitor.db /tmp/backup.db
docker cp relaypulse-monitor:/tmp/backup.db ./monitor-backup-$(date +%Y%m%d).db
```

## Kubernetes 部署

### PostgreSQL 模式（推荐）

Relay Pulse 支持 PostgreSQL 存储，适合 K8s 多副本部署：

**1. 创建 ConfigMap**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: relay-pulse-config
data:
  config.yaml: |
    interval: "1m"
    slow_latency: "5s"
    monitors:
      - provider: "openai"
        service: "gpt-4"
        category: "commercial"
        sponsor: "团队"
        url: "https://api.openai.com/v1/chat/completions"
        method: "POST"
        headers:
          Authorization: "Bearer {{API_KEY}}"
        body: |
          {
            "model": "gpt-4",
            "messages": [{"role": "user", "content": "hi"}],
            "max_tokens": 1
          }
```

**2. 创建 Secret（API Keys）**

```bash
kubectl create secret generic relay-pulse-secrets \
  --from-literal=MONITOR_OPENAI_GPT4_API_KEY=sk-your-key \
  --from-literal=MONITOR_POSTGRES_PASSWORD=your-db-password
```

**3. 部署 PostgreSQL**

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres
spec:
  serviceName: postgres
  replicas: 1
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
      - name: postgres
        image: postgres:16-alpine
        env:
        - name: POSTGRES_DB
          value: llm_monitor
        - name: POSTGRES_USER
          value: monitor
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: relay-pulse-secrets
              key: MONITOR_POSTGRES_PASSWORD
        ports:
        - containerPort: 5432
        volumeMounts:
        - name: postgres-data
          mountPath: /var/lib/postgresql/data
  volumeClaimTemplates:
  - metadata:
      name: postgres-data
    spec:
      accessModes: [ "ReadWriteOnce" ]
      resources:
        requests:
          storage: 10Gi
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
spec:
  ports:
  - port: 5432
  selector:
    app: postgres
```

**4. 部署 Relay Pulse**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: relay-pulse
spec:
  replicas: 3  # 多副本水平扩展
  selector:
    matchLabels:
      app: relay-pulse
  template:
    metadata:
      labels:
        app: relay-pulse
    spec:
      containers:
      - name: monitor
        image: ghcr.io/prehisle/relay-pulse:latest
        ports:
        - containerPort: 8080
        env:
        - name: MONITOR_STORAGE_TYPE
          value: "postgres"
        - name: MONITOR_POSTGRES_HOST
          value: "postgres"
        - name: MONITOR_POSTGRES_PORT
          value: "5432"
        - name: MONITOR_POSTGRES_USER
          value: "monitor"
        - name: MONITOR_POSTGRES_DATABASE
          value: "llm_monitor"
        - name: MONITOR_POSTGRES_SSLMODE
          value: "require"
        - name: MONITOR_POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: relay-pulse-secrets
              key: MONITOR_POSTGRES_PASSWORD
        envFrom:
        - secretRef:
            name: relay-pulse-secrets  # 加载 API Keys
        volumeMounts:
        - name: config
          mountPath: /app/config.yaml
          subPath: config.yaml
          readOnly: true
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
      volumes:
      - name: config
        configMap:
          name: relay-pulse-config
---
apiVersion: v1
kind: Service
metadata:
  name: relay-pulse
spec:
  type: ClusterIP
  ports:
  - port: 80
    targetPort: 8080
  selector:
    app: relay-pulse
```

**5. 暴露服务（Ingress）**

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: relay-pulse
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
spec:
  tls:
  - hosts:
    - relaypulse.example.com
    secretName: relay-pulse-tls
  rules:
  - host: relaypulse.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: relay-pulse
            port:
              number: 80
```

## 手动部署（Systemd）

### 1. 编译后端

```bash
git clone https://github.com/prehisle/relay-pulse.git
cd relay-pulse

# 编译
go build -o monitor ./cmd/server

# 构建前端
cd frontend
npm ci
npm run build
cd ..
```

### 2. 部署到服务器

```bash
# 创建部署目录
sudo mkdir -p /opt/relay-pulse/{config,data}
sudo useradd -r -s /bin/false monitor

# 复制文件
sudo cp monitor /opt/relay-pulse/
sudo cp config.production.yaml /opt/relay-pulse/config/
sudo chown -R monitor:monitor /opt/relay-pulse
```

### 3. 创建环境变量文件

```bash
sudo vi /etc/relay-pulse.env
```

```bash
# API Keys
MONITOR_OPENAI_GPT4_API_KEY=sk-your-key

# 数据库（可选，默认 SQLite）
MONITOR_SQLITE_PATH=/opt/relay-pulse/data/monitor.db
```

### 4. 创建 Systemd 单元

创建 `/etc/systemd/system/relay-pulse.service`:

```ini
[Unit]
Description=Relay Pulse Monitor
After=network.target

[Service]
Type=simple
User=monitor
WorkingDirectory=/opt/relay-pulse
EnvironmentFile=/etc/relay-pulse.env
ExecStart=/opt/relay-pulse/monitor /opt/relay-pulse/config/config.production.yaml
Restart=always
RestartSec=10
LimitNOFILE=4096

# 安全加固
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/opt/relay-pulse/data

[Install]
WantedBy=multi-user.target
```

### 5. 启动服务

```bash
sudo systemctl daemon-reload
sudo systemctl enable relay-pulse.service
sudo systemctl start relay-pulse.service
sudo systemctl status relay-pulse.service
```

## 升级指南

### Docker Compose 升级

```bash
# 拉取最新镜像
docker compose pull

# 重启服务
docker compose up -d

# 验证版本
curl http://localhost:8080/api/version
```

### 手动升级

```bash
# 备份配置和数据
cp config.yaml config.yaml.backup
cp data/monitor.db data/monitor.db.backup

# 拉取最新代码
git pull origin main

# 重新编译
go build -o monitor ./cmd/server

# 重启服务
sudo systemctl restart relay-pulse.service
```

## 验证安装

运行以下命令验证安装成功：

```bash
# 检查健康状态
curl http://localhost:8080/health
# 应该返回: {"status":"ok"}

# 检查 API 数据
curl http://localhost:8080/api/status | jq .

# 检查版本信息
curl http://localhost:8080/api/version
```

## 故障排查

如果遇到问题，请参考 [运维手册 - 故障排查](operations.md#故障排查)。

## 下一步

- [配置手册](config.md) - 详细配置说明
- [运维手册](operations.md) - 日常运维操作
