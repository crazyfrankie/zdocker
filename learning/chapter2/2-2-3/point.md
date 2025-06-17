# 修改点
书上源码采用的是 cgroup v1, 直接翻译成 v2 版本的代码如下：
```
const cgroupMemoryHierarchyMount = "/sys/fs/cgroup"

func main() {
	if os.Args[0] == "/proc/self/exe" {
		fmt.Printf("current pid %d\n", syscall.Getpid())
   
		cmd := exec.Command("stress", "--vm-bytes", "200m", "--vm-keep", "-m", "1")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Println("stress failed:", err)
			os.Exit(1)
		}
	} else {
		// 主进程逻辑
		cmd := exec.Command("/proc/self/exe")
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
		}
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			fmt.Println("ERROR", err)
			os.Exit(1)
		}

		// 设置 CGroup
		pid := cmd.Process.Pid
		fmt.Printf("Container PID: %d\n", pid)
		cgroupPath := filepath.Join(cgroupMemoryHierarchyMount, "testmemorylimit")
		_ = os.Mkdir(cgroupPath, 0755)

		// 启用 memory 控制器
		if err := os.WriteFile(filepath.Join(cgroupMemoryHierarchyMount, "cgroup.subtree_control"), []byte("+memory"), 0644); err != nil {
			fmt.Println("Failed to enable memory controller:", err)
			os.Exit(1)
		}

		// 添加进程并设置限制
		if err := os.WriteFile(filepath.Join(cgroupPath, "cgroup.procs"), []byte(strconv.Itoa(pid)), 0644); err != nil {
			fmt.Println("Failed to add PID to cgroup:", err)
			os.Exit(1)
		}

		if err := os.WriteFile(filepath.Join(cgroupPath, "memory.max"), []byte("100M"), 0644); err != nil {
			fmt.Println("Failed to set memory limit:", err)
			os.Exit(1)
		}

		cmd.Process.Wait()
	}
}
```

但由于 CGroup v1 的自动进程继承机制

| 特性   | CGroup v1      | CGroup v2  |
|------|----------------|------------|
| 子进程继承规则 | 子进程自动加入父进程的 CGroup | 子进程默认不继承，需显式控制 |
| 控制文件 | tasks          | cgroup.procs |
| Shell 启动的影响 | 无逃逸风险          | 可能逃逸       |

那么就会导致限制不生效, 需要在容器内的处理逻辑中加上
```
if os.Args[0] == "/proc/self/exe" {
		fmt.Printf("current pid %d\n", syscall.Getpid())

		// 确保当前进程在 CGroup
		if err := os.WriteFile(filepath.Join(cgroupMemoryHierarchyMount, "testmemorylimit", "cgroup.procs"), []byte("1"), 0644); err != nil {
			fmt.Println("Failed to re-add PID to cgroup:", err)
			os.Exit(1)
		}

		cmd := exec.Command("stress", "--vm-bytes", "200m", "--vm-keep", "-m", "1")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Println("stress failed:", err)
			os.Exit(1)
		}
	}
```

## 关键差异点解析
### 1. 进程树控制方式不同
- v1 版本：
  ```
  ioutil.WriteFile("tasks", []byte(pid), 0644)  // 写入父进程 PID
  ```
    - 自动继承：所有子进程（包括通过 sh 启动的 stress）会自动加入同一 CGroup 
    - 内核机制：v1 的 tasks 文件会强制后代进程继承 CGroup 
- v2 版本：
   ```
   os.WriteFile("cgroup.procs", []byte("1"), 0644) // 需显式绑定 PID 1
   ``` 
   - 默认不继承：v2 要求显式将进程加入 CGroup
   - 安全设计：避免意外传播限制
### 2. Shell 启动的差异表现
```
// v1 版本
cmd := exec.Command("sh", "-c", `stress...`)  // 通过 shell 启动

// v2 版本
cmd := exec.Command("stress", "--vm-bytes", "200m") // 直接启动
```  
 
- v1：sh 和 stress 都自动继承 CGroup
- v2：若未显式绑定，stress 可能运行在默认 CGroup
- 
### 3. 内存限制的单位处理
```
// v1 成功
ioutil.WriteFile("memory.limit_in_bytes", []byte("100m"), 0644)  // 小写 m 被容忍

// v2 需严格
os.WriteFile("memory.max", []byte("100M"), 0644)  // 必须大写 M
```
- v1：对单位格式更宽松
- v2：要求严格符合规范

## 为什么 v1 不需要重新绑定 PID 1？
1. PID Namespace 的差异： 
   - v1 时代容器技术不成熟，通常直接使用宿主 PID 空间 
   - 现代容器（v2）严格隔离 PID namespace，需显式管理
2. 设计哲学变化：

| 版本 | 设计目标       | 进程控制 |
|----|------------|------|
| v1 | 简单资源限制     | 自动传播 |
| v2 | 精细控制 + 安全性 | 显式管理 |
