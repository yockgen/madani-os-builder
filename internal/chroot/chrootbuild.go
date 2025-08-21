package chroot

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/ospackage/debutils"
	"github.com/open-edge-platform/image-composer/internal/ospackage/rpmutils"
	"github.com/open-edge-platform/image-composer/internal/utils/compression"
	"github.com/open-edge-platform/image-composer/internal/utils/file"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
	"github.com/open-edge-platform/image-composer/internal/utils/mount"
	"github.com/open-edge-platform/image-composer/internal/utils/shell"
)

var (
	TargetOsConfigDir string                 // targetOsConfigDir is the directory where target OS configuration files are stored.
	TargetOsConfig    map[string]interface{} // TargetOsConfig holds the configuration for the target OS.
	ChrootBuildDir    string                 // ChrootBuildDir is the directory where the chroot build.
	ChrootPkgCacheDir string                 // ChrootPkgCacheDir is the directory where chroot environment packages are cached.
	log               = logger.Logger()      // log is the logger instance for this package.
)

func getHostOsInfo() (map[string]string, error) {
	var hostOsInfo = map[string]string{
		"name":    "",
		"version": "",
		"arch":    "",
	}

	// Get architecture using uname command
	output, err := shell.ExecCmd("uname -m", false, "", nil)
	if err != nil {
		log.Errorf("Failed to get host architecture: %v", err)
		return hostOsInfo, fmt.Errorf("failed to get host architecture: %w", err)
	} else {
		hostOsInfo["arch"] = strings.TrimSpace(output)
	}

	// Read from /etc/os-release if it exists
	if _, err := os.Stat("/etc/os-release"); err == nil {
		file, err := os.Open("/etc/os-release")
		if err == nil {
			defer file.Close()
			scanner := bufio.NewScanner(file)

			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "NAME=") {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						hostOsInfo["name"] = strings.Trim(strings.TrimSpace(parts[1]), "\"")
					}
				} else if strings.HasPrefix(line, "VERSION_ID=") {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						hostOsInfo["version"] = strings.Trim(strings.TrimSpace(parts[1]), "\"")
					}
				}
			}

			log.Infof("Detected OS info: " + hostOsInfo["name"] + " " +
				hostOsInfo["version"] + " " + hostOsInfo["arch"])

			return hostOsInfo, nil
		}
	}

	output, err = shell.ExecCmd("lsb_release -si", false, "", nil)
	if err != nil {
		log.Errorf("Failed to get host OS name: %v", err)
		return hostOsInfo, fmt.Errorf("failed to get host OS name: %w", err)
	} else {
		if output != "" {
			hostOsInfo["name"] = strings.TrimSpace(output)
			output, err = shell.ExecCmd("lsb_release -sr", false, "", nil)
			if err != nil {
				log.Errorf("Failed to get host OS version: %v", err)
				return hostOsInfo, fmt.Errorf("failed to get host OS version: %w", err)
			} else {
				if output != "" {
					hostOsInfo["version"] = strings.TrimSpace(output)
					log.Infof("Detected OS info: " + hostOsInfo["name"] + " " +
						hostOsInfo["version"] + " " + hostOsInfo["arch"])
					return hostOsInfo, nil
				}
			}
		}
	}

	log.Errorf("Failed to detect host OS info!")
	return hostOsInfo, fmt.Errorf("failed to detect host OS info")
}

func GetHostOsPkgManager() (string, error) {
	hostOsInfo, err := getHostOsInfo()
	if err != nil {
		return "", err
	}

	switch hostOsInfo["name"] {
	case "Ubuntu", "Debian", "eLxr":
		return "apt", nil
	case "Fedora", "CentOS", "Red Hat Enterprise Linux":
		return "yum", nil
	case "Microsoft Azure Linux", "Edge Microvisor Toolkit":
		return "tdnf", nil
	default:
		log.Errorf("Unsupported host OS: %s", hostOsInfo["name"])
		return "", fmt.Errorf("unsupported host OS: %s", hostOsInfo["name"])
	}
}

