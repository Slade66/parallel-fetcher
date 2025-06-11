
### 架构:

* 机器 A: API 服务 + Worker 服务 + Redis + NFS 服务端
* 机器 B: Worker 服务 + NFS 客户端
* 机器 C: Worker 服务 + NFS 客户端

### 阶段一：环境准备 - 配置NFS共享存储

#### 在机器 A 上 (配置NFS服务端):

* chmod +x setup_nfs_server.sh
* sudo ./setup_nfs_server.sh <机器B的IP> <机器C的IP>

#### 在机器 B 和 C 上 (配置NFS客户端):

* chmod +x setup_nfs_client.sh
* sudo ./setup_nfs_client.sh <机器A的IP>

### 阶段二：代码与配置分发

你需要将你的整个项目文件夹（包含所有 .go 源码、go.mod、Dockerfile 以及我们下面将要确定的两个 docker-compose 文件）完整地复制到你的三台机器上。

### 阶段三：最终的 docker-compose 文件 (就地构建版)



### 阶段四：启动与验证

在机器 A 上 (进入项目目录后执行): `docker-compose -f docker-compose.A.yml up --build -d`

在机器 B 和 C 上 (分别进入项目目录后执行): `docker-compose -f docker-compose.BC.yml up --build -d`

