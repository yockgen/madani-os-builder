package imageos

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/utils/shell"
)

// Mock executor for shell commands
type MockShellExecutor struct{}

var MockExpectedOutput map[string][]interface{} = map[string][]interface{}{
	"mkdir -p /tmp/test-install":                           {"", nil},
	"rpm --root /tmp/test-install --initdb":               {"", nil},
	"dracut -f /boot/initramfs-5.15.0-generic.img 5.15.0-generic": {"", nil},
	"dracut --force --add systemd-veritysetup --no-hostonly --verbose --kver 5.15.0-generic /boot/initramfs-5.15.0-generic.img": {"", nil},
	"mkdir -p /boot/efi":                                   {"", nil},
	"mkdir -p /boot/efi/EFI/Linux":                         {"", nil},
	"mkdir -p /boot/efi/EFI/BOOT":                          {"", nil},
	"sh -c 'rm -rf /boot/efi/*'":                           {"", nil},
	"cp usr/lib/systemd/boot/efi/systemd-bootx64.efi boot/efi/EFI/BOOT/BOOTX64.EFI": {"", nil},
	"chmod 0444 /tmp/test-install/etc/image-id":            {"", nil},
	"useradd -m -s /bin/bash user":                         {"", nil},
	"usermod -aG sudo user":                                {"", nil},
	"grep '^user:' /etc/passwd":                            {"user:x:1000:1000::/home/user:/bin/bash\n", nil},
	"grep '^user:' /etc/shadow":                            {"user:$6$salt$hash:19000:0:99999:7:::\n", nil},
	"groups user":                                          {"user : user sudo\n", nil},
	"mount -o remount,ro /dev/sda1":                        {"", nil},
	"mkdir -p /tmp":                                        {"", nil},
	"mount -t tmpfs tmpfs /tmp":                            {"", nil},
	"chmod 1777 /tmp":                                      {"", nil},
	"mkdir -p /boot/efi/tmp":                               {"", nil},
	"mount -t tmpfs tmpfs /boot/efi/tmp":                   {"", nil},
	"chmod 1777 /boot/efi/tmp":                             {"", nil},
	"umount /tmp":                                          {"", nil},
	"umount /boot/efi/tmp":                                 {"", nil},
	"veritysetup format /dev/sda1 /dev/sda2":               {"VERITY header information for /dev/sda1\nRoot hash: abcdef123456\n", nil},
}

func mockExecCmd(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	if output, exists := MockExpectedOutput[cmdStr]; exists {
		if output[1] != nil {
			return output[0].(string), output[1].(error)
		} else {
			return output[0].(string), nil
		}
	} else {
		// Return a generic success for unknown commands in mock mode
		return "", nil
	}
}

func (m *MockShellExecutor) ExecCmd(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	return mockExecCmd(cmdStr, sudo, chrootPath, envVal)
}

func (m *MockShellExecutor) ExecCmdSilent(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	return mockExecCmd(cmdStr, sudo, chrootPath, envVal)
}

func (m *MockShellExecutor) ExecCmdWithStream(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	return mockExecCmd(cmdStr, sudo, chrootPath, envVal)
}

func (m *MockShellExecutor) ExecCmdWithInput(inputStr string, cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	return mockExecCmd(cmdStr, sudo, chrootPath, envVal)
}

func setupMockShell() func() {
	originalExecutor := shell.Default
	shell.Default = &MockShellExecutor{}
	return func() { shell.Default = originalExecutor }
}

func TestBuildImageUKI_NonSystemdBoot(t *testing.T) {
	cleanup := setupMockShell()
	defer cleanup()

	installRoot := t.TempDir()
	template := &config.ImageTemplate{
		SystemConfig: config.SystemConfig{
			Bootloader: config.Bootloader{
				Provider: "grub2",
			},
		},
		Image: config.ImageInfo{
			Name: "test-image",
		},
	}

	err := buildImageUKI(installRoot, template)
	if err != nil {
		t.Errorf("unexpected error for non-systemd-boot provider: %v", err)
	}
}