func InitChrootBuildSpace(targetOs string, targetDist string, targetArch string) error {
	globalWorkDir, err := config.WorkDir()
	if err != nil {
		return fmt.Errorf("failed to get global work directory: %w", err)
	}
	globalCache, err := config.CacheDir()
	if err != nil {
		return fmt.Errorf("failed to get global cache dir: %w", err)
	}
	if err := getTargetOsConfig(targetOs, targetDist, targetArch); err != nil {
		return fmt.Errorf("failed to get target OS config: %w", err)
	}
	ChrootBuildDir = filepath.Join(globalWorkDir, config.ProviderId, "chrootbuild")
	ChrootPkgCacheDir = filepath.Join(globalCache, "pkgCache", config.ProviderId)

	return nil
}

func getTargetOsConfig(targetOs, targetDist, targetArch string) error {
	var err error
	TargetOsConfigDir, err = config.GetTargetOsConfigDir(targetOs, targetDist)
	if err != nil {
		return fmt.Errorf("failed to get target OS config directory: %w", err)
	}
	targetOsConfigFile := filepath.Join(TargetOsConfigDir, "config.yml")
	if _, err := os.Stat(targetOsConfigFile); os.IsNotExist(err) {
		log.Errorf("Target OS config file does not exist: %s", targetOsConfigFile)
		return fmt.Errorf("target OS config file does not exist: %s", targetOsConfigFile)
	}
	targetOsConfigs, err := file.ReadFromYaml(targetOsConfigFile)
	if err != nil {
		log.Errorf("Failed to read target OS config file: %v", err)
		return fmt.Errorf("failed to read target OS config file: %w", err)
	}
	if targetOsConfig, ok := targetOsConfigs[targetArch]; ok {
		TargetOsConfig = targetOsConfig.(map[string]interface{})
	} else {
		log.Errorf("Target OS %s config for architecture %s not found in %s", targetOs, targetArch, targetOsConfigFile)
		return fmt.Errorf("target OS %s config for architecture %s not found in %s", targetOs, targetArch, targetOsConfigFile)
	}
	return nil
}

func GetTargetOsPkgType() string {
	pkgType, ok := TargetOsConfig["pkgType"]
	if !ok {
		return "unknown"
	}
	if s, ok := pkgType.(string); ok {
		return s
	}
	return "unknown"
}

func GetTargetOsReleaseVersion() string {
	releaseVersion, ok := TargetOsConfig["releaseVersion"]
	if !ok {
		return "unknown"
	}
	if s, ok := releaseVersion.(string); ok {
		return s
	}
	return "unknown"
}

func getChrootEnvConfig() (map[interface{}]interface{}, error) {
	chrootEnvConfigFile, ok := TargetOsConfig["chrootenvConfigFile"]
	if !ok {
		log.Errorf("Chroot environment config file not found in target OS config")
		return nil, fmt.Errorf("chroot config file not found in target OS config")
	}
	chrootEnvConfigPath := filepath.Join(TargetOsConfigDir, chrootEnvConfigFile.(string))
	if _, err := os.Stat(chrootEnvConfigPath); os.IsNotExist(err) {
		log.Errorf("Chroot environment config file does not exist: %s", chrootEnvConfigPath)
		return nil, fmt.Errorf("chroot environment config file does not exist: %s", chrootEnvConfigPath)
	}
	return file.ReadFromYaml(chrootEnvConfigPath)
}

func GetChrootEnvEssentialPackageList() ([]string, error) {
	pkgList := []string{}
	chrootEnvConfig, err := getChrootEnvConfig()
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
					return nil, fmt.Errorf("invalid package format in chroot environment config: %w", pkg)
				}
			}
		} else {
			log.Errorf("Essential packages field is not a list in chroot environment config")
			return nil, fmt.Errorf("essential packages field is not a list in chroot environment config")
		}
	}
	return pkgList, nil
}

