#!/bin/bash

# ═══════════════════════════════════════════════════════════════
# NOFX AI Trading System - Docker Quick Start Script
# Usage: ./start.sh [command]
# ═══════════════════════════════════════════════════════════════

set -e

# ------------------------------------------------------------------------
# Color Definitions
# ------------------------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# ------------------------------------------------------------------------
# Utility Functions: Colored Output
# ------------------------------------------------------------------------
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# ------------------------------------------------------------------------
# Detection: Docker Compose Command (Backward Compatible)
# ------------------------------------------------------------------------
detect_compose_cmd() {
    if command -v docker compose &> /dev/null; then
        COMPOSE_CMD="docker compose"
    elif command -v docker-compose &> /dev/null; then
        COMPOSE_CMD="docker-compose"
    else
        print_error "Docker Compose 未安装！请先安装 Docker Compose"
        exit 1
    fi
    print_info "使用 Docker Compose 命令: $COMPOSE_CMD"
}

setup_compose_args() {
    COMPOSE_ARGS=()
    # Use explicit compose file for consistency (and to allow future extension).
    if [ -f "docker-compose.yml" ]; then
        COMPOSE_ARGS+=(-f docker-compose.yml)
    fi
}

# ------------------------------------------------------------------------
# Validation: Docker Installation
# ------------------------------------------------------------------------
check_docker() {
    if ! command -v docker &> /dev/null; then
        print_error "Docker 未安装！请先安装 Docker: https://docs.docker.com/get-docker/"
        exit 1
    fi

    detect_compose_cmd
    print_success "Docker 和 Docker Compose 已安装"
}

# ------------------------------------------------------------------------
# Validation: Environment File (.env)
# ------------------------------------------------------------------------
check_env() {
    if [ ! -f ".env" ]; then
        print_warning ".env 不存在，从模板复制..."
        cp .env.example .env
        print_info "已创建 .env 文件"
    fi
    print_success "环境变量文件存在"
}

# ------------------------------------------------------------------------
# Helper: Check if env var is set and not placeholder
# ------------------------------------------------------------------------
is_env_configured() {
    local var_name="$1"
    local value=$(grep "^${var_name}=" .env 2>/dev/null | cut -d'=' -f2-)

    # 去除引号
    value=$(echo "$value" | tr -d '"'"'")

    # 检查是否为空或占位符
    if [ -z "$value" ]; then
        return 1
    fi

    # 检查是否是示例值
    case "$value" in
        *your-*|*YOUR_*|*change-this*|*CHANGE_THIS*|*example*|*EXAMPLE*)
            return 1
            ;;
    esac

    return 0
}

# ------------------------------------------------------------------------
# Helper: Generate and set env var in .env file
# ------------------------------------------------------------------------
set_env_var() {
    local var_name="$1"
    local var_value="$2"

    # 如果变量已存在（即使是占位符），替换它
    if grep -q "^${var_name}=" .env 2>/dev/null; then
        # macOS 和 Linux 兼容的 sed
        if [[ "$OSTYPE" == "darwin"* ]]; then
            sed -i '' "s|^${var_name}=.*|${var_name}=${var_value}|" .env
        else
            sed -i "s|^${var_name}=.*|${var_name}=${var_value}|" .env
        fi
    else
        # 变量不存在，追加
        echo "${var_name}=${var_value}" >> .env
    fi
}

# ------------------------------------------------------------------------
# Validation: Encryption Keys in .env
# ------------------------------------------------------------------------
check_encryption() {
    print_info "检查加密密钥配置..."

    local generated=false

    # 检查并生成 JWT_SECRET
    if ! is_env_configured "JWT_SECRET"; then
        print_warning "JWT_SECRET 未配置，正在生成..."
        local jwt_secret=$(openssl rand -base64 32)
        set_env_var "JWT_SECRET" "$jwt_secret"
        print_success "JWT_SECRET 已生成"
        generated=true
    fi

    # 检查并生成 DATA_ENCRYPTION_KEY
    if ! is_env_configured "DATA_ENCRYPTION_KEY"; then
        print_warning "DATA_ENCRYPTION_KEY 未配置，正在生成..."
        local data_key=$(openssl rand -base64 32)
        set_env_var "DATA_ENCRYPTION_KEY" "$data_key"
        print_success "DATA_ENCRYPTION_KEY 已生成"
        generated=true
    fi

    # 检查并生成 RSA_PRIVATE_KEY
    if ! is_env_configured "RSA_PRIVATE_KEY"; then
        print_warning "RSA_PRIVATE_KEY 未配置，正在生成..."
        # 生成 RSA 密钥并转换为单行格式（\n 替换为 \\n）
        local rsa_key=$(openssl genrsa 2048 2>/dev/null | awk '{printf "%s\\n", $0}')
        set_env_var "RSA_PRIVATE_KEY" "\"$rsa_key\""
        print_success "RSA_PRIVATE_KEY 已生成"
        generated=true
    fi

    if [ "$generated" = true ]; then
        echo ""
        print_success "所有缺失的密钥已自动生成并保存到 .env"
        print_warning "请妥善保管 .env 文件，不要提交到版本控制系统"
        echo ""
    fi

    print_success "加密密钥检查完成"
    print_info "  • JWT_SECRET: OK"
    print_info "  • DATA_ENCRYPTION_KEY: OK"
    print_info "  • RSA_PRIVATE_KEY: OK"

    # 修复 .env 文件权限
    chmod 600 .env 2>/dev/null || true
}

