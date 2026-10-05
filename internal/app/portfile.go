package app

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const portFileName = ".overview-port"

// portInfo is the discovery record written next to the vault so MCP and WebDAV
// clients can find the running server's address.
type portInfo struct {
	Addr string `json:"addr"`
	PID  int    `json:"pid"`
}

func writePortFile(dataDir, addr string) (string, error) {
	path := filepath.Join(dataDir, portFileName)
	b, err := json.Marshal(portInfo{Addr: addr, PID: os.Getpid()})
	if err != nil {
		return "", err
	}
	b = append(b, '\n')
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func removePortFile(path string) error {
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
