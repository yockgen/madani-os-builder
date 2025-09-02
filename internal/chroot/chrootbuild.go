package chroot

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/open-edge-platform/image-composer/internal/chroot/deb"
	"github.com/open-edge-platform/image-composer/internal/chroot/rpm"
	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/ospackage/debutils"
	"github.com/open-edge-platform/image-composer/internal/ospackage/rpmutils"
	"github.com/open-edge-platform/image-composer/internal/utils/compression"
	"github.com/open-edge-platform/image-composer/internal/utils/file"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
	"github.com/open-edge-platform/image-composer/internal/utils/shell"
)

var log = logger.Logger()

type ChrootBuilder struct {
	targetOsConfigDir string
	targetOsConfig    map[string]interface{}
	chrootBuildDir    string
	chrootPkgCacheDir string
	rpmInstaller      *rpm.RpmInstaller
	debInstaller      *deb.DebInstaller
}

func NewChrootBuilder(targetOs string, targetDist string, targetArch string) (*ChrootBuilder, error) {
	var targetOsConfig map[string]interface{}
	if targetOs == "" || targetDist == "" || targetArch == "" {
		return nil, fmt.Errorf("target OS, distribution, and architecture must be specified")
	}
	globalWorkDir, err := config.WorkDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get global work directory: %w", err)
	}
	globalCache, err := config.CacheDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get global cache dir: %w", err)
	}

	chrootBuildDir := filepath.Join(globalWorkDir, config.ProviderId, "chrootbuild")
	chrootPkgCacheDir := filepath.Join(globalCache, "pkgCache", config.ProviderId)

	targetOsConfigDir, err := config.GetTargetOsConfigDir(targetOs, targetDist)
	if err != nil {
		return nil, fmt.Errorf("failed to get target OS config directory: %w", err)
	}
	targetOsConfigFile := filepath.Join(targetOsConfigDir, "config.yml")
	if _, err := os.Stat(targetOsConfigFile); os.IsNotExist(err) {
		log.Errorf("Target OS config file does not exist: %s", targetOsConfigFile)
		return nil, fmt.Errorf("target OS config file does not exist: %s", targetOsConfigFile)
	}
	targetOsConfigs, err := file.ReadFromYaml(targetOsConfigFile)
	if err != nil {
		log.Errorf("Failed to read target OS config file: %v", err)
		return nil, fmt.Errorf("failed to read target OS config file: %w", err)
	}
	if targetConfig, ok := targetOsConfigs[targetArch]; ok {
		targetOsConfig = targetConfig.(map[string]interface{})
	} else {
		log.Errorf("Target OS %s config for architecture %s not found in %s", targetOs, targetArch, targetOsConfigFile)
		return nil, fmt.Errorf("target OS %s config for architecture %s not found in %s", targetOs, targetArch, targetOsConfigFile)
	}

	return &ChrootBuilder{
		targetOsConfigDir: targetOsConfigDir,
		targetOsConfig:    targetOsConfig,
		chrootBuildDir:    chrootBuildDir,
		chrootPkgCacheDir: chrootPkgCacheDir,
		rpmInstaller:      rpm.NewRpmInstaller(),
		debInstaller:      deb.NewDebInstaller(),
	}, nil
}

func (chrootBuilder *ChrootBuilder) getTargetOsPkgType() string {
	pkgType, ok := chrootBuilder.targetOsConfig["pkgType"]
	if !ok {
		return "unknown"
	}
	if s, ok := pkgType.(string); ok {
		return s
	}
	return "unknown"
}

func (chrootBuilder *ChrootBuilder) getChrootEnvConfig() (map[interface{}]interface{}, error) {
	chrootEnvConfigFile, ok := chrootBuilder.targetOsConfig["chrootenvConfigFile"]
	if !ok {
		log.Errorf("Chroot environment config file not found in target OS config")
		return nil, fmt.Errorf("chroot config file not found in target OS config")
	}
	chrootEnvConfigPath := filepath.Join(chrootBuilder.targetOsConfigDir, chrootEnvConfigFile.(string))
	if _, err := os.Stat(chrootEnvConfigPath); os.IsNotExist(err) {
		log.Errorf("Chroot environment config file does not exist: %s", chrootEnvConfigPath)
		return nil, fmt.Errorf("chroot environment config file does not exist: %s", chrootEnvConfigPath)
	}
	return file.ReadFromYaml(chrootEnvConfigPath)
}

func (chrootBuilder *ChrootBuilder) getChrootEnvEssentialPackageList() ([]string, error) {
	pkgList := []string{}
	chrootEnvConfig, err := chrootBuilder.getChrootEnvConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to read chroot environment config: %w", err)
	}
	if pkgListRaw, ok := chrootEnvConfig["essential"]; ok {
		if pkgListStr, ok := pkgListRaw.([]interface{}); ok {
			for _, pkg := range pkgListStr {
				if pkgStr, ok := pkg.(string); ok {
					pkgList = append(pkgList, pkgStr)
				} else {
					log.Errorf("Invalid package format in chroot environment config: %v", pkg)
					return nil, fmt.Errorf("invalid package format in chroot environment config: %v", pkg)
				}
			}
		} else {
			log.Errorf("Essential packages field is not a list in chroot environment config")
			return nil, fmt.Errorf("essential packages field is not a list in chroot environment config")
		}
	}
	return pkgList, nil
}

