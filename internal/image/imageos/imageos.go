package imageos

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/open-edge-platform/image-composer/internal/chroot"
	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/image/imageboot"
	"github.com/open-edge-platform/image-composer/internal/image/imagesecure"
	"github.com/open-edge-platform/image-composer/internal/image/imagesign"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
	"github.com/open-edge-platform/image-composer/internal/utils/mount"
	"github.com/open-edge-platform/image-composer/internal/utils/shell"
)

func InstallImageOs(diskPathIdMap map[string]string, template *config.ImageTemplate) error {
	log := logger.Logger()
	log.Infof("Installing OS for image: %s", template.GetImageName())

	installRoot, err := initChrootInstallRoot(template)
	if err != nil {
		return fmt.Errorf("failed to initialize chroot install root: %w", err)
	}

	mountPointInfoList, err := mountDiskToChroot(installRoot, diskPathIdMap, template)
	if err != nil {
		return fmt.Errorf("failed to mount disk to chroot: %w", err)
	}

	log.Infof("Image installation pre-processing...")
	if err = preImageOsInstall(installRoot, template); err != nil {
		return fmt.Errorf("pre-install failed: %w", err)
	}

	log.Infof("Image package installation...")
	if err = installImagePkgs(installRoot, template); err != nil {
		return fmt.Errorf("failed to install image packages: %w", err)
	}

	log.Infof("Image system configuration...")
	if err = updateImageConfig(installRoot, template); err != nil {
		return fmt.Errorf("failed to update image config: %w", err)
	}

	log.Infof("Installing bootloader...")
	if err = imageboot.InstallImageBoot(diskPathIdMap, template); err != nil {
		return fmt.Errorf("failed to install image boot: %w", err)
	}

	if err = imagesecure.ConfigImageSecurity(installRoot, template); err != nil {
		return fmt.Errorf("failed to configure image security: %w", err)
	}

	log.Infof("Configuring UKI...")
	if err = buildImageUKI(installRoot, template); err != nil {
		return fmt.Errorf("failed to configure UKI: %w", err)
	}

	if err = imagesign.SignImage(installRoot, template); err != nil {
		return fmt.Errorf("failed to sign image: %w", err)
	}

	log.Infof("Image installation post-processing...")
	if err = postImageOsInstall(installRoot, template); err != nil {
		return fmt.Errorf("post-install failed: %w", err)
	}

	if err = umountDiskFromChroot(mountPointInfoList); err != nil {
		return fmt.Errorf("failed to unmount disk from chroot: %w", err)
	}

	return nil
}

func initChrootInstallRoot(template *config.ImageTemplate) (string, error) {
	if _, err := os.Stat(chroot.ChrootImageBuildDir); os.IsNotExist(err) {
		return "", fmt.Errorf("chroot image build directory does not exist: %s", chroot.ChrootImageBuildDir)
	}
	sysConfigName := template.GetSystemConfigName()
	installRoot := filepath.Join(chroot.ChrootImageBuildDir, sysConfigName)
	if _, err := shell.ExecCmd("mkdir -p "+installRoot, true, "", nil); err != nil {
		return installRoot, fmt.Errorf("failed to create directory %s: %w", installRoot, err)
	}
	return installRoot, nil
}

func mountDiskToChroot(installRoot string, diskPathIdMap map[string]string, template *config.ImageTemplate) ([]map[string]string, error) {
	var mountPointInfoList []map[string]string
	diskInfo := template.GetDiskConfig()
	partions := diskInfo.Partitions
	for diskId, diskPath := range diskPathIdMap {
		for _, partition := range partions {
			if partition.ID == diskId {
				mountPointInfo := make(map[string]string)
				mountPointInfo["Id"] = diskId
				mountPointInfo["Path"] = diskPath
				mountPointInfo["MountPoint"] = filepath.Join(installRoot, partition.MountPoint)
				if partition.MountPoint == "/boot/efi" {
					if partition.FsType == "fat32" || partition.FsType == "fat16" {
						mountPointInfo["Flags"] = fmt.Sprintf("-t %s -o umask=0077", "vfat")
					} else {
						mountPointInfo["Flags"] = fmt.Sprintf("-t %s -o umask=0077", partition.FsType)
					}
				} else {
					mountPointInfo["Flags"] = fmt.Sprintf("-t %s", partition.FsType)
				}
				mountPointInfoList = append(mountPointInfoList, mountPointInfo)
			}
		}
	}

	if len(mountPointInfoList) == 0 {
		//return nil, fmt.Errorf("no mount points found for the provided diskPathIdMap")
		return nil, nil
	}

	// sort the mountPointInfoList by the partition.MountPoint
	// mount requires order that the "/" mounted first, then "/boot", "/boot/efi", etc.
	sort.Slice(mountPointInfoList, func(i, j int) bool {
		return mountPointInfoList[i]["MountPoint"] < mountPointInfoList[j]["MountPoint"]
	})

	for _, mountPointInfo := range mountPointInfoList {
		mountPoint := mountPointInfo["MountPoint"]
		path := mountPointInfo["Path"]
		flags := mountPointInfo["Flags"]
		if err := mount.MountPath(path, mountPoint, flags); err != nil {
			return nil, fmt.Errorf("failed to mount %s to %s with flags %s: %w", path, mountPoint, flags, err)
		}
	}

	return mountPointInfoList, nil
}

func umountDiskFromChroot(mountPointInfoList []map[string]string) error {
	mountPointInfoListLen := len(mountPointInfoList)
	for i := mountPointInfoListLen - 1; i >= 0; i-- {
		mountPointInfo := mountPointInfoList[i]
		mountPoint := mountPointInfo["MountPoint"]
		err := mount.UmountPath(mountPoint)
		if err != nil {
			return fmt.Errorf("failed to unmount %s: %w", mountPoint, err)
		}
	}
	return nil
}

func preImageOsInstall(installRoot string, template *config.ImageTemplate) error {
	return nil
}

func installImagePkgs(installRoot string, template *config.ImageTemplate) error {
	return nil
}

func updateImageConfig(installRoot string, template *config.ImageTemplate) error {
	err := updateImageHostname(installRoot, template)
	if err != nil {
		return fmt.Errorf("failed to update image hostname: %w", err)
	}
	err = updateImageUsrGroup(installRoot, template)
	if err != nil {
		return fmt.Errorf("failed to update image user/group: %w", err)
	}
	err = updateImageNetwork(installRoot, template)
	if err != nil {
		return fmt.Errorf("failed to update image network: %w", err)
	}
	err = addImageAdditionalFiles(installRoot, template)
	if err != nil {
		return fmt.Errorf("failed to add additional files to image: %w", err)
	}
	return nil
}

func postImageOsInstall(installRoot string, template *config.ImageTemplate) error {
	return nil
}

func updateImageHostname(installRoot string, template *config.ImageTemplate) error {
	return nil
}

func updateImageUsrGroup(installRoot string, template *config.ImageTemplate) error {
	return nil
}

func updateImageNetwork(installRoot string, template *config.ImageTemplate) error {
	return nil
}

func addImageAdditionalFiles(installRoot string, template *config.ImageTemplate) error {
	return nil
}

func buildImageUKI(installRoot string, template *config.ImageTemplate) error {
	return nil
}
