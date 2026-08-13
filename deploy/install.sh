#!/usr/bin/env bash
# EasyPub Linux 服务器一键安装脚本
#
# 用法:
#   sudo ./deploy/install.sh [路径/到/easypub-linux-amd64] [监听地址]
#
# 默认行为:
#   - 二进制安装到 /opt/easypub/easypub
#   - 创建专用用户 easypub (无登录权限)
#   - 安装 systemd 服务 easypub (默认监听 127.0.0.1:8080)
#   - 目录结构:
#       /opt/easypub/easypub        二进制
#       /opt/easypub/config.xml     可选配置(原 EasyPub 兼容)
#       /opt/easypub/.easypub-output  WebUI 输出目录
#
# 安装后:
#   - 编辑 /etc/systemd/system/easypub.service 里的 EASYPUB_TOKEN 设置访问令牌
#   - systemctl restart easypub
set -euo pipefail

BIN_SRC="${1:-dist/easypub-linux-amd64}"
ADDR="${2:-127.0.0.1:8080}"
INSTALL_DIR="/opt/easypub"
SERVICE_NAME="easypub"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"

if [[ $EUID -ne 0 ]]; then
  echo "错误: 请用 root 或 sudo 运行" >&2
  exit 1
fi

if [[ ! -f "${BIN_SRC}" ]]; then
  echo "错误: 未找到二进制 ${BIN_SRC}" >&2
  echo "请先构建: make linux  (产物在 dist/easypub-linux-amd64)" >&2
  exit 1
fi

echo "==> 创建目录 ${INSTALL_DIR}"
mkdir -p "${INSTALL_DIR}"

echo "==> 创建专用用户 easypub"
if ! id -u easypub >/dev/null 2>&1; then
  useradd --system --no-create-home --home-dir "${INSTALL_DIR}" --shell /usr/sbin/nologin easypub
fi

echo "==> 安装二进制"
install -o easypub -g easypub -m 0755 "${BIN_SRC}" "${INSTALL_DIR}/easypub"

echo "==> 安装 systemd 服务"
sed "s|-addr 127.0.0.1:8080|-addr ${ADDR}|" deploy/easypub.service > "${SERVICE_FILE}"

echo "==> 权限"
chown -R easypub:easypub "${INSTALL_DIR}"
chmod 700 "${INSTALL_DIR}"

echo "==> 启动服务"
systemctl daemon-reload
systemctl enable "${SERVICE_NAME}"
systemctl restart "${SERVICE_NAME}"

echo ""
echo "==> 完成。服务信息:"
systemctl --no-pager status "${SERVICE_NAME}" | head -n 8 || true
echo ""
echo "下一步:"
echo "  1. 设置访问令牌:"
echo "     sed -i 's/^Environment=EASYPUB_TOKEN=.*/Environment=EASYPUB_TOKEN='$(openssl rand -hex 24)'/' ${SERVICE_FILE}"
echo "     systemctl daemon-reload && systemctl restart ${SERVICE_NAME}"
echo "  2. 查看日志: journalctl -u ${SERVICE_NAME} -f"
echo "  3. 可选: 把 config.xml 放到 ${INSTALL_DIR}/config.xml 以使用自定义排版/分章配置"
