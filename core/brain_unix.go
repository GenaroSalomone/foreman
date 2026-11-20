//go:build !windows

package main

import (
	"os"
	"os/exec"
	"syscall"
)

// realExecve is `exec claude …`: the process becomes claude.
func realExecve(path string, argv, env []string) error { return syscall.Exec(path, argv, env) }

// realDetach starts a process in its OWN session, its output appended to log (0600): the arm of a self-reset must
// outlive the agent it is about to replace.
func realDetach(path string, args []string, env []string, log string) (int, error) {
	out, err := os.OpenFile(log, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return 0, err
	}
	defer out.Close()
	null, err := os.Open(os.DevNull)
	if err != nil {
		return 0, err
	}
	defer null.Close()
	cmd := exec.Command(path, args...)
	cmd.Env, cmd.Stdin, cmd.Stdout, cmd.Stderr = env, null, out, out
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	cmd.Process.Release()
	return pid, nil
}

// realSpawn is `nohup <cmd> >/dev/null 2>&1 &`: started, not waited for, and out of the pane's session.
func realSpawn(path string, argv []string, env []string) {
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return
	}
	defer null.Close()
	cmd := exec.Command(path, argv[1:]...)
	cmd.Env, cmd.Stdin, cmd.Stdout, cmd.Stderr = env, null, null, null
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if cmd.Start() == nil {
		cmd.Process.Release()
	}
}

// tryLock takes an exclusive lock on path without waiting: held=false when another process holds it. release frees it.
func tryLock(path string) (release func(), held bool, err error) {
	f, held, err := tryLockFile(path)
	if !held {
		return nil, false, err
	}
	return func() { f.Close() }, true, nil // closing the descriptor drops the lock
}

// tryLockFile is tryLock's descriptor: the caller closes it to let go.
func tryLockFile(path string) (f *os.File, held bool, err error) {
	f, err = os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, false, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if err == syscall.EWOULDBLOCK {
			return nil, false, nil
		}
		return nil, false, err
	}
	return f, true, nil
}
