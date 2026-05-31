//go:build !windows

package main

import "fmt"

func resolveRelaunchPath(currentPath string) string {
	return currentPath
}

func runSilentInstaller(installerPath, appPath string) error {
	return fmt.Errorf("self-update is not supported on this platform")
}
