# Phase 00：单人模式 + Docker 长跑（基础设施阶段）

目标：把 NOFX 变成“你个人电脑上能一直跑的服务”，并且默认更安全、更少干扰（遥测/注册/暴露面）。

## 范围（Scope）

- Docker 部署为主（`docker-compose.prod.yml`/`docker-compose.yml`）
- 单人可用：限制注册/限制用户数/关闭遥测
- 给出明确的“验证步骤”

## 你需要确认（Before we start）

1) 你是否希望“首次启动允许注册一次，然后永久关闭注册”？（推荐：是）  
2) Web/后端是否只在本机访问（127.0.0.1）？（推荐：是；如果你要局域网访问，需要额外安全措施）

## 任务清单（Tasks）

### P00-1：默认 `.env` 建议（不改代码版）
- 将以下写入你本机 `.env`：
  - `EXPERIENCE_IMPROVEMENT=false`
  - `MAX_USERS=1`
  - `REGISTRATION_ENABLED=true`（首次注册阶段）
- 建议仅本机访问（端口绑定 127.0.0.1）：
  - `NOFX_BIND_IP=127.0.0.1`
  - `NOFX_SINGLE_USER=true`（仅作标记/习惯用，不影响程序逻辑）
- 第一次注册成功后，把 `REGISTRATION_ENABLED` 改为 `false`，锁死注册入口。

你可以直接用脚本命令一键完成（推荐）：
- 开启单人模式：`./start.sh single-user-on`
- 首次注册后锁定注册：`./start.sh lock-registration`

### P00-2：Docker 长跑建议
- 使用 `docker-compose.prod.yml`（镜像版）或 `docker-compose.yml`（本地 build）。
- 确认 `./data` 做持久化（当前 compose 已包含）。
- 单人模式端口绑定：compose 已支持 `NOFX_BIND_IP`（默认 `0.0.0.0`），设置为 `127.0.0.1` 后 `3000/8080` 仅绑定到本机（更安全）。

### P00-3：可观测性（最小集）
- 使用内置 `pprof`（后端 `:6060`）观察内存与 goroutine。
- 日志：通过 `docker compose logs -f` 观察每个决策周期耗时与错误。
- 最常用命令（脚本版）：
  - `./start.sh status`
  - `./start.sh logs`
  - `./start.sh logs nofx` / `./start.sh logs nofx-frontend`

## 验收方式（Verification）

### 启动
- `cp .env.example .env`
- 运行 `./start.sh start`（它会自动生成加密相关 key，并启动 compose）

### 健康检查
- `curl http://127.0.0.1:8080/api/health`

### 单人模式（推荐）
- 运行 `./start.sh single-user-on`
- 应用端口绑定变更：`./start.sh recreate`
- 验证端口仅监听本机（任选其一）：
  - `lsof -nP -iTCP:8080 -sTCP:LISTEN`
  - `lsof -nP -iTCP:3000 -sTCP:LISTEN`

### 注册与锁定
- Web 打开 `http://127.0.0.1:3000` 完成注册/登录
- 注册完成后执行：`./start.sh lock-registration`
- 重启服务验证无法再注册：`./start.sh restart`

### 遥测关闭验证
- `.env` 设置 `EXPERIENCE_IMPROVEMENT=false` 后，重启服务
- 观察日志中不再出现 experience 相关上报失败/请求（如有）

## 交付物（Deliverables）

- 你本机有一个稳定可重启恢复的 NOFX 实例
- 单人登录可用，且注册入口已锁死
