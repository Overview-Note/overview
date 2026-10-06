//go:build windows || darwin || desktop

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Overview-Note/overview/internal/config"
)

// runOnboarding walks a first-time user through choosing a vault. It must run
// on the Wails event goroutine while the main loop is running, because it opens
// modal native dialogs. It returns the chosen directory, or "" when the user
// cancelled every picker (in which case the caller falls back to the default).
func (d *desktop) runOnboarding(def string) string {
	if d.confirm("设置数据目录", strings.Join([]string{
		"欢迎使用 Overview。",
		"",
		"Overview 会把笔记、附件和搜索索引保存在一个「数据目录」（vault）中，你可以稍后在设置里更改。",
		"",
		"是否使用默认位置？",
		def,
	}, "\n")) {
		return def
	}
	if d.confirm("打开已有 vault", strings.Join([]string{
		"是否打开一个已有的 Overview vault？",
		"",
		"请选择包含 notes/ 或 overview.db 的目录。",
		"",
		"选择「否」将改为新建一个 vault。",
	}, "\n")) {
		return d.pickVault("选择已有的 vault", filepath.Dir(def))
	}
	return d.pickVault("选择新 vault 的位置", filepath.Dir(def))
}

// pickVault opens a directory picker and validates the selection, re-prompting
// on an invalid directory until the user picks a valid one or cancels.
func (d *desktop) pickVault(title, start string) string {
	for {
		chosen, err := d.pickDirectory(title, start)
		if err != nil {
			d.logger.Warn("directory picker failed", "error", err)
			return ""
		}
		if chosen == "" {
			return ""
		}
		abs, err := config.PrepareDataDir(chosen)
		if err != nil {
			d.errorDialog("无法使用该目录", err.Error())
			start = filepath.Dir(chosen)
			continue
		}
		return abs
	}
}

// changeVault persists a new vault directory. An empty dataDir opens a native
// directory picker. It returns the resolved directory and whether it changed;
// a cancelled picker returns ("", false, nil).
func (d *desktop) changeVault(dataDir string) (string, bool, error) {
	if d.explicitDataDir {
		return "", false, errors.New("数据目录由 --data-dir 或 OVERVIEW_DATA_DIR 指定，无法在此更改")
	}
	if strings.TrimSpace(dataDir) == "" {
		chosen := d.pickVault("选择数据目录", d.cfg.DataDir)
		if chosen == "" {
			return "", false, nil
		}
		dataDir = chosen
	}
	abs, err := config.PrepareDataDir(dataDir)
	if err != nil {
		return "", false, err
	}
	if filepath.Clean(abs) == filepath.Clean(d.cfg.DataDir) {
		return abs, false, nil
	}
	if err := config.SaveDesktopConfig(config.DesktopConfig{DataDir: abs}); err != nil {
		return "", false, err
	}
	d.logger.Info("data directory changed", "dataDir", abs)
	return abs, true, nil
}

// restart launches a fresh copy of the running executable, then quits. The new
// process waits for this one to exit before it initialises, so the
// single-instance and vault locks are free by the time it starts.
func (d *desktop) restart() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := spawnRestart(exe, os.Getpid(), d.explicitDataDir, d.cfg.DataDir); err != nil {
		return err
	}
	go func() {
		time.Sleep(300 * time.Millisecond)
		d.quit()
	}()
	return nil
}

// restartWaitPIDEnv names the environment variable carrying the PID the
// relaunched process must wait for.
const restartWaitPIDEnv = "OVERVIEW_RESTART_WAIT_PID"

// spawnRestart starts a detached copy of exe. The copy inherits an environment
// that tells it to wait for pid before starting.
func spawnRestart(exe string, pid int, explicit bool, dataDir string) error {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// `start` detaches the new process from this one via ShellExecute.
		cmd = exec.Command("cmd", "/c", "start \"\" \""+exe+"\"")
	} else {
		cmd = exec.Command("sh", "-c", "exec \""+exe+"\"")
	}
	cmd.Env = append(restartEnv(explicit, dataDir), restartWaitPIDEnv+"="+strconv.Itoa(pid))
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = detachedProcAttr()
	return cmd.Start()
}

// restartEnv builds the child environment. The data directory env var is
// dropped unless it was an explicit choice, so a relaunch after a vault change
// picks up the persisted configuration instead of the previous directory.
func restartEnv(explicit bool, dataDir string) []string {
	env := os.Environ()
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		if strings.HasPrefix(kv, "OVERVIEW_DATA_DIR=") || strings.HasPrefix(kv, restartWaitPIDEnv+"=") {
			continue
		}
		out = append(out, kv)
	}
	if explicit {
		out = append(out, "OVERVIEW_DATA_DIR="+dataDir)
	}
	return out
}

// waitForPredecessor blocks a relaunched process until the process that
// restarted it has exited. It is a no-op for a normal launch.
func waitForPredecessor() {
	raw := os.Getenv(restartWaitPIDEnv)
	if raw == "" {
		return
	}
	pid, err := strconv.Atoi(raw)
	if err != nil || pid <= 0 {
		return
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// confirm shows a native Yes/No question dialog and reports whether the user
// chose Yes. Dismissing the dialog counts as No.
func (d *desktop) confirm(title, message string) bool {
	result := make(chan bool, 1)
	dialog := d.wails.Dialog.Question().SetTitle(title).SetMessage(message)
	yes := dialog.AddButton("Yes")
	no := dialog.AddButton("No")
	yes.OnClick(func() { result <- true })
	no.OnClick(func() { result <- false })
	dialog.SetDefaultButton(yes)
	dialog.SetCancelButton(no)
	dialog.Show()

	if runtime.GOOS == "windows" {
		// Show blocks until the dialog closes on Windows, so the callback has
		// already run; an empty result means it was dismissed without a choice.
		select {
		case v := <-result:
			return v
		default:
			return false
		}
	}
	// On macOS and Linux the dialog renders asynchronously, so wait for the
	// button callback.
	select {
	case v := <-result:
		return v
	case <-time.After(10 * time.Minute):
		return false
	}
}

// errorDialog shows a native error dialog.
func (d *desktop) errorDialog(title, message string) {
	d.wails.Dialog.Error().SetTitle(title).SetMessage(message).Show()
}

// pickDirectory opens a native directory picker, allowing new folders.
func (d *desktop) pickDirectory(title, start string) (string, error) {
	dialog := d.wails.Dialog.OpenFile().
		SetTitle(title).
		SetButtonText("选择").
		CanChooseFiles(false).
		CanChooseDirectories(true).
		CanCreateDirectories(true)
	if start != "" {
		dialog.SetDirectory(start)
	}
	chosen, err := dialog.PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(chosen), nil
}
