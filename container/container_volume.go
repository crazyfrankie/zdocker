package container

import (
	"os"
	"os/exec"

	log "github.com/sirupsen/logrus"
)

func NewWorkSpace(root string, mnt string) {
	createReadOnlyLayer(root)
	createWriteLayer(root)
	createMountPoint(root, mnt)
}

// createReadOnlyLayer extract busybox.tar into the busybox directory as a read-only layer for the container
func createReadOnlyLayer(root string) {
	busyboxUrl := root + "busybox/"
	busyboxTarUrl := root + "busybox.tar"
	exists, err := pathExists(busyboxUrl)
	if err != nil {
		log.Infof("fail to judge whether dir %s exists. %v", busyboxUrl, err)
	}
	if !exists {
		if err := os.Mkdir(busyboxUrl, 0777); err != nil {
			log.Errorf("mkdir dir %s error. %v", busyboxUrl, err)
		}
		if _, err := exec.Command("tar", []string{"-xvf", busyboxTarUrl, "-C", busyboxUrl}...).CombinedOutput(); err != nil {
			log.Errorf("untar dir %s error. %v", busyboxTarUrl, err)
		}
	}
}

func createWriteLayer(root string) {
	writeUrl := root + "writeLayer/"
	if err := os.Mkdir(writeUrl, 0777); err != nil {
		log.Errorf("mkdir dir %s error. %v", writeUrl, err)
	}
}

func createMountPoint(root string, mnt string) {
	// create mnt folder as mount point
	if err := os.Mkdir(mnt, 0777); err != nil {
		log.Errorf("mkdir dir %s error. %v", mnt, err)
	}
	dirs := "dirs=" + root + "writeLayer:" + root + "busybox"
	cmd := exec.Command("mount", "-t", "aufs", "-o", dirs, "none", mnt)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("%v", err)
	}
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func DeleteWorkSpace(root string, mnt string) {
	deleteMountPoint(mnt)
	deleteWriteLayer(root)
}

func deleteMountPoint(mnt string) {
	cmd := exec.Command("umount", mnt)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("%v", err)
	}
	if err := os.RemoveAll(mnt); err != nil {
		log.Errorf("remove dir %s error. %v", mnt, err)
	}
}

func deleteWriteLayer(root string) {
	writePath := root + "writeLayer/"
	if err := os.RemoveAll(writePath); err != nil {
		log.Errorf("remove dir %s error %v", writePath, err)
	}
}
