package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func readStatsCache(accountPath string) (*StatsCache, error) {
	data, err := os.ReadFile(filepath.Join(accountPath, "stats-cache.json"))
	if err != nil {
		return nil, err
	}
	var sc StatsCache
	if err := json.Unmarshal(data, &sc); err != nil {
		return nil, err
	}
	return &sc, nil
}

func readCredentials(accountPath string) (*Credentials, error) {
	data, err := os.ReadFile(filepath.Join(accountPath, ".credentials.json"))
	if err != nil {
		return nil, err
	}
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}
	return &creds, nil
}

func readSessions(accountPath string) []SessionFile {
	sessionsDir := filepath.Join(accountPath, "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return nil
	}
	var sessions []SessionFile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(sessionsDir, entry.Name()))
		if err != nil {
			continue
		}
		var s SessionFile
		if err := json.Unmarshal(data, &s); err != nil {
			continue
		}
		sessions = append(sessions, s)
	}
	return sessions
}

func readNotifications(accountPath string) *NotificationStates {
	data, err := os.ReadFile(filepath.Join(accountPath, "config", "notification_states.json"))
	if err != nil {
		return nil
	}
	var ns NotificationStates
	if err := json.Unmarshal(data, &ns); err != nil {
		return nil
	}
	return &ns
}
