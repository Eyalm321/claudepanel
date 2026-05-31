//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"syscall"
)

func runSilentInstaller(installerPath, appPath string) error {
	psCommand := fmt.Sprintf(
		`Start-Sleep -Seconds 1; Stop-Process -Name "ClaudePanel" -Force -ErrorAction SilentlyContinue; Start-Process -FilePath "%s" -ArgumentList "/S" -Wait; Start-Process -FilePath "%s"`,
		installerPath, appPath,
	)

	cmd := exec.Command("powershell", "-NoProfile", "-Command", psCommand)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	return cmd.Start()
}