func getChrootEnvPackageList() ([]string, error) {
	pkgList := []string{}
	chrootEnvConfig, err := getChrootEnvConfig()
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

func downloadChrootEnvPackages() ([]string, []string, error) {
	var pkgsList []string
	var allPkgsList []string

	pkgType := GetTargetOsPkgType()
	essentialPkgsList, err := GetChrootEnvEssentialPackageList()
	if err != nil {
		return pkgsList, allPkgsList, fmt.Errorf("failed to get essential packages list: %w", err)
	}
	pkgsList, err = getChrootEnvPackageList()
	if err != nil {
		return pkgsList, allPkgsList, fmt.Errorf("failed to get chroot environment package list: %w", err)
	}
	pkgsList = append(essentialPkgsList, pkgsList...)

	if _, err := os.Stat(ChrootPkgCacheDir); os.IsNotExist(err) {
		if err := os.MkdirAll(ChrootPkgCacheDir, 0755); err != nil {
			log.Errorf("Failed to create chroot package cache directory: %v", err)
			return pkgsList, allPkgsList, fmt.Errorf("failed to create chroot package cache directory: %w", err)
		}
	}

	dotFilePath := filepath.Join(ChrootPkgCacheDir, "chrootpkgs.dot")

	if pkgType == "rpm" {
		allPkgsList, err = rpmutils.DownloadPackages(pkgsList, ChrootPkgCacheDir, dotFilePath)
		if err != nil {
			return pkgsList, allPkgsList, fmt.Errorf("failed to download chroot environment packages: %w", err)
		}
		return pkgsList, allPkgsList, nil
	} else if pkgType == "deb" {
		allPkgsList, err = debutils.DownloadPackages(pkgsList, ChrootPkgCacheDir, dotFilePath)
		if err != nil {
			return pkgsList, allPkgsList, fmt.Errorf("failed to download chroot environment packages: %w", err)
		}
		return pkgsList, allPkgsList, nil
	} else {
		return pkgsList, allPkgsList, fmt.Errorf("unsupported package type: %s", pkgType)
	}
}

// updateRpmDB updates the RPM database in the chroot environment
func updateRpmDB(chrootEnvBuildPath string, rpmList []string) error {
	cmdStr := "rpm -E '%{_db_backend}'"
	hostRpmDbBackend, err := shell.ExecCmd(cmdStr, false, "", nil)
	if err != nil {
		log.Errorf("Failed to get host RPM DB backend: %v", err)
		return fmt.Errorf("failed to get host RPM DB backend: %w", err)
	}
	hostRpmDbBackend = strings.TrimSpace(hostRpmDbBackend)
	chrootRpmDbBackend, err := shell.ExecCmd(cmdStr, false, chrootEnvBuildPath, nil)
	if err != nil {
		log.Errorf("Failed to get chroot RPM DB backend: %v", err)
		return fmt.Errorf("failed to get chroot RPM DB backend: %w", err)
	}
	chrootRpmDbBackend = strings.TrimSpace(chrootRpmDbBackend)
	if hostRpmDbBackend == chrootRpmDbBackend {
		log.Debugf("The host RPM DB: " + hostRpmDbBackend + " matches the chroot RPM DB: " + chrootRpmDbBackend)
		log.Debugf("Not rebuilding the chroot RPM database.")
		return nil
	}

	log.Debugf("The host RPM DB: " + hostRpmDbBackend + " differs from the chroot RPM DB: " + chrootRpmDbBackend)
	log.Debugf("Rebuilding the chroot RPM database.")
	if _, err = shell.ExecCmd("rm -rf /var/lib/rpm/*", true, chrootEnvBuildPath, nil); err != nil {
		log.Errorf("Failed to remove RPM database: %v", err)
		return fmt.Errorf("failed to remove RPM database: %w", err)
	}
	if _, err = shell.ExecCmd("rpm --initdb", false, chrootEnvBuildPath, nil); err != nil {
		log.Errorf("Failed to initialize RPM database: %v", err)
		return fmt.Errorf("failed to initialize RPM database: %w", err)
	}

	chrootPkgDir := filepath.Join(chrootEnvBuildPath, "packages")
	if err = mount.MountPath(ChrootPkgCacheDir, chrootPkgDir, "--bind"); err != nil {
		log.Errorf("Failed to mount package cache directory: %v", err)
		return fmt.Errorf("failed to mount package cache directory: %w", err)
	}

	for _, rpm := range rpmList {
		rpmChrootPath := filepath.Join("/packages", rpm)
		cmdStr := "rpm -i -v --nodeps --noorder --force --justdb " + rpmChrootPath
		if _, err := shell.ExecCmdWithStream(cmdStr, true, chrootEnvBuildPath, nil); err != nil {
			log.Errorf("Failed to update RPM Database for %s in chroot environment: %v", rpm, err)
			return fmt.Errorf("failed to update RPM Database for %s in chroot environment: %w", rpm, err)
		}
	}

	return mount.UmountAndDeletePath(chrootPkgDir)
}

// importGpgKeys imports GPG keys into the chroot environment
func importGpgKeys(targetOs string, chrootEnvBuildPath string) error {
	var cmdStr string
	if targetOs == "edge-microvisor-toolkit" {
		cmdStr = "rpm -q -l edge-repos-shared | grep 'rpm-gpg'"
	} else if targetOs == "azure-linux" {
		cmdStr = "rpm -q -l azurelinux-repos-shared | grep 'rpm-gpg'"
	}

	output, err := shell.ExecCmd(cmdStr, false, chrootEnvBuildPath, nil)
	if err != nil {
		log.Errorf("Failed to get GPG keys: %v", err)
		return fmt.Errorf("failed to get GPG keys: %w", err)
	}
	if output != "" {
		gpgKeys := strings.Split(output, "\n")
		log.Infof("Importing GPG key: " + gpgKeys[0])
		cmdStr = "rpm --import " + gpgKeys[0]
		_, err = shell.ExecCmd(cmdStr, false, chrootEnvBuildPath, nil)
		if err != nil {
			log.Errorf("Failed to import GPG key: %v", err)
			return fmt.Errorf("failed to import GPG key: %w", err)
		}
	} else {
		log.Errorf("No GPG keys found in the chroot environment")
		return fmt.Errorf("no GPG keys found in the chroot environment")
	}
	return nil
}

func installRpmPkg(targetOs, chrootEnvPath string, allPkgsList []string) error {
	chrootRpmDbPath := filepath.Join(chrootEnvPath, "var", "lib", "rpm")
	if _, err := os.Stat(chrootRpmDbPath); os.IsNotExist(err) {
		if _, err := shell.ExecCmd("mkdir -p "+chrootRpmDbPath, true, "", nil); err != nil {
			log.Errorf("Failed to create chroot RPM database directory: %v", err)
			return fmt.Errorf("failed to create chroot environment directory: %w", err)
		}
	}

	err := mount.MountSysfs(chrootEnvPath)
	if err != nil {
		log.Errorf("failed to mount system directories in chroot environment: %v", err)
		return fmt.Errorf("failed to mount system directories in chroot environment: %w", err)
	}

	for _, pkg := range allPkgsList {
		pkgPath := filepath.Join(ChrootPkgCacheDir, pkg)
		if _, err = os.Stat(pkgPath); os.IsNotExist(err) {
			log.Errorf("Package %s does not exist in cache directory: %v", pkg, err)
			err = fmt.Errorf("package %s does not exist in cache directory: %w", pkg, err)
			goto fail
		}
		log.Infof("Installing package %s in chroot environment", pkg)
		cmdStr := fmt.Sprintf("rpm -i -v --nodeps --noorder --force --root %s --define '_dbpath /var/lib/rpm' %s",
			chrootEnvPath, pkgPath)
		var output string
		output, err = shell.ExecCmd(cmdStr, true, "", nil)
		if err != nil {
			log.Errorf("Failed to install package %s: %v, output: %s", pkg, err, output)
			err = fmt.Errorf("failed to install package %s: %w, output: %s", pkg, err, output)
			goto fail
		}
	}

	err = updateRpmDB(chrootEnvPath, allPkgsList)
	if err != nil {
		err = fmt.Errorf("failed to update RPM database in chroot environment: %w", err)
		goto fail
	}
	err = importGpgKeys(targetOs, chrootEnvPath)
	if err != nil {
		err = fmt.Errorf("failed to import GPG keys in chroot environment: %w", err)
		goto fail
	}

	err = StopGPGComponents(chrootEnvPath)
	if err != nil {
		err = fmt.Errorf("failed to stop GPG components in chroot environment: %w", err)
		goto fail
	}

	err = mount.UmountSysfs(chrootEnvPath)
	if err != nil {
		log.Errorf("failed to unmount system directories in chroot environment: %v", err)
		return fmt.Errorf("failed to unmount system directories in chroot environment: %w", err)
	}
	err = mount.CleanSysfs(chrootEnvPath)
	if err != nil {
		log.Errorf("failed to clean system directories in chroot environment: %v", err)
		return fmt.Errorf("failed to clean system directories in chroot environment: %w", err)
	}

	return nil

fail:
	if err := mount.UmountSysfs(chrootEnvPath); err != nil {
		log.Errorf("failed to unmount system directories in chroot environment: %v", err)
	} else {
		log.Infof("Unmounted system directories in chroot environment: %s", chrootEnvPath)
	}
	if err := mount.CleanSysfs(chrootEnvPath); err != nil {
		log.Errorf("failed to clean system directories in chroot environment: %v", err)
	} else {
		log.Infof("Cleaned system directories in chroot environment: %s", chrootEnvPath)
	}
	if _, err := shell.ExecCmd("rm -rf "+chrootEnvPath, true, "", nil); err != nil {
		log.Errorf("failed to remove chroot environment build path: %v", err)
	} else {
		log.Infof("Removed chroot environment build path: %s", chrootEnvPath)
	}
	return err
}

func installDebPkg(targetOs, targetDist, chrootEnvPath string, pkgsList []string) error {
	var err error
	var cmd string

	// from local.list
	repoPath := "/cdrom/cache-repo"
	pkgListStr := strings.Join(pkgsList, ",")

	localRepoConfigPath := filepath.Join(TargetOsConfigDir, "chrootenvconfigs", "local.list")
	if _, err := os.Stat(localRepoConfigPath); os.IsNotExist(err) {
		log.Errorf("Local repository config file does not exist: %s", localRepoConfigPath)
		return fmt.Errorf("local repository config file does not exist: %s", localRepoConfigPath)
	}

	if err := mount.MountPath(ChrootPkgCacheDir, repoPath, "--bind"); err != nil {
		log.Errorf("Failed to mount debian local repository: %v", err)
		return fmt.Errorf("failed to mount debian local repository: %w", err)
	}

	if _, err := os.Stat(chrootEnvPath); os.IsNotExist(err) {
		if err := os.MkdirAll(chrootEnvPath, 0755); err != nil {
			log.Errorf("Failed to create chroot environment directory: %v", err)
			return fmt.Errorf("failed to create chroot environment directory: %w", err)
		}
	}

	cmd = fmt.Sprintf("mmdebstrap "+
		"--variant=custom "+
		"--format=directory "+
		"--aptopt=APT::Authentication::Trusted=true "+
		"--hook-dir=/usr/share/mmdebstrap/hooks/file-mirror-automount "+
		"--include=%s "+
		"--verbose --debug "+
		"-- bookworm %s %s",
		pkgListStr, chrootEnvPath, localRepoConfigPath)

	if _, err = shell.ExecCmdWithStream(cmd, true, "", nil); err != nil {
		log.Errorf("Failed to install debian packages in chroot environment: %v", err)
		goto fail
	}

	if err := mount.UmountPath(repoPath); err != nil {
		log.Errorf("Failed to unmount debian local repository: %v", err)
		return fmt.Errorf("failed to unmount debian local repository: %w", err)
	}

	return nil

fail:
	if err := mount.UmountPath(repoPath); err != nil {
		log.Errorf("failed to unmount debian local repository: %v", err)
	}

	if _, err := shell.ExecCmd("rm -rf "+chrootEnvPath, true, "", nil); err != nil {
		log.Errorf("failed to remove chroot environment build path: %v", err)
		return fmt.Errorf("failed to remove chroot environment build path: %w", err)
	}
	return fmt.Errorf("failed to install debian packages in chroot environment: %w", err)
}

func BuildChrootEnv(targetOs string, targetDist string, targetArch string) error {
	pkgType := GetTargetOsPkgType()
	err := InitChrootBuildSpace(targetOs, targetDist, targetArch)
	if err != nil {
		return fmt.Errorf("failed to initialize chroot build space: %w", err)
	}
	chrootTarPath := filepath.Join(ChrootBuildDir, "chrootenv.tar.gz")
	if _, err := os.Stat(chrootTarPath); err == nil {
		log.Infof("Chroot tarball already exists at %s", chrootTarPath)
		return nil
	}

	chrootEnvPath := filepath.Join(ChrootBuildDir, "chroot")

	pkgsList, allPkgsList, err := downloadChrootEnvPackages()
	if err != nil {
		return fmt.Errorf("failed to download chroot environment packages: %w", err)
	}
	log.Infof("Downloaded %d packages for chroot environment", len(allPkgsList))

	if pkgType == "rpm" {
		if err := installRpmPkg(targetOs, chrootEnvPath, allPkgsList); err != nil {
			return fmt.Errorf("failed to install packages in chroot environment: %w", err)
		}
	} else if pkgType == "deb" {
		if err = UpdateLocalDebRepo(ChrootPkgCacheDir); err != nil {
			return fmt.Errorf("failed to create debian local repository: %w", err)
		}

		if err := installDebPkg(targetOs, targetDist, chrootEnvPath, pkgsList); err != nil {
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
