# ZDocker 网络实现详解

主要组件：
1. 网络管理器 - 负责网络的创建、删除和容器连接
2. Bridge 驱动 - 实现基于 Linux Bridge 的网络驱动
3. IP 地址管理器 (IPAM) - 使用位图算法管理 IP 分配
4. 测试模块 - 验证 IP 分配功能

核心技术：
• **Network Namespace** - 实现网络隔离
• **Veth Pair** - 连接不同网络命名空间
• **Linux Bridge** - 作为虚拟交换机连接容器
• **iptables NAT** - 实现容器访问外网和端口映射

实现亮点：
• 完整的 IP 地址分配和回收机制
• 支持容器间通信和外网访问
• 网络配置持久化
• 灵活的网络驱动架构

## 概述

ZDocker 的网络实现基于 Linux 的网络命名空间（Network Namespace）、虚拟网络设备（Veth Pair）和 Linux Bridge 技术，实现了容器间的网络隔离和通信。
整个网络架构包含四个核心组件：

1. **网络管理器** (`network.go`) - 网络的创建、删除、连接管理
2. **Bridge 驱动** (`bridge.go`) - Linux Bridge 网络驱动实现
3. **IP 地址管理** (`ipam.go`) - IP 地址分配和回收

## 核心数据结构

### Network 结构体
```go
type Network struct {
    Name    string     `json:"name"`    // 网络名称
    IpRange *net.IPNet `json:"ipRange"` // 网络段
    Driver  string     `json:"driver"`  // 网络驱动名称
}
```

### Endpoint 结构体
```go
type Endpoint struct {
    ID          string           `json:"id"`
    Device      netlink.Veth     `json:"device"`      // Veth 设备对
    IPAddress   net.IP           `json:"ip"`          // 分配的 IP 地址
    MacAddress  net.HardwareAddr `json:"mac"`         // MAC 地址
    PortMapping []string         `json:"portMapping"` // 端口映射
    Network     *Network
}
```

## 1. 网络管理器 (network.go)

### 初始化流程

```go
func InitNetwork() error
```

- 注册 Bridge 网络驱动到全局驱动映射
- 创建网络配置存储目录 `/var/run/zdocker/network/network/`
- 加载已存在的网络配置文件

### 网络创建

```go
func CreateNetwork(driver string, subnet string, name string) error
```

**核心步骤：**
1. 解析子网 CIDR 格式
2. 通过 IPAM 分配网关 IP（子网第一个可用 IP）
3. 调用对应驱动创建网络
4. 持久化网络配置到文件系统

### 容器网络连接

```go
func Connect(networkName string, cinfo *container.ContainerInfo) error
```

**连接流程：**
1. **IP 分配**：通过 IPAM 为容器分配 IP 地址
2. **创建 Endpoint**：包含容器 ID、IP、网络信息
3. **驱动连接**：调用网络驱动创建 Veth 对并连接到 Bridge
4. **容器内配置**：配置容器内网络设备和路由
5. **端口映射**：配置 iptables NAT 规则

### 容器网络空间配置

```go
func configEndpointIpAddressAndRoute(ep *Endpoint, cinfo *container.ContainerInfo) error
```

**关键操作：**
- 将 Veth 的一端移动到容器网络命名空间
- 配置容器内 Veth 设备的 IP 地址
- 启用容器内的网络设备（包括 lo 接口）
- 添加默认路由，所有外部流量通过网关转发

### 网络命名空间切换

```go
func enterContainerNetns(enLink *netlink.Link, cinfo *container.ContainerInfo) func()
```

**实现细节：**
- 打开容器的网络命名空间文件 `/proc/{pid}/ns/net`
- 锁定当前 OS 线程防止 goroutine 调度
- 将 Veth 设备移动到容器命名空间
- 切换到容器网络命名空间执行配置
- 返回恢复函数用于切换回原命名空间

## 2. Bridge 网络驱动 (bridge.go)

### 驱动接口实现

```go
type BridgeNetworkDriver struct{}
```

实现了 `NetworkDriver` 接口的四个方法：
- `Create()` - 创建 Bridge 网络
- `Delete()` - 删除 Bridge 网络
- `Connect()` - 连接容器到网络
- `Disconnect()` - 断开容器连接

### Bridge 网络创建

```go
func (b *BridgeNetworkDriver) Create(subnet string, name string) (*Network, error)
```

**创建步骤：**
1. 解析子网信息
2. 创建 Network 对象
3. 初始化 Bridge 设备

### Bridge 初始化

```go
func (b *BridgeNetworkDriver) initBridge(n *Network) error
```

**初始化流程：**
1. **创建 Bridge 设备**：使用 netlink 创建 Linux Bridge
2. **配置 IP 地址**：为 Bridge 分配网关 IP
3. **启用设备**：将 Bridge 设置为 UP 状态
4. **配置 NAT 规则**：设置 iptables MASQUERADE 规则

