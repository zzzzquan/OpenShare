<p align="center">
  <img src="assets/banner.svg" alt="OpenShare Banner" width="100%">
</p>

<p align="center">
面向校园、实验室、社团、研究组等中小型组织的资料管理与共享平台 📚
</p>

## 项目简介 ✨

OpenShare 是一个面向中小型组织内网环境的资料分享平台，适用于校园课程资料、实验室文档、社团资源、组内文件等场景。它部署简单、依赖少，具备完整的资料共建、资料治理和资料分发能力！

项目借鉴了 `OpenList`、`FileBrowser` 等工具在文件浏览和目录管理上的思路，并且在“内网资料分享”这一场景做了大量功能优化，让内网环境下资料的 共建 & 共享 真正成为可能！🚀

## 项目背景 ☝🏻

一直以来，我们学校的学习资料共享氛围并不浓厚（即使是在计院也如此）。更甚者，有的人直接拿着GitHub上学长的资料仓库在树洞里卖，有的人拿着陈年老题在新生里引流来带货……

基于此现状，我们学校一个主打分享交流的社群倡议建立一个像浙江大学课程攻略共享计划资料共享平台。机缘巧合之下，我有幸成为了项目负责人之一。

最开始，我们的运行模式是这样的：校内服务器，仅提供内网服务，设三个站点：Openlist供同学们浏览和下载文件；filebrowser用来收集同学们的资料；此外，还有一位同校的学长用java开发了一个站点，用于搜索与批量下载。

年后，我便开始规划新平台，希望能够融合上述三个站点的功能。我借鉴了 `OpenList`、`FileBrowser` 等工具在文件浏览和目录管理上的思路，并且在“内网资料分享平台”这一场景做了大量功能优化（主要聚焦于“平台化”这一点，比如公告系统、管理系统、回执系统等），让内网环境下资料的 共建 & 共享 真正成为了可能！

当然，我不希望这个项目只停留在我们校内。我更希望它能够走向更多学校，让每个校园都能建立起自己的资料分享平台(当然除了校园，实验室、社团、课题组等场景也很适合契合本项目)。让资料在共享中持续流动、被看见、被延续。

## 核心特性 🌟

### 1. 简约、现代的前端界面

- 首页与管理后台统一采用简洁、现代的界面风格
- 支持卡片 / 表格等多种展示方式

<p align="center">
  <img src="assets/1-1.png" alt="前端界面 1" width="49%">
  <img src="assets/1-2.png" alt="前端界面 2" width="49%">
</p>

### 2. 普通用户使用体验优化

- 免登录即可使用，包括 **浏览、搜索、下载、上传与反馈**
- 支持 Markdown 语法
- 首页集成 **公告、热门下载、资料上新** 等信息面板
- 支持单文件下载与批量下载
- 支持目录内搜索
- 文件详情信息完整，覆盖名称、大小、下载量、更新时间、所属目录等内容

<p align="center">
  <img src="assets/2-1.png" alt="普通用户体验 1" width="49%">
  <img src="assets/2-2.png" alt="普通用户体验 2" width="49%">
</p>

<p align="center">
  <img src="assets/2-3.png" alt="普通用户体验 3" width="49%">
  <img src="assets/2-4.png" alt="普通用户体验 4" width="49%">
</p>

### 3. 后台治理与权限管理优化

- 管理员分为超级管理员和普通管理员，启动时会自动生成超级管理员初始密码
- 管理后台提供控制台、审核、公告、日志、账号设置等页面
- 支持修改账号信息，包括头像、用户名和密码
以下为超级管理员特有权限
- 配置访客策略、设置上传限制、导入本地目录
- 管理员创建、停用、删除、重置密码与权限分配

<p align="center">
  <img src="assets/3-1.png" alt="后台治理 1" width="49%">
  <img src="assets/3-2.png" alt="后台治理 2" width="49%">
</p>

### 4. 资料共建共享

- 支持普通用户上传资料、提交反馈
- 通过回执查询处理状态

<p align="center">
  <img src="assets/4-1.png" alt="资料共建共享 1" width="49%">
  <img src="assets/4-2.png" alt="资料共建共享 2" width="49%">
</p>

## 项目结构 🧩

