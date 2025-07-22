# ZDocker
自己动手写 Docker, 想深入的了解容器技术, 以及自己实现一个简易版本, 后续可以运用在评测机中

项目启发于 [自己动手写 Docker](https://github.com/xianlubird/mydocker)。

# 项目介绍
ZDocker 是一个用 Go 语言实现的简易版 Docker 容器引擎，旨在深入理解容器技术的核心原理。通过实现 Linux 命名空间隔离、cgroups 资源限制、镜像管理、网络配置等功能，本项目提供了一个学习容器技术的实践平台。

## 主要功能
- 基于 Linux Namespace 实现容器隔离
- 使用 cgroups 进行资源限制和管理
- 容器镜像的构建与管理
- 容器网络配置
- 简易容器运行时实现

## 技术栈
- Go 语言
- Linux 系统编程
- Namespace 隔离技术
- Cgroups 资源控制
- 网络虚拟化
- 文件系统管理

# 使用方法
## 安装
```bash
git clone https://github.com/crazyfrankie/zdocker.git
cd zdocker
go build -o zdocker
```

## 基本命令
```bash
# 运行容器
./zdocker run -t [image] [command]

# 查看运行中的容器
./zdocker ps

# 查看帮助
./zdocker --help
```

# Progress
## 1. 基础知识入门
- [x] 看原书第二章
- [x] [动手实现书中示例](./learning/chapter2)
- [x] 看原书第三章
- [x] 动手实现容器
- [x] 看原书第四章
- [x] 实现构造镜像
- [x] 看原书第五章
- [x] 实现构建容器进阶
- [x] 看原书第六章
- [x] 实现容器网络
- [x] 看原书第七章

## 2. 功能实现
- [x] 基础容器运行时
- [x] 资源限制 (cgroups)
- [x] 容器网络配置
- [x] 数据卷 (volume) 支持
- [ ] 容器镜像管理
- [ ] 容器编排简易实现
- [ ] 容器安全加固

## 3. 未来计划
- [ ] 优化容器启动性能
- [ ] 实现简易的容器编排系统
- [ ] 支持更多网络模式
- [ ] 完善镜像构建功能
- [ ] 集成到评测机系统

# 项目结构
```
zdocker/
├── cmd/            # 命令行入口
├── container/      # 容器核心实现
├── network/        # 网络相关功能
├── cgroups/        # 资源限制实现
├── image/          # 镜像管理
├── volume/         # 数据卷管理
├── learning/       # 学习笔记和示例
└── docs/           # 文档
```

# 参考资料
- [《自己动手写Docker》](https://github.com/xianlubird/mydocker)
- [Docker 官方文档](https://docs.docker.com/)
- [Linux Namespace 文档](https://man7.org/linux/man-pages/man7/namespaces.7.html)
- [Cgroups 文档](https://www.kernel.org/doc/Documentation/cgroup-v1/cgroups.txt)
