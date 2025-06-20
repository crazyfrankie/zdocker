package container

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

// Constants for overlay2 filesystem structure
const (
	RootUrl       = "/root"
	MntUrl        = "/root/mnt/%s"
	WriteLayerUrl = "/root/writeLayer/%s"
	OverlayLower  = "/root/%s"
	OverlayWork   = "/root/workdir/%s"
)

func NewWorkSpace(imageName string, containerName string, volume string) {
	createLowerLayer(imageName)
	createUpperLayer(containerName)
	createWorkDir(containerName)
	createMountPoint(imageName, containerName)
	if volume != "" {
		volumeUrls := strings.Split(volume, ":")
		if len(volumeUrls) == 2 && volumeUrls[0] != "" && volumeUrls[1] != "" {
			mountVolume(containerName, volumeUrls)
			log.Infof("NewWorkSpace volume urls %q", volumeUrls)
		} else {
			log.Infof("volumeUrl parameter input is not correct.")
		}
	}
}

// createReadOnlyLayer extract busybox.tar into the busybox directory as a read-only layer for the container
func createLowerLayer(imageName string) {
	lowerDir := fmt.Sprintf(OverlayLower, imageName)
	imageUrl := filepath.Join(RootUrl, fmt.Sprintf("%s.tar", imageName))
	exists, err := pathExists(lowerDir)
	if err != nil {
		log.Errorf("check lower dir error: %v", err)
	}
	if !exists {
		if err := os.MkdirAll(lowerDir, 0777); err != nil {
			log.Errorf("mkdir lower dir error: %v", err)
		}
		if _, err := exec.Command("tar", "-xvf", imageUrl, "-C", lowerDir, "--strip-components=1").CombinedOutput(); err != nil {
			log.Errorf("untar busybox error: %v", err)
		}
	}
}

func createUpperLayer(containerName string) {
	upperDir := fmt.Sprintf(WriteLayerUrl, containerName)
	if err := os.MkdirAll(upperDir, 0777); err != nil {
		log.Errorf("mkdir upper dir %s error: %v", upperDir, err)
	}
}

func createWorkDir(containerName string) {
	workDir := fmt.Sprintf(OverlayWork, containerName)
	if err := os.MkdirAll(workDir, 0777); err != nil {
		log.Errorf("mkdir work dir %s error: %v", workDir, err)
	}
}

func createMountPoint(imageName string, containerName string) {
	mntUrl := fmt.Sprintf(MntUrl, containerName)
	if err := os.MkdirAll(mntUrl, 0777); err != nil {
		log.Errorf("mkdir mount point error: %v", err)
	}
	lowerDir := fmt.Sprintf(OverlayLower, imageName)
	upperDir := fmt.Sprintf(WriteLayerUrl, containerName)
	workDir := fmt.Sprintf(OverlayWork, containerName)

	options := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lowerDir, upperDir, workDir)
	cmd := exec.Command("mount", "-t", "overlay", "overlay", "-o", options, mntUrl)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("overlay mount error: %v", err)
	}
}

func mountVolume(containerName string, volumeUrls []string) {
	// create host dir
	parentUrl := volumeUrls[0]
	mntUrl := fmt.Sprintf(MntUrl, containerName)

	// handle container path - remove leading slash if present
	containerPath := volumeUrls[1]
	if strings.HasPrefix(containerPath, "/") {
		containerPath = containerPath[1:]
	}
	containerVolumeDir := filepath.Join(mntUrl, containerPath)

	// ensure host directory exists
	if err := os.MkdirAll(parentUrl, 0777); err != nil {
		log.Infof("mkdir parent dir %s error. %v", parentUrl, err)
	}

	// create a mount point on the container filesystem.
	if err := os.MkdirAll(containerVolumeDir, 0777); err != nil {
		log.Infof("mkdir container dir %s error. %v", containerVolumeDir, err)
	}

	// mount the host file directory to the container mount point
	cmd := exec.Command("mount", "--bind", parentUrl, containerVolumeDir)
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

func DeleteWorkSpace(containerName string, volume string) {
	if volume != "" {
		volumeUrls := strings.Split(volume, ":")
		if len(volumeUrls) == 2 && volumeUrls[0] != "" && volumeUrls[1] != "" {
			deleteVolumeMount(containerName, volumeUrls)
		}
	}
	deleteMountPoint(containerName)
	deleteOverlayDirs(containerName)
}

func deleteVolumeMount(containerName string, volumeUrls []string) error {
	mntURL := fmt.Sprintf(MntUrl, containerName)

	// handle container path - remove leading slash if present
	containerPath := volumeUrls[1]
	if strings.HasPrefix(containerPath, "/") {
		containerPath = containerPath[1:]
	}
	containerDir := filepath.Join(mntURL, containerPath)

	cmd := exec.Command("umount", containerDir)
	if err := cmd.Run(); err != nil {
		log.Errorf("umount container dir failed %v", err)
		return err
	}

	return nil
}

func deleteMountPoint(containerName string) error {
	mntUrl := fmt.Sprintf(MntUrl, containerName)
	_, err := exec.Command("umount", mntUrl).CombinedOutput()
	if err != nil {
		log.Errorf("unmount %s error %v", mntUrl, err)
		return err
	}
	if err := os.RemoveAll(mntUrl); err != nil {
		log.Errorf("remove mount point dir %s error: %v", mntUrl, err)
		return err
	}

	return nil
}

func deleteOverlayDirs(containerName string) {
	writeUrl := fmt.Sprintf(WriteLayerUrl, containerName)
	if err := os.RemoveAll(writeUrl); err != nil {
		log.Infof("Remove writeLayer dir %s error %v", writeUrl, err)
	}
	workUrl := fmt.Sprintf(OverlayWork, containerName)
	if err := os.RemoveAll(workUrl); err != nil {
		log.Infof("Remove writeLayer dir %s error %v", workUrl, err)
	}
}
