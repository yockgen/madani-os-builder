package file

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/open-edge-platform/image-composer/internal/utils/shell"
	"gopkg.in/yaml.v3"
)

// IsSubPath checks if the target path is a subpath of the base path
func IsSubPath(base, target string) (bool, error) {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return false, err
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return false, err
	}
	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return false, err
	}
	// rel == "." means same dir, rel starting with ".." means not subpath
	if rel == "." {
		return true, nil
	}
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return false, nil
	}
	return true, nil
}

// GetRootPath returns the root path of the application
func GetRootPath() (string, error) {

	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}
	return dir, nil
}

func GetGeneralConfigDir() (string, error) {
	rootPath, err := GetRootPath()
	if err != nil {
		return "", fmt.Errorf("failed to get root path: %w", err)
	}
	configDir := filepath.Join(rootPath, "config", "general")
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		return "", fmt.Errorf("general config directory does not exist: %s", configDir)
	}
	return configDir, nil
}

func GetTargetOsConfigDir(targetOs, targetDist string) (string, error) {
	rootPath, err := GetRootPath()
	if err != nil {
		return "", fmt.Errorf("failed to get root path: %w", err)
	}
	configDir := filepath.Join(rootPath, "config", "osv", targetOs, targetDist)
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		return "", fmt.Errorf("target OS config directory does not exist: %s", configDir)
	}
	return configDir, nil
}

func ReplacePlaceholdersInFile(placeholder, value, filePath string) error {
	sedCmd := fmt.Sprintf("sed -i 's|%s|%s|g' %s", placeholder, value, filePath)
	if _, err := shell.ExecCmd(sedCmd, true, "", nil); err != nil {
		return fmt.Errorf("failed to replace placeholder %s with %s in file %s: %w", placeholder, value, filePath, err)
	}
	return nil
}

func GetFileList(dir string) ([]string, error) {
	var fileList []string
	output, err := shell.ExecCmd("ls "+dir, true, "", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list files in directory %s: %w", dir, err)
	}
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue // skip empty lines
		}
		filesInLine := strings.Split(line, " ")
		fileList = append(fileList, filesInLine...)
	}
	return fileList, nil
}

func Read(filePath string) (string, error) {
	content, err := shell.ExecCmd("cat "+filePath, true, "", nil)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
	}
	return content, nil
}

func Write(data, dst string) error {
	cmdStr := fmt.Sprintf("echo -e '%s' | sudo tee %s", data, dst)
	if _, err := shell.ExecCmd(cmdStr, true, "", nil); err != nil {
		return fmt.Errorf("failed to write %s to file %s: %w", data, dst, err)
	}
	return nil
}

func Append(data, dst string) error {
	cmdStr := fmt.Sprintf("echo -e '%s' | sudo tee -a %s", data, dst)
	if _, err := shell.ExecCmd(cmdStr, true, "", nil); err != nil {
		return fmt.Errorf("failed to append %s to file %s: %w", data, dst, err)
	}
	return nil
}

// ReadFromJSON reads a JSON file and returns its contents as a map
// If the file doesn't exist or is empty, returns an empty map
func ReadFromJSON(jsonFile string) (map[string]interface{}, error) {
	// Initialize empty map for result
	result := make(map[string]interface{})

	// Check if file exists
	if _, err := os.Stat(jsonFile); os.IsNotExist(err) {
		return result, fmt.Errorf("file does not exist: %s", jsonFile)
	}

	// Open the file
	file, err := os.Open(jsonFile)
	if err != nil {
		return result, err
	}
	defer file.Close()

	// Get file info to check if it's empty
	fileInfo, err := file.Stat()
	if err != nil {
		return result, err
	}

	// Return empty map if file is empty
	if fileInfo.Size() == 0 {
		return result, fmt.Errorf("file is empty: %s", jsonFile)
	}

	// Decode JSON content
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&result); err != nil {
		return result, err
	}

	return result, nil
}

