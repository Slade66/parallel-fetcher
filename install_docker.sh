#!/bin/bash

# 当任何命令失败时立即退出脚本
set -e

# --- 脚本开始 ---

echo "--- [步骤 1/5] 更新软件包列表并安装基础依赖 ---"
# 使用 sudo 来获取管理员权限
sudo apt-get update
sudo apt-get install -y ca-certificates curl gnupg

echo "--- [步骤 2/5] 添加 Docker 官方 GPG 密钥 ---"
# 创建用于存放密钥的目录
sudo install -m 0755 -d /etc/apt/keyrings
# 下载并添加 Docker 的 GPG 密钥
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
# 设置密钥文件的权限
sudo chmod a+r /etc/apt/keyrings/docker.gpg

echo "--- [步骤 3/5] 设置 Docker 的 APT 软件源 ---"
# 将 Docker 软件源添加到系统的源列表中
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

echo "--- [步骤 4/5] 安装 Docker 引擎和 Docker Compose ---"
# 再次更新软件包列表以包含新的 Docker 源
sudo apt-get update
# 安装 Docker 相关的所有组件
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

echo "--- [步骤 5/5] 配置非 root 用户权限 ---"
# 检查 docker 用户组是否存在，不存在则创建
if ! getent group docker > /dev/null; then
    sudo groupadd docker
fi
# 将当前执行 sudo 的用户添加到 docker 组
# 使用 $SUDO_USER 可以确保添加到的是执行 sudo 的那个用户，而不是 root
if [ -n "$SUDO_USER" ]; then
    sudo usermod -aG docker "$SUDO_USER"
    echo "已将用户 '$SUDO_USER' 添加到 'docker' 组。"
else
    echo "警告：无法确定原始用户信息，请手动执行 'sudo usermod -aG docker \$USER'"
fi

echo ""
echo "✅ Docker 和 Docker Compose 安装成功！"
echo ""
echo "‼️ 重要提示：请完全退出当前 SSH 会话并重新登录，"
echo "   这样用户组权限的变更才会生效，你才能无需 sudo 直接运行 docker 命令。"
echo ""

# 打印版本信息以供验证
echo "--- 版本信息 ---"
docker --version
docker compose version
echo "----------------"