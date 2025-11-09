# Veth Pair 创建位置和使用逻辑分析

## Veth Pair 创建位置

**创建位置：** `bridge.go` 文件中的 `Connect()` 方法

```go
// bridge.go:56-59
endpoint.Device = netlink.Veth{
    LinkAttrs: la,                    // 设置名称和 MasterIndex
    PeerName:  "cif-" + endpoint.ID[:5], // peer 端名称
}

// bridge.go:65
if err = netlink.LinkAdd(&endpoint.Device); err != nil {
    return fmt.Errorf("error Add Endpoint Device: %v", err)
}
```

## 整体使用逻辑

### 1. 调用链路
```
容器启动 → Connect() → BridgeDriver.Connect() → 创建 Veth Pair
```

### 2. Veth Pair 的两端

**Host 端（Bridge 端）：**
- 名称：`endpoint.ID[:5]` (容器ID前5位)
- 位置：Host 网络命名空间
- 连接：自动挂载到 Bridge（通过 MasterIndex）

**Container 端（Peer 端）：**
- 名称：`"cif-" + endpoint.ID[:5]`
- 位置：容器网络命名空间
- 配置：IP地址、路由、启用状态

### 3. 关键步骤

#### 步骤1：创建 Veth Pair
```go
// 在 Host 命名空间创建 veth pair
la.MasterIndex = br.Attrs().Index  // 自动连接到 bridge
netlink.LinkAdd(&endpoint.Device)   // 创建设备对
```

#### 步骤2：移动 Peer 端到容器
```go
// network.go:176
defer enterContainerNetns(&peerLink, cinfo)()

// network.go:235 - 在 enterContainerNetns 中
netlink.LinkSetNsFd(*enLink, int(nsFD))  // 移动到容器命名空间
```

#### 步骤3：配置容器内网络
```go
// 在容器命名空间内执行
setInterfaceIP(ep.Device.PeerName, interfaceIP.String())  // 设置IP
setInterfaceUp(ep.Device.PeerName)                        // 启用接口
setInterfaceUp("lo")                                      // 启用回环
```

### 4. 网络拓扑

```
Host Network Namespace                Container Network Namespace
┌─────────────────────────┐          ┌─────────────────────────┐
│                         │          │                         │
│  ┌─────────────────┐    │          │    ┌─────────────────┐  │
│  │   zdocker0      │    │          │    │      eth0       │  │
│  │   (Bridge)      │    │          │    │ (cif-xxxxx)     │  │
│  │  192.168.1.1    │    │          │    │  192.168.1.2    │  │
│  └─────────┬───────┘    │          │    └─────────┬───────┘  │
│            │            │          │              │          │
│  ┌─────────┴───────┐    │   Veth   │    ┌─────────┴───────┐  │
│  │     xxxxx       │◄───┼──────────┼───►│   cif-xxxxx     │  │
│  │  (Host端)       │    │   Pair   │    │ (Container端)   │  │
│  └─────────────────┘    │          │    └─────────────────┘  │
│                         │          │                         │
└─────────────────────────┘          └─────────────────────────┘
```

### 5. 核心机制

**自动 Bridge 连接：**
- 通过设置 `la.MasterIndex = br.Attrs().Index`
- Veth 的 Host 端创建时自动连接到 Bridge
- 无需额外的 `brctl addif` 操作

**命名空间切换：**
- 使用 `netlink.LinkSetNsFd()` 移动网络设备
- 通过 `netns.Set()` 切换到容器命名空间
- 在容器内配置 IP 和路由

**数据流向：**
```
Container → cif-xxxxx → xxxxx → Bridge → 其他容器/外网
```

## 总结

Veth Pair 在 `BridgeDriver.Connect()` 中创建，一端自动连接到 Bridge，另一端移动到容器命名空间并配置网络参数。这种设计实现了容器与 Host 网络的隔离和连通。