# ------------------------------------------------------------------------
# Utility: Read Environment Variables
# ------------------------------------------------------------------------
read_env_vars() {
    if [ -f ".env" ]; then
        NOFX_FRONTEND_PORT=$(grep "^NOFX_FRONTEND_PORT=" .env 2>/dev/null | cut -d'=' -f2 || echo "3000")
        NOFX_BACKEND_PORT=$(grep "^NOFX_BACKEND_PORT=" .env 2>/dev/null | cut -d'=' -f2 || echo "8080")
        NOFX_SINGLE_USER=$(grep "^NOFX_SINGLE_USER=" .env 2>/dev/null | cut -d'=' -f2 || echo "false")

        NOFX_FRONTEND_PORT=$(echo "$NOFX_FRONTEND_PORT" | tr -d '"'"'" | tr -d ' ')
        NOFX_BACKEND_PORT=$(echo "$NOFX_BACKEND_PORT" | tr -d '"'"'" | tr -d ' ')
        NOFX_SINGLE_USER=$(echo "$NOFX_SINGLE_USER" | tr -d '"'"'" | tr -d ' ')

        NOFX_FRONTEND_PORT=${NOFX_FRONTEND_PORT:-3000}
        NOFX_BACKEND_PORT=${NOFX_BACKEND_PORT:-8080}
        NOFX_SINGLE_USER=${NOFX_SINGLE_USER:-false}
    else
        NOFX_FRONTEND_PORT=3000
        NOFX_BACKEND_PORT=8080
        NOFX_SINGLE_USER=false
    fi
}

# ------------------------------------------------------------------------
# Validation: Database Directory (data/)
# ------------------------------------------------------------------------
check_database() {
    # Ensure data directory exists
    if [ ! -d "data" ]; then
        print_warning "数据目录不存在，创建 data/ 目录..."
        install -m 700 -d data
        print_success "已创建 data/ 目录"
    else
        print_success "数据目录存在"
    fi
}

# ------------------------------------------------------------------------
# Service Management: Start
# ------------------------------------------------------------------------
start() {
    print_info "正在启动 NOFX AI Trading System..."

    read_env_vars
    setup_compose_args

    if [ ! -d "data" ]; then
        print_info "创建数据目录..."
        install -m 700 -d data
    fi

    if [ "$1" == "--build" ]; then
        print_info "重新构建镜像..."
        $COMPOSE_CMD "${COMPOSE_ARGS[@]}" up -d --build
    else
        print_info "启动容器..."
        $COMPOSE_CMD "${COMPOSE_ARGS[@]}" up -d
    fi

    print_success "服务已启动！"
    print_info "Web 界面: http://localhost:${NOFX_FRONTEND_PORT}"
    print_info "API 端点: http://localhost:${NOFX_BACKEND_PORT}"
    print_info ""
    print_info "查看日志: ./start.sh logs"
    print_info "停止服务: ./start.sh stop"
}

# ------------------------------------------------------------------------
# Service Management: Stop
# ------------------------------------------------------------------------
stop() {
    print_info "正在停止服务..."
    setup_compose_args
    $COMPOSE_CMD "${COMPOSE_ARGS[@]}" stop
    print_success "服务已停止"
}

# ------------------------------------------------------------------------
# Service Management: Restart
# ------------------------------------------------------------------------
restart() {
    print_info "正在重启服务..."
    setup_compose_args
    $COMPOSE_CMD "${COMPOSE_ARGS[@]}" restart
    print_success "服务已重启"
}

# ------------------------------------------------------------------------
# Service Management: Recreate (apply port/env changes)
# ------------------------------------------------------------------------
recreate() {
    print_info "正在重建容器（应用端口/环境变量变更）..."
    setup_compose_args
    $COMPOSE_CMD "${COMPOSE_ARGS[@]}" up -d --force-recreate
    print_success "容器已重建"
}