func TestGetRpmPkgInstallList(t *testing.T) {
	tests := []struct {
		name         string
		packages     []string
		expectFirst  string
		expectLast   string
		expectLength int
	}{
		{
			name:         "packages with filesystem first and initramfs last",
			packages:     []string{"filesystem-1.0", "some-pkg", "initramfs-tools"},
			expectFirst:  "filesystem-1.0",
			expectLast:   "initramfs-tools",
			expectLength: 3,
		},
		{
			name:         "packages without special ordering",
			packages:     []string{"pkg1", "pkg2", "pkg3"},
			expectFirst:  "pkg1",
			expectLast:   "pkg3",
			expectLength: 3,
		},
		{
			name:         "empty package list",
			packages:     []string{},
			expectLength: 0,
		},
		{
			name:         "only filesystem packages",
			packages:     []string{"filesystem-base", "filesystem-core"},
			expectFirst:  "filesystem-base",
			expectLength: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := &config.ImageTemplate{
				SystemConfig: config.SystemConfig{
					Packages: tt.packages,
				},
			}
			
			result := getRpmPkgInstallList(template)
			
			if len(result) != tt.expectLength {
				t.Errorf("expected length %d, got %d", tt.expectLength, len(result))
			}
			
			if tt.expectLength > 0 {
				if result[0] != tt.expectFirst {
					t.Errorf("expected first package %s, got %s", tt.expectFirst, result[0])
				}
				
				if tt.expectLast != "" && result[len(result)-1] != tt.expectLast {
					t.Errorf("expected last package %s, got %s", tt.expectLast, result[len(result)-1])
				}
			}
		})
	}
}

func TestGetDebPkgInstallList(t *testing.T) {
	tests := []struct {
		name         string
		packages     []string
		expectFirst  string
		expectLength int
	}{
		{
			name:         "packages with base-files first",
			packages:     []string{"base-files", "some-pkg", "dracut-core", "systemd-boot"},
			expectFirst:  "base-files",
			expectLength: 4,
		},
		{
			name:         "packages without special ordering",
			packages:     []string{"pkg1", "pkg2", "pkg3"},
			expectFirst:  "pkg1",
			expectLength: 3,
		},
		{
			name:         "empty package list",
			packages:     []string{},
			expectLength: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := &config.ImageTemplate{
				SystemConfig: config.SystemConfig{
					Packages: tt.packages,
				},
			}
			
			result := getDebPkgInstallList(template)
			
			if len(result) != tt.expectLength {
				t.Errorf("expected length %d, got %d", tt.expectLength, len(result))
			}
			
			if tt.expectLength > 0 && result[0] != tt.expectFirst {
				t.Errorf("expected first package %s, got %s", tt.expectFirst, result[0])
			}
		})
	}
}

func TestGetImageVersionInfo(t *testing.T) {
	tests := []struct {
		name            string
		targetOS        string
		setupFunc       func(t *testing.T, installRoot string)
		expectError     bool
		expectedVersion string
	}{
		{
			name:     "azure-linux with valid os-release",
			targetOS: "azure-linux",
			setupFunc: func(t *testing.T, installRoot string) {
				etcDir := filepath.Join(installRoot, "etc")
				err := os.MkdirAll(etcDir, 0755)
				if err != nil {
					t.Fatalf("failed to create etc directory: %v", err)
				}
				
				osReleaseContent := `NAME="Azure Linux"
VERSION="3.0.20240101"
ID=azurelinux`
				osReleasePath := filepath.Join(etcDir, "os-release")
				err = os.WriteFile(osReleasePath, []byte(osReleaseContent), 0644)
				if err != nil {
					t.Fatalf("failed to write os-release file: %v", err)
				}
			},
			expectError:     false,
			expectedVersion: "3.0.20240101",
		},
		{
			name:     "azure-linux with missing os-release",
			targetOS: "azure-linux",
			setupFunc: func(t *testing.T, installRoot string) {
				// Don't create os-release file
			},
			expectError: true,
		},
		{
			name:     "edge-microvisor-toolkit with valid os-release",
			targetOS: "edge-microvisor-toolkit",
			setupFunc: func(t *testing.T, installRoot string) {
				etcDir := filepath.Join(installRoot, "etc")
				err := os.MkdirAll(etcDir, 0755)
				if err != nil {
					t.Fatalf("failed to create etc directory: %v", err)
				}
				
				osReleaseContent := `NAME="Edge Microvisor Toolkit"
VERSION="2.1.0"
ID=emt`
				osReleasePath := filepath.Join(etcDir, "os-release")
				err = os.WriteFile(osReleasePath, []byte(osReleaseContent), 0644)
				if err != nil {
					t.Fatalf("failed to write os-release file: %v", err)
				}
			},
			expectError:     false,
			expectedVersion: "2.1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Backup original TargetOs
			originalTargetOs := config.TargetOs
			defer func() {
				config.TargetOs = originalTargetOs
			}()
			
			// Set TargetOs for test
			config.TargetOs = tt.targetOS
			
			installRoot := t.TempDir()
			template := &config.ImageTemplate{}
			
			if tt.setupFunc != nil {
				tt.setupFunc(t, installRoot)
			}
			
			versionInfo, err := getImageVersionInfo(installRoot, template)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			
			if tt.expectedVersion != "" && versionInfo != tt.expectedVersion {
				t.Errorf("expected version %s, got %s", tt.expectedVersion, versionInfo)
			}
		})
	}
}

