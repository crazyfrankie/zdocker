package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

// 两次加入 Cgroup 操作：
// 内核源码片段（简化）
// void cgroup_procs_write() {
//    if (在容器Namespace中操作) {
//        // 设置进程树继承标志
//        task->cgroups->dom_cgrp = cgrp;
//    } else {
//        // 仅加入单个进程
//        cgroup_attach_task(cgrp, task);
//    }
//}
// 第一次绑定是 进程级限制，仅限制单个进程
// 第二次绑定是 进程树级 规则声明，建立继承链

const cgroupMemoryHierarchyMount = "/sys/fs/cgroup"

func main() {
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
