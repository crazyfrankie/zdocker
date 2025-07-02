package container

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"
)

// RunContainerInitProcess execute initialization procedures inside the container
func RunContainerInitProcess() error {
	commands := readUserCommand()
	if commands == nil || len(commands) == 0 {
		return fmt.Errorf("run container get user command error, commands is nil")
	}

	setUpMount()

	path, err := exec.LookPath(commands[0])
	if err != nil {
		log.Errorf("Exec loop path error %v", err)
		return err
	}
	log.Infof("Find path %s", path)
	if err := syscall.Exec(path, commands[0:], os.Environ()); err != nil {
		log.Errorf(err.Error())
	}
	return nil
}

func setUpMount() {
	// The original mydocker project did not do this here, perhaps because the environment itself supports the mount propagation type to be private,
	// most distributions default to shared, and pivotRoot requires the current root filesystem to be clean, i.e.,
	// the mount point cannot be shared, otherwise the old and the new root will interact with each other,
	// which is forbidden by the kernel, so you need to set the current mount namespace to private to prevent it from affecting the host.
	// The kernel forbids this, so you need to set the mount propagation type of the current mount namespace to private to prevent it from affecting the host.
	if err := syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		log.Errorf("failed to make / private: %v", err)
		return
	}

	pwd, err := os.Getwd()
	if err != nil {
		log.Errorf("get current location error: %v", err)
		return
	}
	log.Infof("current location is %s", pwd)
	if err := pivotRoot(pwd); err != nil {
		log.Errorf("pivot root error: %v", err)
		return
	}

	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), "")

	syscall.Mount("tmpfs", "/dev", "tmpfs", syscall.MS_NOSUID|syscall.MS_STRICTATIME, "mode=755")
}

// pivotRoot switches the current mount namespace's root filesystem to the specified path.
//
// It performs a bind-mount of `root` onto itself to ensure it is a mount point,
// creates a temporary `.pivot_root` directory to store the old root,
// and then calls the `pivot_root(2)` syscall to make `root` the new filesystem root.
// After switching, it unmounts the old root to fully isolate the new root filesystem.
//
// The target `root` must be a valid mount point and must not be the same as `/`.
func pivotRoot(root string) error {
	if err := syscall.Mount(root, root, "bind", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("mount rootfs to itself error: %v", err)
	}
	// Change working directory to new root
	if err := syscall.Chdir(root); err != nil {
		return fmt.Errorf("chdir to new root: %v", err)
	}
	// create `rootfs/.pivot_root` to store old root
	pivotDir := filepath.Join(root, ".pivot_root")
	if err := os.Mkdir(pivotDir, 0777); err != nil {
		return err
	}
	// pivot_root to the new rootfs, old_root is now mounted on `rootfs/.pivot_root`
	// The mount point is still visible in the mount command.
	if err := syscall.PivotRoot(root, pivotDir); err != nil {
		return fmt.Errorf("pivot_root %v", err)
	}
	// Modify the current working directory to the root directory
	if err := syscall.Chdir("/"); err != nil {
		return fmt.Errorf("chdir / %v", err)
	}
	pivotDir = filepath.Join("/", ".pivot_root")
	// umount rootfs/.pivot_root
	if err := syscall.Unmount(pivotDir, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount pivot_root dir %v", err)
	}
	// Remove the temporary .pivot_root directory
	return os.Remove(pivotDir)
}

func readUserCommand() []string {
	// Read index 3 (3rd file descriptor) from the pipe
	pipe := os.NewFile(uintptr(3), "pipe")
	msg, err := io.ReadAll(pipe)
	if err != nil {
		log.Errorf("init read pipe error %v", err)
		return nil
	}
	msgStr := string(msg)
	return strings.Split(msgStr, " ")
}
