package shell

import (
	"strings"
)

type MockExecutor struct {
	// mockExpectedOutput contains the mapping of command strings to their expected outputs and errors
	mockExpectedOutput map[string][]interface{}
}

func NewMockExecutor(mockExpectedOutput map[string][]interface{}) *MockExecutor {
	return &MockExecutor{
		mockExpectedOutput: mockExpectedOutput}
}

func (m *MockExecutor) execCmdOverride(cmdStr string, sudo bool, chrootPath string, envVal []string) (bool, string, error) {
	var fallback2Default = true
	for cmdStrKeyword, MockOutput := range m.mockExpectedOutput {
		if strings.Contains(cmdStr, cmdStrKeyword) {
			fallback2Default = false
			if MockOutput[1] != nil {
				return fallback2Default, MockOutput[0].(string), MockOutput[1].(error)
			} else {
				return fallback2Default, MockOutput[0].(string), nil
			}
		}
	}
	return fallback2Default, "", nil
}

func (m *MockExecutor) ExecCmd(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	fallback2Default, output, err := m.execCmdOverride(cmdStr, sudo, chrootPath, envVal)
	if fallback2Default {
		return (&DefaultExecutor{}).ExecCmd(cmdStr, sudo, chrootPath, envVal)
	} else {
		return output, err
	}
}

func (m *MockExecutor) ExecCmdSilent(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	fallback2Default, output, err := m.execCmdOverride(cmdStr, sudo, chrootPath, envVal)
	if fallback2Default {
		return (&DefaultExecutor{}).ExecCmdSilent(cmdStr, sudo, chrootPath, envVal)
	} else {
		return output, err
	}
}

func (m *MockExecutor) ExecCmdWithStream(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	fallback2Default, output, err := m.execCmdOverride(cmdStr, sudo, chrootPath, envVal)
	if fallback2Default {
		return (&DefaultExecutor{}).ExecCmdWithStream(cmdStr, sudo, chrootPath, envVal)
	} else {
		return output, err
	}
}

func (m *MockExecutor) ExecCmdWithInput(inputStr string, cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	fallback2Default, output, err := m.execCmdOverride(cmdStr, sudo, chrootPath, envVal)
	if fallback2Default {
		return (&DefaultExecutor{}).ExecCmdWithInput(inputStr, cmdStr, sudo, chrootPath, envVal)
	} else {
		return output, err
	}
}
