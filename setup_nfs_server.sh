#!/bin/bash

# 当任何命令失败时立即退出
set -e

# --- 配置 ---
# 共享目录的路径
SHARED_DIR="/mnt/shared_downloads"
# --- End of 配置 ---

# 检查脚本是否以 root 权限运行
if [[ $EUID -ne 0 ]]; then
   echo "错误：请使用 sudo 运行此脚本"
   exit 1
fi

# 检查是否提供了客户端 IP 地址作为参数
if [ -z "$1" ] || [ -z "$2" ]; then
    echo "用法: sudo $0 <机器B的IP> <机器C的IP>"
    exit 1
fi

CLIENT_IP_B="$1"
CLIENT_IP_C="$2"

echo "--- [步骤 1/5] 更新软件包列表 ---"
apt-get update -y

echo "--- [步骤 2/5] 安装 NFS 服务端 ---"
apt-get install nfs-kernel-server -y

echo "--- [步骤 3/5] 创建并设置共享目录 (${SHARED_DIR}) ---"
mkdir -p "$SHARED_DIR"
chown nobody:nogroup "$SHARED_DIR"
chmod 777 "$SHARED_DIR"
echo "目录创建并设置权限完成。"

echo "--- [步骤 4/5] 配置 /etc/exports 文件 ---"
# 要添加到 exports 文件的行
EXPORT_LINE="${SHARED_DIR}    ${CLIENT_IP_B}(rw,sync,no_subtree_check) ${CLIENT_IP_C}(rw,sync,no_subtree_check)"

# 检查配置是否已存在，避免重复添加
if grep -qF "${SHARED_DIR}" /etc/exports; then
    echo "NFS 导出配置已存在，跳过添加。"
else
    echo "添加新的 NFS 导出配置..."
    # 将配置行追加到文件末尾
    echo "$EXPORT_LINE" >> /etc/exports
    echo "配置已添加。"
fi

echo "--- [步骤 5/5] 应用配置并重启 NFS 服务 ---"
exportfs -a
systemctl restart nfs-kernel-server
echo "NFS 服务已重启并加载新配置。"

echo ""
echo "✅ NFS 服务端配置完成！"
echo "现在你可以在机器 B 和 C 上运行客户端设置脚本了。"