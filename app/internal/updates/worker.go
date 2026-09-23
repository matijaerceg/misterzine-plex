package updates

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

type Status struct {
	Updated float64  `json:"updated"`
	Stage   string   `json:"stage"`
	Message string   `json:"message"`
	PID     int      `json:"pid"`
	Release *Release `json:"release"`
}

func (s Status) Busy() bool {
	return s.Stage == "download" || s.Stage == "verify" || s.Stage == "install" || s.Stage == "activating"
}
func ReadStatus(root string) Status {
	var s Status
	data, err := os.ReadFile(filepath.Join(root, "updates/status.json"))
	if err != nil {
		return s
	}
	if len(data) > 128<<10 || json.Unmarshal(data, &s) != nil {
		return Status{Stage: "failed", Message: "Could not read update status."}
	}
	if s.Busy() && (s.PID <= 0 || syscall.Kill(s.PID, 0) != nil) {
		s.Stage = "failed"
		s.Message = "The update stopped. Your current version will keep working."
	}
	return s
}

// Copy the worker before starting it: installing a new runtime must not change
// the code supervising the current update or its recovery path.
func Start(root, action string, release *Release) error {
	if action != "prepare" && action != "activate" {
		return fmt.Errorf("invalid update action")
	}
	folder := filepath.Join(root, "updates")
	if err := os.MkdirAll(folder, 0700); err != nil {
		return err
	}
	worker, err := os.MkdirTemp(folder, "worker-")
	if err != nil {
		return err
	}
	for _, name := range []string{"manager.py", "update_service.py", "catalogue.py", "menu_launcher.py"} {
		data, e := os.ReadFile(filepath.Join(root, name))
		if e != nil {
			return fmt.Errorf("run MisterZine-Plex-Install to add update support")
		}
		if e = os.WriteFile(filepath.Join(worker, name), data, 0600); e != nil {
			return e
		}
	}
	args := []string{filepath.Join(worker, "update_service.py"), action, "--card", filepath.Dir(root)}
	if release != nil {
		if err = release.Validate(); err != nil {
			return err
		}
		data, _ := json.Marshal(release)
		request := filepath.Join(worker, "request.json")
		if err = os.WriteFile(request, data, 0600); err != nil {
			return err
		}
		args = append(args, "--request", request)
	}
	log, err := os.OpenFile(filepath.Join(folder, "worker.log"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	cmd := exec.Command("python3", args...)
	// The app marks its descendants as playback-owned for cleanup. An updater
	// must survive that cleanup when it stops the app for activation.
	for _, item := range os.Environ() {
		if !strings.HasPrefix(item, "MISTERZINE_PLEX_OWNER=") && !strings.HasPrefix(item, "MISTERZINE_PLEX_READY_FILE=") {
			cmd.Env = append(cmd.Env, item)
		}
	}
	cmd.Stdout = log
	cmd.Stderr = log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err = cmd.Start(); err != nil {
		log.Close()
		return err
	}
	go func() { cmd.Wait(); log.Close() }()
	return nil
}
