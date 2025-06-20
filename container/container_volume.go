package container

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

func NewWorkSpace(rootUrl string, mntUrl string, volumeUrl string) {
	createLowerLayer(rootUrl)
	createUpperLayer(rootUrl)
	createWorkDir(rootUrl)
	createMountPoint(rootUrl, mntUrl)
	if volumeUrl != "" {
		volumeUrls := strings.Split(volumeUrl, ":")
		if len(volumeUrls) == 2 && volumeUrls[0] != "" && volumeUrls[1] != "" {
			mountVolume(mntUrl, volumeUrls)
			log.Infof("%q", volumeUrls)
		} else {
			log.Infof("volumeUrl parameter input is not correct.")
		}
	}
}

// createReadOnlyLayer extract busybox.tar into the busybox directory as a read-only layer for the container
func createLowerLayer(rootUrl string) {
	lowerDir := filepath.Join(rootUrl, "lower")
	busyboxTar := filepath.Join(rootUrl, "busybox.tar")
	exists, err := pathExists(lowerDir)
	if err != nil {
		log.Errorf("check lower dir error: %v", err)
	}
	if !exists {
		if err := os.Mkdir(lowerDir, 0777); err != nil {
			log.Errorf("mkdir lower dir error: %v", err)
		}
		if _, err := exec.Command("tar", "-xvf", busyboxTar, "-C", lowerDir, "--strip-components=1").CombinedOutput(); err != nil {
			log.Errorf("untar busybox error: %v", err)
		}
	}
}

func createUpperLayer(rootUrl string) {
	upperDir := filepath.Join(rootUrl, "upper")
	if err := os.MkdirAll(upperDir, 0777); err != nil {
		log.Errorf("mkdir upper dir error: %v", err)
	}
}

func createWorkDir(rootUrl string) {
	workDir := filepath.Join(rootUrl, "work")
	if err := os.MkdirAll(workDir, 0777); err != nil {
		log.Errorf("mkdir work dir error: %v", err)
	}
}

func createMountPoint(rootUrl, mntUrl string) {
	if err := os.MkdirAll(mntUrl, 0777); err != nil {
		log.Errorf("mkdir mount point error: %v", err)
	}
	lowerDir := filepath.Join(rootUrl, "lower")
	upperDir := filepath.Join(rootUrl, "upper")
	workDir := filepath.Join(rootUrl, "work")

	options := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lowerDir, upperDir, workDir)
	cmd := exec.Command("mount", "-t", "overlay", "overlay", "-o", options, mntUrl)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("overlay mount error: %v", err)
	}
}

func mountVolume(mntUrl string, volumeUrls []string) {
	// create host dir
	parentUrl := volumeUrls[0]
	containerDir := filepath.Join(mntUrl, volumeUrls[1])
	if err := os.Mkdir(parentUrl, 0777); err != nil {
		log.Infof("mkdir parent dir %s error. %v", parentUrl, err)
	}
	// create a mount point on the container filesystem.
	if err := os.Mkdir(containerDir, 0777); err != nil {
		log.Infof("mkdir container dir %s error. %v", containerDir, err)
	}
	// mount the host file directory to the container mount point
	cmd := exec.Command("mount", "--bind", parentUrl, containerDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("mount volume failed. %v", err)
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

func DeleteWorkSpace(rootUrl, mntUrl, volumeUrl string) {
	if volumeUrl != "" {
		volumeUrls := strings.Split(volumeUrl, ":")
		if len(volumeUrls) == 2 && volumeUrls[0] != "" && volumeUrls[1] != "" {
			deleteVolumeMount(mntUrl, volumeUrls)
		}
	}
	deleteMountPoint(mntUrl)
	deleteOverlayDirs(rootUrl)
}

func deleteVolumeMount(mntUrl string, volumeUrls []string) {
	containerDir := filepath.Join(mntUrl, volumeUrls[1])
	cmd := exec.Command("umount", containerDir)
	if err := cmd.Run(); err != nil {
		log.Errorf("umount container dir failed %v", err)
	}
}

func deleteMountPoint(mntUrl string) {
	cmd := exec.Command("umount", mntUrl)
	cmd.Run()
	if err := os.RemoveAll(mntUrl); err != nil {
		log.Errorf("remove mount point error: %v", err)
	}
}

func deleteOverlayDirs(rootUrl string) {
	os.RemoveAll(filepath.Join(rootUrl, "upper"))
	os.RemoveAll(filepath.Join(rootUrl, "work"))
}
