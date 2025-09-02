package rawmaker

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/open-edge-platform/image-composer/internal/chroot"
	"github.com/open-edge-platform/image-composer/internal/config"

	"github.com/open-edge-platform/image-composer/internal/image/imageconvert"
	"github.com/open-edge-platform/image-composer/internal/image/imagedisc"
	"github.com/open-edge-platform/image-composer/internal/image/imageos"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
	"github.com/open-edge-platform/image-composer/internal/utils/shell"
)

type RawMaker struct {
	imageBuildDir string
	chrootEnv     *chroot.ChrootEnv
	imageOs       *imageos.ImageOs
}

func NewRawMaker(chrootEnv *chroot.ChrootEnv) (*RawMaker, error) {
	globalWorkDir, err := config.WorkDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get global work directory: %w", err)
	}

	imageBuildDir := filepath.Join(globalWorkDir, config.ProviderId, "imagebuild")
	if err := os.MkdirAll(imageBuildDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create imagebuild directory: %w", err)
	}

	return &RawMaker{
		imageBuildDir: imageBuildDir,
		chrootEnv:     chrootEnv,
	}, nil
}

func (rawMaker *RawMaker) BuildRawImage(template *config.ImageTemplate) error {
	var err error
	var versionInfo string
	var newFilePath string

	log := logger.Logger()
	log.Infof("Building raw image for: %s", template.GetImageName())

	imageName := template.GetImageName()
	sysConfigName := template.GetSystemConfigName()
	filePath := filepath.Join(rawMaker.imageBuildDir, sysConfigName, fmt.Sprintf("%s.raw", imageName))

	log.Infof("Creating raw image disk...")
	loopDevPath, diskPathIdMap, err := imagedisc.CreateRawImage(filePath, template)
	if err != nil {
		err = fmt.Errorf("failed to create raw image: %w", err)
		if loopDevPath != "" {
			goto fail
		} else {
			return err
		}
	}

	rawMaker.imageOs, err = imageos.NewImageOs(rawMaker.chrootEnv, template)
	if err != nil {
		return fmt.Errorf("failed to create image OS instance: %v", err)
	}

	versionInfo, err = rawMaker.imageOs.InstallImageOs(diskPathIdMap)
	if err != nil {
		err = fmt.Errorf("failed to install image OS: %w", err)
		goto fail
	}

	err = imagedisc.LoopSetupDelete(loopDevPath)
	if err != nil {
		return fmt.Errorf("failed to detach loopback device: %w", err)
	}

	newFilePath = filepath.Join(rawMaker.imageBuildDir, sysConfigName, fmt.Sprintf("%s-%s.raw", imageName, versionInfo))
	if _, err := shell.ExecCmd(fmt.Sprintf("mv %s %s", filePath, newFilePath), true, "", nil); err != nil {
		return fmt.Errorf("failed to rename raw image file: %w", err)
	}
	filePath = newFilePath

	err = imageconvert.ConvertImageFile(filePath, template)
	if err != nil {
		return fmt.Errorf("failed to convert image file: %w", err)
	}
	return nil

fail:
	detachErr := imagedisc.LoopSetupDelete(loopDevPath)
	if detachErr != nil {
		log.Errorf("Failed to detach loopback device after error: %v", detachErr)
	}

	if _, err := os.Stat(filePath); err == nil {
		if _, rmErr := shell.ExecCmd(fmt.Sprintf("rm -f %s", filePath), true, "", nil); rmErr != nil {
			log.Errorf("Failed to remove raw image file %s after error: %v", filePath, rmErr)
		}
	}
	return fmt.Errorf("error during raw image build: %w", err)
}
