package cgroups

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

const (
	// CgroupRoot default cgroup2
	CgroupRoot = "/sys/fs/cgroup"
)

// CgroupManager manages cgroup v2 resources
type CgroupManager struct {
	// Path of the cgroup relative to the cgroup root
	Path string
	// Resource configuration
	Resource *ResourceConfig
}

// ResourceConfig holds resource limit configurations
type ResourceConfig struct {
	MemoryLimit string // in bytes
	CpuShare    string // in shares (relative weight)
	CpuSet      string // cpus the container can use, e.g., "0-1,3"
}

// NewCgroupManager creates a new CgroupManager instance
func NewCgroupManager(path string) *CgroupManager {
	return &CgroupManager{
		Path: path,
	}
}

// Apply ensures that the cgroup directory exists,
// writes the process IDs to the cgroup.procs file under the cgroup path,
// and the processes can be subject to the resource limits set by that cgroup
func (c *CgroupManager) Apply(pid int) error {
	if err := c.createCgroupIfNotExists(); err != nil {
		return err
	}
	// Write process ID to cgroup.procs
	cgroupProcsPath := filepath.Join(c.getAbsolutePath(), "cgroup.procs")

	return os.WriteFile(cgroupProcsPath, []byte(strconv.Itoa(pid)), 0700)
}

func (c *CgroupManager) Set(res *ResourceConfig) error {
	c.Resource = res
	if err := c.createCgroupIfNotExists(); err != nil {
		return err
	}

	cgroupPath := c.getAbsolutePath()

	// Set memory limit
	if res.MemoryLimit != "" {
		memoryLimitPath := filepath.Join(cgroupPath, "memory.max")
		if err := os.WriteFile(memoryLimitPath, []byte(res.MemoryLimit), 0700); err != nil {
			return fmt.Errorf("failed to set memory limit: %v", err)
		}
	}

	// Set CPU shares (weight)
	if res.CpuShare != "" {
		// In cgroup v2, cpu.shares is replaced by cpu.weight
		// Convert shares (1-1024) to weight (1-10000)
		shares, err := strconv.Atoi(res.CpuShare)
		if err != nil {
			return fmt.Errorf("invalid cpu share value: %v", err)
		}

		// Convert from shares to weight (approximate conversion)
		weight := shares * 10
		if weight < 1 {
			weight = 1
		} else if weight > 10000 {
			weight = 10000
		}

		cpuWeightPath := filepath.Join(cgroupPath, "cpu.weight")
		if err := os.WriteFile(cpuWeightPath, []byte(strconv.Itoa(weight)), 0700); err != nil {
			return fmt.Errorf("failed to set cpu weight: %v", err)
		}
	}

	// Set CPU set
	if res.CpuSet != "" {
		cpuSetPath := filepath.Join(cgroupPath, "cpuset.cpus")
		if err := os.WriteFile(cpuSetPath, []byte(res.CpuSet), 0700); err != nil {
			return fmt.Errorf("failed to set cpuset: %v", err)
		}
	}

	return nil
}

// Destroy migrates all processes in the cgroup to the parent cgroup by reading cgroup.procs
// and writing the process IDs to the parent cgroup's cgroup.procs,
// then deletes the entire cgroup directory
func (c *CgroupManager) Destroy() error {
	cgroupPath := c.getAbsolutePath()

	// First, move all processes to parent cgroup
	procsPath := filepath.Join(cgroupPath, "cgroup.procs")
	procs, err := os.ReadFile(procsPath)
	if err == nil && len(procs) > 0 {
		// Get parent cgroup path
		parentPath := path.Dir(cgroupPath)
		parentProcsPath := filepath.Join(parentPath, "cgroup.procs")

		// Move each process to parent
		for _, pidStr := range strings.Split(string(procs), "\n") {
			if pidStr == "" {
				continue
			}
			if err := os.WriteFile(parentProcsPath, []byte(pidStr), 0700); err != nil {
				log.Warnf("Failed to move process %s to parent cgroup: %v", pidStr, err)
			}
		}
	}

	// Remove the cgroup directory
	if err := os.RemoveAll(cgroupPath); err != nil {
		return fmt.Errorf("failed to remove cgroup: %v", err)
	}

	return nil
}

// createCgroupIfNotExists creates the cgroup directory if it doesn't exist
func (c *CgroupManager) createCgroupIfNotExists() error {
	cgroupPath := c.getAbsolutePath()
	if _, err := os.Stat(cgroupPath); os.IsNotExist(err) {
		if err := os.MkdirAll(cgroupPath, 0755); err != nil {
			return fmt.Errorf("failed to create cgroup directory: %v", err)
		}

		// Enable controllers in parent directory
		parentPath := filepath.Dir(cgroupPath)
		if parentPath != CgroupRoot {
			controllers := []string{"cpu", "cpuset", "memory"}
			for _, ctrl := range controllers {
				subtreeControlPath := filepath.Join(parentPath, "cgroup.subtree_control")
				// Add controller to subtree_control if not already enabled
				if err := c.enableController(subtreeControlPath, ctrl); err != nil {
					log.Warnf("Failed to enable %s controller: %v", ctrl, err)
				}
			}
		}
	}

	return nil
}

// enableController enables a controller in the subtree_control file
func (c *CgroupManager) enableController(subtreeControlPath string, controller string) error {
	content, err := os.ReadFile(subtreeControlPath)
	if err != nil {
		return err
	}

	// Check if controller is already enabled
	if strings.Contains(string(content), controller) {
		return nil
	}

	// Enable controller by writing "+controller"
	return os.WriteFile(subtreeControlPath, []byte(fmt.Sprintf("+%s", controller)), 0700)
}

// getAbsolutePath returns the absolute path of the cgroup
func (c *CgroupManager) getAbsolutePath() string {
	return filepath.Join(CgroupRoot, c.Path)
}
