# Clone 命令 flags 分组解析
`go/src/syscall.go`:
```
// Linux unshare/clone/clone2/clone3 flags, architecture-independent,
// copied from linux/sched.h.
const (
	CLONE_VM             = 0x00000100 // set if VM shared between processes
	CLONE_FS             = 0x00000200 // set if fs info shared between processes
	CLONE_FILES          = 0x00000400 // set if open files shared between processes
	CLONE_SIGHAND        = 0x00000800 // set if signal handlers and blocked signals shared
	CLONE_PIDFD          = 0x00001000 // set if a pidfd should be placed in parent
	CLONE_PTRACE         = 0x00002000 // set if we want to let tracing continue on the child too
	CLONE_VFORK          = 0x00004000 // set if the parent wants the child to wake it up on mm_release
	CLONE_PARENT         = 0x00008000 // set if we want to have the same parent as the cloner
	CLONE_THREAD         = 0x00010000 // Same thread group?
	CLONE_NEWNS          = 0x00020000 // New mount namespace group
	CLONE_SYSVSEM        = 0x00040000 // share system V SEM_UNDO semantics
	CLONE_SETTLS         = 0x00080000 // create a new TLS for the child
	CLONE_PARENT_SETTID  = 0x00100000 // set the TID in the parent
	CLONE_CHILD_CLEARTID = 0x00200000 // clear the TID in the child
	CLONE_DETACHED       = 0x00400000 // Unused, ignored
	CLONE_UNTRACED       = 0x00800000 // set if the tracing process can't force CLONE_PTRACE on this clone
	CLONE_CHILD_SETTID   = 0x01000000 // set the TID in the child
	CLONE_NEWCGROUP      = 0x02000000 // New cgroup namespace
	CLONE_NEWUTS         = 0x04000000 // New utsname namespace
	CLONE_NEWIPC         = 0x08000000 // New ipc namespace
	CLONE_NEWUSER        = 0x10000000 // New user namespace
	CLONE_NEWPID         = 0x20000000 // New pid namespace
	CLONE_NEWNET         = 0x40000000 // New network namespace
	CLONE_IO             = 0x80000000 // Clone io context

	// Flags for the clone3() syscall.

	CLONE_CLEAR_SIGHAND = 0x100000000 // Clear any signal handler and reset to SIG_DFL.
	CLONE_INTO_CGROUP   = 0x200000000 // Clone into a specific cgroup given the right permissions.

	// Cloning flags intersect with CSIGNAL so can be used with unshare and clone3
	// syscalls only:

	CLONE_NEWTIME = 0x00000080 // New time namespace
)
```

# 1. 资源共享标志（共享上下文）
这些标志决定子进程与父进程共享哪些资源（默认情况下，子进程会继承父进程的资源，但某些资源可以被隔离或共享）：

| 标志            | 作用                                  |
|---------------|-------------------------------------|
| CLONE_VM      | 共享虚拟内存（父子进程在同一内存空间运行，类似线程）。         |
| CLONE_FS      | 共享文件系统信息（根目录、当前工作目录等）。              |
| CLONE_FILES   | 共享打开的文件描述符表（父子进程共享相同的文件句柄）。         |
| CLONE_SIGHAND | 共享信号处理函数和阻塞信号集。                     |
| CLONE_SYSVSEM | 共享 System V 信号量的 SEM_UNDO 行为。       |
| CLONE_THREAD  | 将子进程放入父进程的线程组（类似 POSIX 线程，共享 TGID）。 |
| CLONE_IO      | 共享 I/O 上下文（用于异步 I/O，如 io_uring）。    |

## 2. 命名空间隔离标志（创建新命名空间）
这些标志用于隔离资源，每个标志对应一种 Linux Namespace，是实现容器化的核心：

| 标志              | 作用                                              |
|-----------------|-------------------------------------------------|
| CLONE_NEWNS     | 创建新的 Mount Namespace（隔离文件系统挂载点）。                |
| CLONE_NEWUTS    | 创建新的 UTS Namespace（隔离主机名和域名）。                   |
| CLONE_NEWIPC    | 创建新的 IPC Namespace（隔离 System V IPC/POSIX 消息队列）。 |
| CLONE_NEWPID    | 创建新的 PID Namespace（隔离进程 ID，容器内 PID 从 1 开始）。     |
| CLONE_NEWNET    | 创建新的 Network Namespace（隔离网络设备、端口等）。             |
| CLONE_NEWUSER   | 创建新的 User Namespace（隔离用户/组 ID，允许普通用户提权）。        |
| CLONE_NEWCGROUP | 创建新的 CGroup Namespace（隔离 CGroup 视图）。            |
| CLONE_NEWTIME   | 创建新的 Time Namespace（隔离系统时间，用于容器内时间偏移）。          |

## 3. 进程/线程控制标志
这些标志控制进程或线程的创建行为：

| 标志                   | 作用                                    |
|----------------------|---------------------------------------|
| CLONE_VFORK          | 父进程暂停，直到子进程调用 execve 或退出（类似 vfork()）。 |
| CLONE_PARENT         | 子进程与调用者（而非父进程）共享父进程（即调用者的父进程）。        |
| CLONE_PTRACE         | 如果父进程被跟踪（如 GDB），子进程也会被跟踪。             |
| CLONE_UNTRACED       | 禁止调试器强制附加 CLONE_PTRACE。               |
| CLONE_CHILD_SETTID   | 将子进程的 TID（线程 ID）写入指定的用户空间地址。          |
| CLONE_PARENT_SETTID  | 将子进程的 TID 写入父进程的用户空间地址。               |
| CLONE_CHILD_CLEARTID | 子进程退出时清除指定的 TID（用于线程同步）。              |
| CLONE_SETTLS         | 为子进程设置新的 TLS（线程本地存储）描述符。              |
| CLONE_DETACHED       | 已废弃，现代内核忽略此标志。                        |

## 4. 其他高级标志

| 标志                  | 作用                              |
|---------------------|---------------------------------|
| CLONE_PIDFD         | 返回一个指向子进程的 pidfd 文件描述符（用于进程监控）。 |
| CLONE_CLEAR_SIGHAND | 清除所有信号处理程序（重置为 SIG_DFL）。        |
| CLONE_INTO_CGROUP   | 将子进程放入指定的 CGroup（需内核支持）。        |

## 总结：
这些标志可以分为：

1. 资源共享组（如 CLONE_VM、CLONE_FILES）。
2. 隔离组（如 CLONE_NEWPID、CLONE_NEWNET）。
3. 控制组（如 CLONE_VFORK、CLONE_CHILD_SETTID）。
4. 高级功能组（如 CLONE_PIDFD、CLONE_INTO_CGROUP）。