# ------------------------------------------------------------------------
# Monitoring: Logs
# ------------------------------------------------------------------------
logs() {
    setup_compose_args
    if [ -z "$2" ]; then
        $COMPOSE_CMD "${COMPOSE_ARGS[@]}" logs -f
    else
        $COMPOSE_CMD "${COMPOSE_ARGS[@]}" logs -f "$2"
    fi
}

# ------------------------------------------------------------------------
# Monitoring: Status
# ------------------------------------------------------------------------
status() {
    read_env_vars
    setup_compose_args

    print_info "服务状态:"
    $COMPOSE_CMD "${COMPOSE_ARGS[@]}" ps
    echo ""
    print_info "健康检查:"
    curl -s "http://localhost:${NOFX_BACKEND_PORT}/api/health" | jq '.' || echo "后端未响应"
}

# ------------------------------------------------------------------------
# Maintenance: Clean (Destructive)
# ------------------------------------------------------------------------
clean() {
    print_warning "这将删除所有容器和数据！"
    read -p "确认删除？(yes/no): " confirm
    if [ "$confirm" == "yes" ]; then
        print_info "正在清理..."
        setup_compose_args
        $COMPOSE_CMD "${COMPOSE_ARGS[@]}" down -v
        print_success "清理完成"
    else
        print_info "已取消"
    fi
}

# ------------------------------------------------------------------------
# Maintenance: Update
# ------------------------------------------------------------------------
update() {
    print_info "正在更新..."
    git pull
    setup_compose_args
    $COMPOSE_CMD "${COMPOSE_ARGS[@]}" up -d --build
    print_success "更新完成"
}

# ------------------------------------------------------------------------
# Single-user helpers: toggle and lock registration
# ------------------------------------------------------------------------
single_user_on() {
    check_env
    set_env_var "NOFX_SINGLE_USER" "true"
    set_env_var "NOFX_BIND_IP" "127.0.0.1"
    set_env_var "MAX_USERS" "1"
    set_env_var "EXPERIENCE_IMPROVEMENT" "false"
    # Keep registration enabled for first-time setup unless already disabled
    if ! grep -q "^REGISTRATION_ENABLED=" .env 2>/dev/null; then
        set_env_var "REGISTRATION_ENABLED" "true"
    fi
    chmod 600 .env 2>/dev/null || true
    print_success "已开启单人模式（NOFX_SINGLE_USER=true, NOFX_BIND_IP=127.0.0.1）。建议运行 ./start.sh recreate 以应用端口绑定变更"
}

single_user_off() {
    check_env
    set_env_var "NOFX_SINGLE_USER" "false"
    set_env_var "NOFX_BIND_IP" "0.0.0.0"
    chmod 600 .env 2>/dev/null || true
    print_success "已关闭单人模式（NOFX_SINGLE_USER=false, NOFX_BIND_IP=0.0.0.0）。建议运行 ./start.sh recreate 以应用端口绑定变更"
}

lock_registration() {
    check_env
    set_env_var "REGISTRATION_ENABLED" "false"
    set_env_var "MAX_USERS" "1"
    chmod 600 .env 2>/dev/null || true
    print_success "已锁定注册（REGISTRATION_ENABLED=false, MAX_USERS=1）。建议重启服务"
}

# ------------------------------------------------------------------------
# Maintenance: Backup/Restore SQLite DB (single-user)
# ------------------------------------------------------------------------
backup_db() {
    check_database

    local db_path="data/data.db"
    if [ ! -f "$db_path" ]; then
        print_error "未找到数据库文件: $db_path"
        exit 1
    fi

    local backup_dir="data/backups"
    install -m 700 -d "$backup_dir"

    local ts
    ts=$(date +"%Y%m%d-%H%M%S")
    local backup_path="${backup_dir}/data.db.${ts}.bak"

    cp "$db_path" "$backup_path"
    chmod 600 "$backup_path" 2>/dev/null || true
    print_success "已备份数据库: $backup_path"
}