### 容器连接到 Bridge

```go
func (b *BridgeNetworkDriver) Connect(network *Network, endpoint *Endpoint) error
```

**连接过程：**
1. 获取 Bridge 设备
2. 创建 Veth 对：
   - 一端连接到 Bridge（通过 MasterIndex）
   - 另一端将移动到容器命名空间
3. 启用 Veth 设备

### iptables NAT 配置

```go
func setupIPTables(bridgeName string, subnet *net.IPNet) error
```

配置 MASQUERADE 规则：
```bash
iptables -t nat -A POSTROUTING -s <subnet> ! -o <bridgeName> -j MASQUERADE
```

这确保容器可以访问外部网络。

## 3. IP 地址管理 (ipam.go)

### IPAM 结构体

```go
type IPAM struct {
    SubnetAllocatorPath string             // 配置文件路径
    Subnets             *map[string][]byte // 子网分配位图
}
```

### IP 分配算法

```go
func (i *IPAM) Allocate(subnet *net.IPNet) (net.IP, error)
```

**分配流程：**
1. 加载现有分配信息
2. 计算子网可用 IP 数量：`2^(32-prefix_length)`
3. 初始化位图（如果是新子网）
4. 遍历位图找到第一个未分配的位（'0'）
5. 标记为已分配（'1'）
6. 计算对应的 IP 地址
7. 持久化分配信息

**IP 计算逻辑：**
```go
// 将位图索引转换为 IP 地址
for t := uint(4); t > 0; t -= 1 {
    []byte(ip)[4-t] += uint8(c >> ((t - 1) * 8))
}
ip[3] += 1  // 跳过网络地址
```

### IP 释放

```go
func (i *IPAM) Release(subnet *net.IPNet, ipAddr *net.IP) error
```

**释放流程：**
1. 计算 IP 在位图中的索引
2. 将对应位设置为 '0'
3. 持久化更新后的分配信息

## 4. 网络架构图

```
Host Network Namespace
┌─────────────────────────────────────────────────────────────┐
│                                                             │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐     │
│  │   eth0      │    │   zdocker0  │    │   iptables  │     │
│  │ (物理网卡)   │    │  (Bridge)   │    │   (NAT)     │     │
│  └─────────────┘    └─────────────┘    └─────────────┘     │
│                            │                               │
│                     ┌──────┼──────┐                        │
│                     │      │      │                        │
│                  veth1   veth2   veth3                     │
│                     │      │      │                        │
└─────────────────────┼──────┼──────┼─────────────────────────┘
                      │      │      │
        ┌─────────────┘      │      └─────────────┐
        │                    │                    │
Container1 NS          Container2 NS        Container3 NS
┌─────────────┐       ┌─────────────┐       ┌─────────────┐
│    eth0     │       │    eth0     │       │    eth0     │
│192.168.1.2  │       │192.168.1.3  │       │192.168.1.4  │
└─────────────┘       └─────────────┘       └─────────────┘
```

## 5. 关键技术点

### Veth Pair 技术
- 虚拟网络设备对，数据从一端进入从另一端出来
- 用于连接不同网络命名空间
- 一端在容器内，一端连接到 Bridge

### Linux Bridge
- 工作在数据链路层的虚拟交换机
- 连接多个网络接口
- 支持 MAC 地址学习和转发

### Network Namespace
- Linux 内核提供的网络隔离机制
- 每个容器拥有独立的网络栈
- 包括网络接口、路由表、iptables 规则等

### iptables NAT
- 网络地址转换，实现容器访问外网
- MASQUERADE 规则自动处理源地址转换
- DNAT 规则实现端口映射

## 6. 使用示例

### 创建网络
```bash
./zdocker network create --driver bridge --subnet 192.168.1.0/24 mynet
```

### 运行容器并连接网络
```bash
./zdocker run -net mynet -p 8080:80 nginx
```

### 查看网络列表
```bash
./zdocker network ls
```

## 7. 总结

ZDocker 的网络实现通过以下技术栈实现了完整的容器网络功能：

1. **隔离性**：通过 Network Namespace 实现网络隔离
2. **连通性**：通过 Veth Pair + Bridge 实现容器间通信
3. **外网访问**：通过 iptables NAT 实现容器访问外网
4. **IP 管理**：通过位图算法实现 IP 地址分配和回收
5. **持久化**：网络配置和 IP 分配信息持久化到文件系统

这种设计既保证了容器的网络隔离，又提供了灵活的网络连接能力，是容器网络的经典实现方案。

## 各组件作用总结

### 1. Bridge - 容器间通信枢纽
作用：给容器提供向外沟通的能力
- 作为虚拟交换机连接所有容器
- 提供网关功能 (192.168.1.1)
- 转发容器间的数据包


### 2. Veth Pair - 跨命名空间的信息桥梁
作用：在不同命名空间间充当信息桥梁
- 一端在容器内 (cif-xxxxx)
- 一端连接到 Bridge (xxxxx)
- 实现跨命名空间的数据传输