```text
OpenShare/
├── assets/                     README 配图与静态资源
├── backend/                    Go 后端服务
│   ├── cmd/server/             服务入口
│   ├── configs/                默认配置与本地配置样例
│   ├── internal/               路由、服务、仓储、模型等核心实现
│   └── web/                    嵌入式前端构建产物
├── docker/                     Linux 构建镜像文件
├── frontend/                   Vue 前端项目
├── release/                    打包输出目录
└── scripts/                    开发与构建脚本
```

## 快速开始 ⚡

### 方法一：本地源码启动

环境要求：

- Go 1.25+
- Node.js / npm

在项目根目录执行下面的脚本即可：

本脚本：

- 不会清空已有数据库和存储目录
- 不会覆盖已存在的 `backend/configs/config.local.json`
- 第一次启动时会自动初始化数据库，并输出超级管理员初始凭据

```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(pwd)"
LOCAL_DATA_DIR="$ROOT_DIR/.localdata"
LOG_DIR="$LOCAL_DATA_DIR/logs"
BACKEND_LOG="$LOG_DIR/backend.log"
FRONTEND_LOG="$LOG_DIR/frontend.log"
BACKEND_CONFIG_LOCAL="$ROOT_DIR/backend/configs/config.local.json"

mkdir -p "$LOG_DIR"

if [ ! -f "$BACKEND_CONFIG_LOCAL" ]; then
  echo "==> 创建本地配置"
  cat > "$BACKEND_CONFIG_LOCAL" <<EOF
{
  "database": {
    "path": "$LOCAL_DATA_DIR/openshare.db"
  },
  "storage": {
    "root": "$LOCAL_DATA_DIR"
  },
  "session": {
    "secret": "dev-local-session-secret"
  }
}
EOF
else
  echo "==> 使用现有本地配置"
fi

echo "==> 安装前端依赖"
cd "$ROOT_DIR/frontend"
npm install > "$FRONTEND_LOG" 2>&1

echo "==> 启动前端开发服务器"
npm run dev -- --host 127.0.0.1 > "$FRONTEND_LOG" 2>&1 &
FRONTEND_PID=$!

echo "==> 启动后端服务"
cd "$ROOT_DIR/backend"
go run ./cmd/server > "$BACKEND_LOG" 2>&1 &
BACKEND_PID=$!

echo
echo "OpenShare 已启动"
echo "Public: http://localhost:5173/"
echo "Admin : http://localhost:5173/admin"
echo "Health: http://127.0.0.1:8080/healthz"
echo "Logs  : $LOG_DIR"
echo

attempts=30
for ((i = 1; i <= attempts; i++)); do
  if [[ -f "$BACKEND_LOG" ]]; then
    line="$(grep -E '\[bootstrap\] super admin initialized; username=.* password=.*' "$BACKEND_LOG" | tail -n 1 || true)"
    if [[ -n "$line" ]]; then
      echo
      echo "超级管理员初始凭据："
      echo "$line"
      echo
      break
    fi
  fi
  sleep 1
done

echo "按 Ctrl+C 停止服务"

trap 'kill $FRONTEND_PID $BACKEND_PID 2>/dev/null' EXIT
wait
```

默认访问地址：

- Public: `http://localhost:5173/`
- Admin: `http://localhost:5173/admin`
- API Health: `http://127.0.0.1:8080/healthz`

### 方法二：二进制文件启动

1. 从仓库的 Releases 页面下载 linux-amd64 平台的压缩包
2. 根据需求修改 `configs/config.local.json`
3. 运行发布包中的 `start.sh`

目录结构：

```text
openshare-1.0.0-linux-amd64/
├── openshare
├── start.sh
└── configs/
```

启动：

```bash
chmod +x start.sh
./start.sh
```

默认情况下，服务会自动初始化数据库、存储目录、搜索索引，并在首次启动时创建超级管理员。

## 致谢 🫶

如果这个项目对你有帮助，欢迎点一个 Star ！

<div align="center">
  <p><strong>感谢LinuxDo社区的支持！</strong></p>
  <p>
    <a href="https://linux.do" target="_blank" rel="noopener noreferrer">
      <img
        alt="LinuxDo Community"
        src="https://img.shields.io/badge/COMMUNITY-LINUXDO-3E7FC1?style=for-the-badge&labelColor=5A5A5A&logoWidth=0"
      />
    </a>
  </p>
</div>