restore_db() {
    local backup_path="$2"
    if [ -z "$backup_path" ]; then
        print_error "用法: ./start.sh restore-db <backup_file>"
        exit 1
    fi

    if [ ! -f "$backup_path" ]; then
        print_error "备份文件不存在: $backup_path"
        exit 1
    fi

    check_database
    setup_compose_args

    local db_path="data/data.db"
    local ts
    ts=$(date +"%Y%m%d-%H%M%S")
    local pre_restore="data/data.db.pre-restore.${ts}.bak"

    print_warning "即将从备份恢复数据库，将会覆盖当前 data/data.db"
    print_info "当前 DB 会先备份到: $pre_restore"

    # Stop services to avoid SQLite write conflicts.
    $COMPOSE_CMD "${COMPOSE_ARGS[@]}" stop || true

    if [ -f "$db_path" ]; then
        cp "$db_path" "$pre_restore"
        chmod 600 "$pre_restore" 2>/dev/null || true
    fi

    cp "$backup_path" "$db_path"
    chmod 600 "$db_path" 2>/dev/null || true

    print_success "数据库已恢复，正在重新启动服务..."
    $COMPOSE_CMD "${COMPOSE_ARGS[@]}" up -d
    print_success "恢复完成"
}

# ------------------------------------------------------------------------
# Command: Regenerate all keys (force)
# ------------------------------------------------------------------------
regenerate_keys() {
    print_warning "这将重新生成所有加密密钥！"
    print_warning "如果已有加密数据，重新生成后将无法解密！"
    echo ""
    read -p "确认重新生成？(yes/no): " confirm
    if [ "$confirm" != "yes" ]; then
        print_info "已取消"
        return
    fi

    check_env

    print_info "正在生成新的密钥..."

    # 生成 JWT_SECRET
    local jwt_secret=$(openssl rand -base64 32)
    set_env_var "JWT_SECRET" "$jwt_secret"
    print_success "JWT_SECRET 已生成"

    # 生成 DATA_ENCRYPTION_KEY
    local data_key=$(openssl rand -base64 32)
    set_env_var "DATA_ENCRYPTION_KEY" "$data_key"
    print_success "DATA_ENCRYPTION_KEY 已生成"

    # 生成 RSA_PRIVATE_KEY
    local rsa_key=$(openssl genrsa 2048 2>/dev/null | awk '{printf "%s\\n", $0}')
    set_env_var "RSA_PRIVATE_KEY" "\"$rsa_key\""
    print_success "RSA_PRIVATE_KEY 已生成"

    chmod 600 .env 2>/dev/null || true

    echo ""
    print_success "所有密钥已重新生成并保存到 .env"
    print_warning "请妥善保管 .env 文件"
}

# ------------------------------------------------------------------------
# Help: Usage Information
# ------------------------------------------------------------------------
show_help() {
    echo "NOFX AI Trading System - Docker 管理脚本"
    echo ""
    echo "用法: ./start.sh [command] [options]"
    echo ""
    echo "命令:"
    echo "  start [--build]    启动服务（可选：重新构建）"
    echo "  stop               停止服务"
    echo "  restart            重启服务"
    echo "  recreate           重建容器（应用端口/环境变量变更）"
    echo "  logs [service]     查看日志（可选：指定服务名 backend/frontend）"
    echo "  status             查看服务状态"
    echo "  clean              清理所有容器和数据"
    echo "  update             更新代码并重启"
    echo "  backup-db          备份 data/data.db 到 data/backups/"
    echo "  restore-db <file>  从备份恢复 data/data.db（会先自动备份当前 DB）"
    echo "  regenerate-keys    重新生成所有加密密钥（慎用）"
    echo "  single-user-on     开启单人模式（NOFX_BIND_IP=127.0.0.1 仅本机访问）"
    echo "  single-user-off    关闭单人模式"
    echo "  lock-registration  关闭注册入口（首次注册后建议执行）"
    echo "  help               显示此帮助信息"
    echo ""
    echo "示例:"
    echo "  ./start.sh start --build    # 构建并启动"
    echo "  ./start.sh logs backend     # 查看后端日志"
    echo "  ./start.sh status           # 查看状态"
    echo ""
    echo "首次使用:"
    echo "  直接运行 ./start.sh 即可，缺失的密钥会自动生成"
}

# ------------------------------------------------------------------------
# Main: Command Dispatcher
# ------------------------------------------------------------------------
main() {
    check_docker
    setup_compose_args

    case "${1:-start}" in
        start)
            check_env
            check_encryption
            check_database
            start "$2"
            ;;
        backup-db)
            backup_db
            ;;
        restore-db)
            restore_db "$@"
            ;;
        single-user-on)
            single_user_on
            ;;
        single-user-off)
            single_user_off
            ;;
        lock-registration)
            lock_registration
            ;;
        stop)
            stop
            ;;
        restart)
            restart
            ;;
        recreate)
            recreate
            ;;
        logs)
            logs "$@"
            ;;
        status)
            status
            ;;
        clean)
            clean
            ;;
        update)
            update
            ;;
        regenerate-keys)
            regenerate_keys
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            print_error "未知命令: $1"
            show_help
            exit 1
            ;;
    esac
}

# Execute Main
main "$@"