// WriteToJSON writes a map to a JSON file with specified indentation
func WriteToJSON(jsonFile string, data map[string]interface{}, indent int) error {
	// Create or truncate the file
	file, err := os.Create(jsonFile)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create encoder with indentation
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", strings.Repeat(" ", indent))

	// Encode and write the data
	if err := encoder.Encode(data); err != nil {
		return err
	}

	return nil
}

func decodeYAML(file *os.File, result *map[interface{}]interface{}) error {
	decoder := yaml.NewDecoder(file)
	var raw map[interface{}]interface{}
	if err := decoder.Decode(&raw); err != nil {
		return err
	}
	*result = raw
	return nil
}

func ReadFromYaml(yamlFile string) (map[interface{}]interface{}, error) {
	// Initialize empty map for result
	result := make(map[interface{}]interface{})

	// Check if file exists
	if _, err := os.Stat(yamlFile); os.IsNotExist(err) {
		return result, fmt.Errorf("file does not exist: %s", yamlFile)
	}

	// Open the file
	file, err := os.Open(yamlFile)
	if err != nil {
		return result, err
	}
	defer file.Close()

	// Get file info to check if it's empty
	fileInfo, err := file.Stat()
	if err != nil {
		return result, err
	}

	// Return empty map if file is empty
	if fileInfo.Size() == 0 {
		return result, fmt.Errorf("file is empty: %s", yamlFile)
	}

	// Decode YAML content (assuming a function DecodeYAML exists)
	if err := decodeYAML(file, &result); err != nil {
		return result, err
	}

	return result, nil
}

func CopyFile(srcFile, dstFile, flags string, sudo bool) error {
	srcFilePath, err := filepath.Abs(srcFile)
	if err != nil {
		return fmt.Errorf("failed to get absolute path of source file: %w", err)
	}
	if _, err := os.Stat(srcFilePath); os.IsNotExist(err) {
		return fmt.Errorf("source file does not exist: %s", srcFilePath)
	}

	dstFilePath, err := filepath.Abs(dstFile)
	if err != nil {
		return fmt.Errorf("failed to get absolute path of destination file: %w", err)
	}
	dstDir := filepath.Dir(dstFilePath)
	if _, err := os.Stat(dstDir); os.IsNotExist(err) {
		if _, err := shell.ExecCmd("mkdir -p "+dstDir, sudo, "", nil); err != nil {
			return fmt.Errorf("failed to create directory for destination file: %w", err)
		}
	}
	var cmdStr string
	if flags == "" {
		cmdStr = fmt.Sprintf("cp %s %s", srcFilePath, dstFilePath)
	} else {
		cmdStr = fmt.Sprintf("cp %s %s %s", flags, srcFilePath, dstFilePath)
	}
	if _, err := shell.ExecCmd(cmdStr, sudo, "", nil); err != nil {
		return fmt.Errorf("failed to copy file from %s to %s: %w", srcFilePath, dstFilePath, err)
	}
	return nil
}

func CopyDir(srcDir, dstDir, flags string, sudo bool) error {
	srcDirPath, err := filepath.Abs(srcDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path of source directory: %w", err)
	}
	if _, err := os.Stat(srcDirPath); os.IsNotExist(err) {
		return fmt.Errorf("source directory does not exist: %s", srcDirPath)
	}

	dstDirPath, err := filepath.Abs(dstDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path of destination directory: %w", err)
	}
	if _, err := os.Stat(dstDirPath); os.IsNotExist(err) {
		if _, err := shell.ExecCmd("mkdir -p "+dstDirPath, sudo, "", nil); err != nil {
			return fmt.Errorf("failed to create destination directory: %w", err)
		}
	}

	var cmdStr string
	if flags == "" {
		cmdStr = fmt.Sprintf("cp -r %s/* %s", srcDirPath, dstDirPath)
	} else {
		cmdStr = fmt.Sprintf("cp -r %s %s/* %s", flags, srcDirPath, dstDirPath)
	}
	if _, err := shell.ExecCmd(cmdStr, sudo, "", nil); err != nil {
		return fmt.Errorf("failed to copy directory from %s to %s: %w", srcDirPath, dstDirPath, err)
	}
	return nil
}