func (chrootBuilder *ChrootBuilder) getChrootEnvPackageList() ([]string, error) {
	pkgList := []string{}
	chrootEnvConfig, err := chrootBuilder.getChrootEnvConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to read chroot environment config: %w", err)
	}
	if pkgListRaw, ok := chrootEnvConfig["packages"]; ok {
		if pkgListStr, ok := pkgListRaw.([]interface{}); ok {
			for _, pkg := range pkgListStr {
				if pkgStr, ok := pkg.(string); ok {
					pkgList = append(pkgList, pkgStr)
				} else {
					log.Errorf("Invalid package format in chroot environment config: %v", pkg)
					return nil, fmt.Errorf("invalid package format in chroot environment config: %v", pkg)
				}
			}
		} else {
			log.Errorf("Packages field is not a list in chroot environment config")
			return nil, fmt.Errorf("packages field is not a list in chroot environment config")
		}
	} else {
		log.Errorf("Packages field not found in chroot environment config")
		return nil, fmt.Errorf("packages field not found in chroot environment config")
	}
	return pkgList, nil
}

func (chrootBuilder *ChrootBuilder) downloadChrootEnvPackages() ([]string, []string, error) {
	var pkgsList []string
	var allPkgsList []string

	pkgType := chrootBuilder.getTargetOsPkgType()
	essentialPkgsList, err := chrootBuilder.getChrootEnvEssentialPackageList()
	if err != nil {
		return pkgsList, allPkgsList, fmt.Errorf("failed to get essential packages list: %w", err)
	}
	pkgsList, err = chrootBuilder.getChrootEnvPackageList()
	if err != nil {
		return pkgsList, allPkgsList, fmt.Errorf("failed to get chroot environment package list: %w", err)
	}
	pkgsList = append(essentialPkgsList, pkgsList...)

	if _, err := os.Stat(chrootBuilder.chrootPkgCacheDir); os.IsNotExist(err) {
		if err := os.MkdirAll(chrootBuilder.chrootPkgCacheDir, 0755); err != nil {
			log.Errorf("Failed to create chroot package cache directory: %v", err)
			return pkgsList, allPkgsList, fmt.Errorf("failed to create chroot package cache directory: %w", err)
		}
	}

	dotFilePath := filepath.Join(chrootBuilder.chrootPkgCacheDir, "chrootpkgs.dot")

	if pkgType == "rpm" {
		allPkgsList, err = rpmutils.DownloadPackages(pkgsList, chrootBuilder.chrootPkgCacheDir, dotFilePath)
		if err != nil {
			return pkgsList, allPkgsList, fmt.Errorf("failed to download chroot environment packages: %w", err)
		}
		return pkgsList, allPkgsList, nil
	} else if pkgType == "deb" {
		allPkgsList, err = debutils.DownloadPackages(pkgsList, chrootBuilder.chrootPkgCacheDir, dotFilePath)
		if err != nil {
			return pkgsList, allPkgsList, fmt.Errorf("failed to download chroot environment packages: %w", err)
		}
		return pkgsList, allPkgsList, nil
	} else {
		return pkgsList, allPkgsList, fmt.Errorf("unsupported package type: %s", pkgType)
	}
}

func (chrootBuilder *ChrootBuilder) BuildChrootEnv(targetOs string, targetDist string, targetArch string) error {
	pkgType := chrootBuilder.getTargetOsPkgType()

	chrootTarPath := filepath.Join(chrootBuilder.chrootBuildDir, "chrootenv.tar.gz")
	if _, err := os.Stat(chrootTarPath); err == nil {
		log.Infof("Chroot tarball already exists at %s", chrootTarPath)
		return nil
	}
	chrootEnvPath := filepath.Join(chrootBuilder.chrootBuildDir, "chroot")

	pkgsList, allPkgsList, err := chrootBuilder.downloadChrootEnvPackages()
	if err != nil {
		return fmt.Errorf("failed to download chroot environment packages: %w", err)
	}
	log.Infof("Downloaded %d packages for chroot environment", len(allPkgsList))

	if pkgType == "rpm" {
		if err := chrootBuilder.rpmInstaller.InstallRpmPkg(targetOs, chrootEnvPath, chrootBuilder.chrootPkgCacheDir, allPkgsList); err != nil {
			return fmt.Errorf("failed to install packages in chroot environment: %w", err)
		}
	} else if pkgType == "deb" {
		if err = chrootBuilder.debInstaller.UpdateLocalDebRepo(chrootBuilder.chrootPkgCacheDir, targetArch); err != nil {
			return fmt.Errorf("failed to create debian local repository: %w", err)
		}

		if err := chrootBuilder.debInstaller.InstallDebPkg(chrootBuilder.targetOsConfigDir, chrootEnvPath, chrootBuilder.chrootPkgCacheDir, pkgsList); err != nil {
			return fmt.Errorf("failed to install packages in chroot environment: %w", err)
		}
	} else {
		log.Errorf("Unsupported package type: %s", pkgType)
		return fmt.Errorf("unsupported package type: %s", pkgType)
	}

	if err = compression.CompressFolder(chrootEnvPath, chrootTarPath, "tar.gz", true); err != nil {
		log.Errorf("Failed to compress chroot environment: %v", err)
		return fmt.Errorf("failed to compress chroot environment: %w", err)
	}

	log.Infof("Chroot environment build completed successfully")

	if _, err = shell.ExecCmd("rm -rf "+chrootEnvPath, true, "", nil); err != nil {
		log.Errorf("Failed to remove chroot environment build path: %v", err)
		return fmt.Errorf("failed to remove chroot environment build path: %w", err)
	}

	return nil
}