### 3. iptables - 网络地址转换

#### MASQUERADE (出站流量)
bash
iptables -t nat -A POSTROUTING -s 192.168.1.0/24 ! -o zdocker0 -j MASQUERADE

作用： 让主机知道流量来自哪个容器
• 容器访问外网时，将容器IP转换为主机IP
• 主机收到回包时，能正确转发回对应容器

#### DNAT (入站流量 - Port Mapping)
bash
iptables -t nat -A PREROUTING -p tcp --dport 8080 -j DNAT --to-destination 192.168.1.2:80

作用： 端口映射，外部访问容器服务
• 外部访问主机8080端口
• 自动转发到容器192.168.1.2的80端口

## 数据流向示例

### 容器访问外网
Container(192.168.1.2) → veth → Bridge → Host → 外网
↑                                                                                                                                                               
MASQUERADE转换                                                                                                                                                         
(192.168.1.2 → Host IP)


### 外部访问容器
外网 → Host:8080 → DNAT转换 → Bridge → veth → Container:80
↑                                                                                                                                                                                
(Host:8080 → 192.168.1.2:80)


你的理解完全正确：**Bridge提供通信能力，veth做跨空间桥梁，iptables处理地址转换和端口映射**！

## 容器间通信机制详解

### 核心架构：每容器一对veth + 统一Bridge中转

```
Container0 NS          Host NS              Container1 NS
┌─────────────┐       ┌─────────────┐       ┌─────────────┐
│ cif-xxxxx   │◄──────┤   xxxxx     │       │ cif-yyyyy   │
│(容器内端)    │       │(宿主机端)    │       │(容器内端)    │
└─────────────┘       └─────┬───────┘       └─────────────┘
                             │                      ▲
                             ▼                      │
                      ┌─────────────┐               │
                      │   Bridge    │               │
                      │ (zdocker0)  │               │
                      └─────┬───────┘               │
                             │                      │
                             └──────────────────────┘
                                   yyyyy
                                (宿主机端)
```

### 通信类型分析

#### 1. 容器与宿主机通信
**所需组件：** veth pair + iptables

**通信流程：**
```bash
# 容器 → 宿主机
Container → cif-xxxxx → xxxxx → Bridge → Host网络栈

# 宿主机 → 容器  
Host → Bridge → xxxxx → cif-xxxxx → Container
```

#### 2. 容器间通信（间接通信）
**实现方式：** 通过Bridge作为中转站

**通信流程：**
```bash
Container0 → cif-xxxxx → xxxxx → Bridge → yyyyy → cif-yyyyy → Container1
```

**关键特点：**
- 容器间不直接连接
- 所有容器的veth宿主机端都连接到同一个Bridge
- Bridge自动学习MAC地址，实现二层转发
- 可扩展到任意数量容器

#### 3. 容器访问外网
**所需组件：** veth + Bridge + iptables MASQUERADE

**地址转换：**
```bash
# 出站：容器IP → 宿主机IP
iptables -t nat -A POSTROUTING -s 192.168.1.0/24 ! -o zdocker0 -j MASQUERADE

# 回包：宿主机IP → 容器IP (自动处理)
```

#### 4. 外网访问容器（端口映射）
**所需组件：** iptables DNAT规则

**端口转发：**
```bash
# 外网:8080 → 容器:80
iptables -t nat -A PREROUTING -p tcp --dport 8080 -j DNAT --to-destination 192.168.1.2:80
```

### 代码实现关键点

#### veth pair创建与连接
```go
// bridge.go - 每个容器都执行此逻辑
la.MasterIndex = br.Attrs().Index  // 宿主机端自动连接到Bridge

endpoint.Device = netlink.Veth{
    LinkAttrs: la,                    // xxxxx (宿主机端)
    PeerName:  "cif-" + endpoint.ID[:5], // cif-xxxxx (容器端)
}
```

#### 网络命名空间隔离
```go
// network.go - 将容器端veth移入容器命名空间
netlink.LinkSetNsFd(*enLink, int(nsFD))  // 移动到容器命名空间
```

### 架构优势

1. **扩展性** - 可连接任意数量容器
2. **统一管理** - 所有网络通过一个Bridge管理  
3. **隔离性** - 每个容器独立的网络命名空间
4. **灵活性** - 支持容器间通信、外网访问、端口映射
5. **性能** - Bridge工作在二层，转发效率高

### 总结

ZDocker网络实现的核心思想：
- **每个容器一对veth** - 实现跨命名空间连接
- **统一Bridge中转** - 实现容器间间接通信
- **iptables协助** - 处理地址转换和端口映射
- **命名空间隔离** - 保证网络安全和独立性

这种设计既保证了容器的网络隔离，又提供了完整的网络连通能力，是容器网络的经典实现方案。