func TestExtractRootHashPH(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid roothash parameter",
			input:    "console=tty0 roothash=abc-def-123 quiet",
			expected: "abc def 123",
		},
		{
			name:     "roothash at beginning",
			input:    "roothash=xyz-123-456 console=tty0",
			expected: "xyz 123 456",
		},
		{
			name:     "roothash at end",
			input:    "quiet console=tty0 roothash=test-hash",
			expected: "test hash",
		},
		{
			name:     "no roothash parameter",
			input:    "console=tty0 quiet splash",
			expected: "",
		},
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
		{
			name:     "roothash with no dashes",
			input:    "console=tty0 roothash=abcdef123",
			expected: "abcdef123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRootHashPH(tt.input)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestReplaceRootHashPH(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		newRootHash string
		expected    string
	}{
		{
			name:        "replace existing roothash",
			input:       "console=tty0 roothash=old-hash quiet",
			newRootHash: "new-hash",
			expected:    "console=tty0 roothash=new-hash quiet",
		},
		{
			name:        "replace roothash at beginning",
			input:       "roothash=old-hash console=tty0",
			newRootHash: "new-hash",
			expected:    "roothash=new-hash console=tty0",
		},
		{
			name:        "replace roothash at end",
			input:       "console=tty0 quiet roothash=old-hash",
			newRootHash: "new-hash",
			expected:    "console=tty0 quiet roothash=new-hash",
		},
		{
			name:        "no roothash to replace",
			input:       "console=tty0 quiet splash",
			newRootHash: "new-hash",
			expected:    "console=tty0 quiet splash",
		},
		{
			name:        "empty input",
			input:       "",
			newRootHash: "new-hash",
			expected:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := replaceRootHashPH(tt.input, tt.newRootHash)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestGetKernelVersion(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(t *testing.T, installRoot string)
		expectError bool
		expected    string
	}{
		{
			name: "valid kernel file",
			setupFunc: func(t *testing.T, installRoot string) {
				bootDir := filepath.Join(installRoot, "boot")
				err := os.MkdirAll(bootDir, 0755)
				if err != nil {
					t.Fatalf("failed to create boot directory: %v", err)
				}
				
				kernelFile := filepath.Join(bootDir, "vmlinuz-5.15.0-generic")
				err = os.WriteFile(kernelFile, []byte("fake kernel"), 0644)
				if err != nil {
					t.Fatalf("failed to create kernel file: %v", err)
				}
			},
			expectError: false,
			expected:    "5.15.0-generic",
		},
		{
			name: "no kernel files",
			setupFunc: func(t *testing.T, installRoot string) {
				bootDir := filepath.Join(installRoot, "boot")
				err := os.MkdirAll(bootDir, 0755)
				if err != nil {
					t.Fatalf("failed to create boot directory: %v", err)
				}
			},
			expectError: true,
		},
		{
			name: "boot directory doesn't exist",
			setupFunc: func(t *testing.T, installRoot string) {
				// Don't create boot directory
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installRoot := t.TempDir()
			
			if tt.setupFunc != nil {
				tt.setupFunc(t, installRoot)
			}
			
			version, err := getKernelVersion(installRoot)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			
			if version != tt.expected {
				t.Errorf("expected version %s, got %s", tt.expected, version)
			}
		})
	}
}

func TestCopyBootloader(t *testing.T) {
	cleanup := setupMockShell()
	defer cleanup()

	installRoot := t.TempDir()
	
	// Create source directory and file
	srcDir := filepath.Join(installRoot, "usr", "lib", "systemd", "boot", "efi")
	err := os.MkdirAll(srcDir, 0755)
	if err != nil {
		t.Fatalf("failed to create source directory: %v", err)
	}
	
	srcFile := filepath.Join(srcDir, "systemd-bootx64.efi")
	err = os.WriteFile(srcFile, []byte("fake bootloader"), 0644)
	if err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}
	
	// Create destination directory
	dstDir := filepath.Join(installRoot, "boot", "efi", "EFI", "BOOT")
	err = os.MkdirAll(dstDir, 0755)
	if err != nil {
		t.Fatalf("failed to create destination directory: %v", err)
	}
	
	src := filepath.Join("usr", "lib", "systemd", "boot", "efi", "systemd-bootx64.efi")
	dst := filepath.Join("boot", "efi", "EFI", "BOOT", "BOOTX64.EFI")
	
	err = copyBootloader(installRoot, src, dst)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUpdateImageHostname(t *testing.T) {
	installRoot := t.TempDir()
	template := &config.ImageTemplate{
		Image: config.ImageInfo{
			Name: "test-image",
		},
	}

	err := updateImageHostname(installRoot, template)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUpdateImageNetwork(t *testing.T) {
	installRoot := t.TempDir()
	template := &config.ImageTemplate{
		Image: config.ImageInfo{
			Name: "test-image",
		},
	}

	err := updateImageNetwork(installRoot, template)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPreImageOsInstall(t *testing.T) {
	installRoot := t.TempDir()
	template := &config.ImageTemplate{
		Image: config.ImageInfo{
			Name: "test-image",
		},
	}

	err := preImageOsInstall(installRoot, template)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPostImageOsInstall(t *testing.T) {
	installRoot := t.TempDir()
	template := &config.ImageTemplate{
		Image: config.ImageInfo{
			Name: "test-image",
		},
	}
	
	// Setup a valid os-release file for version extraction
	etcDir := filepath.Join(installRoot, "etc")
	err := os.MkdirAll(etcDir, 0755)
	if err != nil {
		t.Fatalf("failed to create etc directory: %v", err)
	}
	
	// Backup and set TargetOs for this test
	originalTargetOs := config.TargetOs
	defer func() {
		config.TargetOs = originalTargetOs
	}()
	config.TargetOs = "azure-linux"
	
	osReleaseContent := `VERSION="1.0.0"`
	osReleasePath := filepath.Join(etcDir, "os-release")
	err = os.WriteFile(osReleasePath, []byte(osReleaseContent), 0644)
	if err != nil {
		t.Fatalf("failed to write os-release file: %v", err)
	}

	versionInfo, err := postImageOsInstall(installRoot, template)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	
	if versionInfo != "1.0.0" {
		t.Errorf("expected version info '1.0.0', got '%s'", versionInfo)
	}
}

func TestCreateUser(t *testing.T) {
	cleanup := setupMockShell()
	defer cleanup()

	installRoot := t.TempDir()
	template := &config.ImageTemplate{
		Image: config.ImageInfo{
			Name: "test-image",
		},
	}

	err := createUser(installRoot, template)
	if err != nil {
		t.Errorf("unexpected error with mock shell: %v", err)
	}
}

func TestVerifyUserCreated(t *testing.T) {
	cleanup := setupMockShell()
	defer cleanup()

	installRoot := t.TempDir()
	username := "user"

	err := verifyUserCreated(installRoot, username)
	if err != nil {
		t.Errorf("unexpected error with mock shell: %v", err)
	}
}

func TestUpdateImageUsrGroup(t *testing.T) {
	cleanup := setupMockShell()
	defer cleanup()

	installRoot := t.TempDir()
	template := &config.ImageTemplate{
		Image: config.ImageInfo{
			Name: "test-image",
		},
	}

	err := updateImageUsrGroup(installRoot, template)
	if err != nil {
		t.Errorf("unexpected error with mock shell: %v", err)
	}
}

func TestGetVerityRootHash(t *testing.T) {
	cleanup := setupMockShell()
	defer cleanup()

	installRoot := t.TempDir()
	partPair := "/dev/sda1 /dev/sda2"

	rootHash, err := getVerityRootHash(partPair, installRoot)
	if err != nil {
		t.Errorf("unexpected error with mock shell: %v", err)
		return
	}
	
	if rootHash != "abcdef123456" {
		t.Errorf("expected root hash 'abcdef123456', got '%s'", rootHash)
	}
}

func TestPrepareVeritySetup(t *testing.T) {
	cleanup := setupMockShell()
	defer cleanup()

	tests := []struct {
		name        string
		partPair    string
		expectError bool
	}{
		{
			name:        "valid partition pair",
			partPair:    "/dev/sda1 /dev/sda2",
			expectError: false,
		},
		{
			name:        "invalid partition pair - empty",
			partPair:    "",
			expectError: true,
		},
		{
			name:        "invalid partition pair - only spaces",
			partPair:    "   ",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installRoot := t.TempDir()
			err := prepareVeritySetup(tt.partPair, installRoot)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestRemoveVerityTmp(t *testing.T) {
	cleanup := setupMockShell()
	defer cleanup()

	installRoot := t.TempDir()
	
	// Create tmp directories to test cleanup
	tmpDir := filepath.Join(installRoot, "tmp")
	veritytmpDir := filepath.Join(installRoot, "boot", "efi", "tmp")
	
	err := os.MkdirAll(tmpDir, 0755)
	if err != nil {
		t.Fatalf("failed to create tmp directory: %v", err)
	}
	
	err = os.MkdirAll(veritytmpDir, 0755)
	if err != nil {
		t.Fatalf("failed to create verity tmp directory: %v", err)
	}
	
	// Call function - should not return error with mock
	removeVerityTmp(installRoot)
}

func TestUpdateInitramfs(t *testing.T) {
	cleanup := setupMockShell()
	defer cleanup()

	tests := []struct {
		name             string
		kernelVersion    string
		immutableEnabled bool
		setupFunc        func(t *testing.T, installRoot string)
		expectError      bool
	}{
		{
			name:             "immutability enabled",
			kernelVersion:    "5.15.0-generic",
			immutableEnabled: true,
			setupFunc:        nil,
			expectError:      false,
		},
		{
			name:             "immutability disabled with existing initrd",
			kernelVersion:    "5.15.0-generic", 
			immutableEnabled: false,
			setupFunc: func(t *testing.T, installRoot string) {
				bootDir := filepath.Join(installRoot, "boot")
				err := os.MkdirAll(bootDir, 0755)
				if err != nil {
					t.Fatalf("failed to create boot directory: %v", err)
				}
				
				initrdFile := filepath.Join(bootDir, "initramfs-5.15.0-generic.img")
				err = os.WriteFile(initrdFile, []byte("fake initrd"), 0644)
				if err != nil {
					t.Fatalf("failed to create initrd file: %v", err)
				}
			},
			expectError: false, // Should skip creation when file exists
		},
		{
			name:             "immutability disabled with missing initrd",
			kernelVersion:    "5.15.0-generic",
			immutableEnabled: false,
			setupFunc: func(t *testing.T, installRoot string) {
				bootDir := filepath.Join(installRoot, "boot")
				err := os.MkdirAll(bootDir, 0755)
				if err != nil {
					t.Fatalf("failed to create boot directory: %v", err)
				}
			},
			expectError: false, // With mock, should succeed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installRoot := t.TempDir()
			template := &config.ImageTemplate{
				SystemConfig: config.SystemConfig{
					Immutability: config.ImmutabilityConfig{
						Enabled: tt.immutableEnabled,
					},
				},
			}
			
			if tt.setupFunc != nil {
				tt.setupFunc(t, installRoot)
			}
			
			err := updateInitramfs(installRoot, tt.kernelVersion, template)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestBuildUKI(t *testing.T) {
	cleanup := setupMockShell()
	defer cleanup()

	installRoot := t.TempDir()
	kernelPath := "/boot/vmlinuz-5.15.0"
	initrdPath := "/boot/initramfs-5.15.0.img"
	cmdlineFile := "/boot/cmdline.conf"
	outputPath := "/boot/efi/EFI/Linux/linux.efi"
	
	template := &config.ImageTemplate{
		SystemConfig: config.SystemConfig{
			Immutability: config.ImmutabilityConfig{
				Enabled: false,
			},
		},
	}
	
	// Create required files
	bootDir := filepath.Join(installRoot, "boot")
	err := os.MkdirAll(bootDir, 0755)
	if err != nil {
		t.Fatalf("failed to create boot directory: %v", err)
	}
	
	// Create cmdline file
	cmdlineContent := "console=tty0 quiet splash"
	cmdlineFullPath := filepath.Join(installRoot, cmdlineFile)
	err = os.WriteFile(cmdlineFullPath, []byte(cmdlineContent), 0644)
	if err != nil {
		t.Fatalf("failed to create cmdline file: %v", err)
	}
	
	err = buildUKI(installRoot, kernelPath, initrdPath, cmdlineFile, outputPath, template)
	if err != nil {
		t.Errorf("unexpected error with mock shell: %v", err)
	}
}
