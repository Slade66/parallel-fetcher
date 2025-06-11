#!/bin/bash

# 当任何命令失败时立即退出
set -e

# --- 配置 ---
# 本地挂载点
MOUNT_POINT="/data/downloads"
# NFS 服务端共享的目录
REMOTE_DIR="/mnt/shared_downloads"
# --- End of 配置 ---

# 检查脚本是否以 root 权限运行
if [[ $EUID -ne 0 ]]; then
   echo "错误：请使用 sudo 运行此脚本"
   exit 1
fi

# 检查是否提供了服务端 IP 地址作为参数
if [ -z "$1" ]; then
    echo "用法: sudo $0 <机器A的IP>"
    exit 1
fi

SERVER_IP="$1"
REMOTE_PATH="${SERVER_IP}:${REMOTE_DIR}"

echo "--- [步骤 1/4] 更新软件包列表 ---"
apt-get update -y

echo "--- [步骤 2/4] 安装 NFS 客户端 ---"
apt-get install nfs-common -y

echo "--- [步骤 3/4] 创建本地挂载点 (${MOUNT_POINT}) ---"
mkdir -p "$MOUNT_POINT"
echo "挂载点创建完成。"

echo "--- [步骤 4/4] 挂载网络文件系统并设置开机自启 ---"

# 检查是否已经挂载
if mountpoint -q "$MOUNT_POINT"; then
    echo "目录 '${MOUNT_POINT}' 已经挂载，跳过。"
else
    echo "正在挂载 ${REMOTE_PATH} 到 ${MOUNT_POINT}..."
    mount "$REMOTE_PATH" "$MOUNT_POINT"
    echo "挂载成功。"
fi

# 检查 /etc/fstab 中是否已存在配置，避免重复添加
FSTAB_LINE="${REMOTE_PATH}   ${MOUNT_POINT}   nfs   defaults   0   0"
if grep -qF "$REMOTE_PATH" /etc/fstab; then
    echo "fstab 自动挂载配置已存在，跳过。"
else
    echo "添加开机自动挂载配置到 /etc/fstab..."
    echo "$FSTAB_LINE" >> /etc/fstab
    echo "配置已添加。"
fi

echo ""
echo "✅ NFS 客户端配置完成！"
echo "共享目录现在位于 ${MOUNT_POINT}。"
df -h "$MOUNT_POINT"