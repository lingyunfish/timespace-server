#!/bin/bash
# ============================================================================
# timespace 服务启动脚本
# 用法:
#   ./start.sh           # 启动（已运行则重启）
#   ./start.sh start     # 启动
#   ./start.sh stop      # 停止
#   ./start.sh restart   # 重启
#   ./start.sh status    # 查看状态
#   ./start.sh tail      # 实时查看日志
# ============================================================================

set -e

# ---------- 配置 ----------
APP_NAME="timespace"
# 脚本所在目录就是工作目录，避免依赖执行时的 cwd
APP_DIR="$(cd "$(dirname "$0")" && pwd)"
APP_BIN="$APP_DIR/$APP_NAME"
ENV_FILE="$APP_DIR/.env"
LOG_DIR="$APP_DIR/logs"
LOG_FILE="$LOG_DIR/stdout.log"
PID_FILE="$APP_DIR/$APP_NAME.pid"

# 进程优雅停止等待秒数
STOP_TIMEOUT=10

# ---------- 颜色输出 ----------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'
log_info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }

# ---------- 工具函数 ----------

# 获取真正在跑的 PID（先看 pid 文件，再用 pgrep 兜底）
get_pid() {
    local pid=""
    if [ -f "$PID_FILE" ]; then
        pid=$(cat "$PID_FILE" 2>/dev/null)
        # 检查这个 PID 是不是还活着、且确实是我们的程序
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            if ps -p "$pid" -o comm= 2>/dev/null | grep -q "^$APP_NAME$"; then
                echo "$pid"
                return
            fi
        fi
        # PID 文件过期，清理
        rm -f "$PID_FILE"
    fi

    # PID 文件不可信时，用 pgrep 按程序绝对路径匹配
    pid=$(pgrep -f "^$APP_BIN" | head -n1)
    if [ -n "$pid" ]; then
        echo "$pid"
    fi
}

# 加载环境变量（从 .env）
load_env() {
    if [ ! -f "$ENV_FILE" ]; then
        log_warn ".env 文件不存在: $ENV_FILE"
        log_warn "将使用 trpc_go.yaml 中的默认值（敏感配置可能为空）"
        return
    fi

    log_info "加载环境变量: $ENV_FILE"
    # 自动 export 文件中的所有变量
    set -a
    # shellcheck disable=SC1090
    source "$ENV_FILE"
    set +a
}

# ---------- 主命令 ----------

cmd_start() {
    local pid
    pid=$(get_pid)
    if [ -n "$pid" ]; then
        log_warn "$APP_NAME 已经在运行 (PID: $pid)"
        log_warn "如需重启请使用: $0 restart"
        return 0
    fi

    # 检查二进制
    if [ ! -x "$APP_BIN" ]; then
        log_error "二进制文件不存在或不可执行: $APP_BIN"
        log_error "请先 go build 编译"
        exit 1
    fi

    # 准备日志目录
    mkdir -p "$LOG_DIR"

    # 加载环境变量
    load_env

    log_info "启动 $APP_NAME ..."
    log_info "工作目录: $APP_DIR"
    log_info "二进制:   $APP_BIN"
    log_info "日志文件: $LOG_FILE"

    # 切到工作目录启动（trpc_go.yaml、uploads/ 都用相对路径）
    cd "$APP_DIR"

    # 启动：nohup + setsid 双重保险，完全脱离终端
    nohup "$APP_BIN" >> "$LOG_FILE" 2>&1 &
    local new_pid=$!
    echo "$new_pid" > "$PID_FILE"
    disown 2>/dev/null || true

    # 等待 2 秒，确认进程没立刻挂掉
    sleep 2
    if kill -0 "$new_pid" 2>/dev/null; then
        log_info "$APP_NAME 启动成功 (PID: $new_pid)"
        log_info "查看日志: tail -f $LOG_FILE"
    else
        log_error "$APP_NAME 启动失败！最后 30 行日志："
        echo "----------------------------------------"
        tail -n 30 "$LOG_FILE" 2>/dev/null || true
        echo "----------------------------------------"
        rm -f "$PID_FILE"
        exit 1
    fi
}

cmd_stop() {
    local pid
    pid=$(get_pid)
    if [ -z "$pid" ]; then
        log_warn "$APP_NAME 未运行"
        return 0
    fi

    log_info "正在停止 $APP_NAME (PID: $pid) ..."
    # 先发送 SIGTERM 优雅停止
    kill -TERM "$pid" 2>/dev/null || true

    # 等待最多 STOP_TIMEOUT 秒
    local i=0
    while [ $i -lt $STOP_TIMEOUT ]; do
        if ! kill -0 "$pid" 2>/dev/null; then
            log_info "$APP_NAME 已停止"
            rm -f "$PID_FILE"
            return 0
        fi
        sleep 1
        i=$((i + 1))
    done

    # 超时后强杀
    log_warn "优雅停止超时，强制杀死进程..."
    kill -KILL "$pid" 2>/dev/null || true
    sleep 1
    if kill -0 "$pid" 2>/dev/null; then
        log_error "无法杀死进程 $pid"
        exit 1
    fi
    log_info "$APP_NAME 已强制停止"
    rm -f "$PID_FILE"
}

cmd_restart() {
    log_info "===== 重启 $APP_NAME ====="
    cmd_stop
    sleep 1
    cmd_start
}

cmd_status() {
    local pid
    pid=$(get_pid)
    if [ -n "$pid" ]; then
        log_info "$APP_NAME 正在运行 (PID: $pid)"
        # 显示资源占用
        echo ""
        ps -p "$pid" -o pid,ppid,user,%cpu,%mem,etime,cmd 2>/dev/null || true
    else
        log_warn "$APP_NAME 未运行"
        exit 1
    fi
}

cmd_tail() {
    if [ ! -f "$LOG_FILE" ]; then
        log_error "日志文件不存在: $LOG_FILE"
        exit 1
    fi
    tail -f "$LOG_FILE"
}

cmd_help() {
    cat <<EOF
$APP_NAME 服务启动脚本

用法:
    $0 [命令]

命令:
    start      启动服务（已运行则跳过）
    stop       停止服务
    restart    重启服务（默认行为）
    status     查看服务状态
    tail       实时查看日志
    help       显示帮助

文件路径:
    工作目录:  $APP_DIR
    二进制:    $APP_BIN
    环境变量:  $ENV_FILE
    日志文件:  $LOG_FILE
    PID 文件:  $PID_FILE
EOF
}

# ---------- 入口 ----------

case "${1:-restart}" in
    start)   cmd_start ;;
    stop)    cmd_stop ;;
    restart) cmd_restart ;;
    status)  cmd_status ;;
    tail)    cmd_tail ;;
    help|-h|--help) cmd_help ;;
    *)
        log_error "未知命令: $1"
        cmd_help
        exit 1
        ;;
esac
