package browser

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	lockFile       = "lock"
	parentLockFile = ".parentlock"
	lockSeparator  = ":+"
)

func lockPath(profile string) string {
	return filepath.Join(profile, lockFile)
}

func lockOwner(profile string) (int, bool) {
	target, err := os.Readlink(lockPath(profile))
	if err != nil {
		return 0, false
	}

	_, pidStr, ok := strings.Cut(target, lockSeparator)
	if !ok {
		return 0, false
	}

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, false
	}

	return pid, true
}

func alive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	return proc.Signal(syscall.Signal(0)) == nil
}

func clearStaleLock(profile string) {
	pid, ok := lockOwner(profile)
	if !ok || alive(pid) {
		return
	}

	log.Printf("removing stale browser profile lock (pid %d is gone)", pid)

	os.Remove(lockPath(profile))
	os.Remove(filepath.Join(profile, parentLockFile))
}

func waitExited(profile string, timeout time.Duration) {
	pid, ok := lockOwner(profile)
	if !ok {
		return
	}

	deadline := time.Now().Add(timeout)

	for alive(pid) && time.Now().Before(deadline) {
		time.Sleep(closePoll)
	}
}
