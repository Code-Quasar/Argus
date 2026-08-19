package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

func startBackground() error {
	store, err := OpenDefault()
	if err != nil {
		return err
	}
	pidPath, err := DefaultPIDPath()
	if err != nil {
		return err
	}
	if pid, ok := readPID(pidPath); ok && processRunning(pid) {
		return fmt.Errorf("Argus is already running with pid %d", pid)
	}

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve Argus executable: %w", err)
	}
	passwordReader, passwordWriter, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("create password pipe: %w", err)
	}
	logPath, err := DefaultLogPath()
	if err != nil {
		passwordReader.Close()
		passwordWriter.Close()
		return err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		passwordReader.Close()
		passwordWriter.Close()
		return fmt.Errorf("open server log: %w", err)
	}

	child := exec.Command(executable, "serve", "--port", strconv.Itoa(servePort), "--daemon")
	child.Stdin = passwordReader
	child.Stdout = logFile
	child.Stderr = logFile
	if err := child.Start(); err != nil {
		passwordReader.Close()
		passwordWriter.Close()
		logFile.Close()
		return fmt.Errorf("start background server: %w", err)
	}
	if _, err := passwordWriter.WriteString(store.password + "\n"); err != nil {
		passwordReader.Close()
		passwordWriter.Close()
		logFile.Close()
		_ = child.Process.Kill()
		return fmt.Errorf("send server password: %w", err)
	}
	passwordReader.Close()
	passwordWriter.Close()
	logFile.Close()
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(child.Process.Pid)+"\n"), 0600); err != nil {
		_ = child.Process.Kill()
		return fmt.Errorf("write server pid: %w", err)
	}
	fmt.Printf("Argus started in background with pid %d\n", child.Process.Pid)
	fmt.Printf("Logs: %s\n", logPath)
	return nil
}

func readPID(path string) (int, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	return pid, err == nil && pid > 0
}

func processRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the background Argus server",
	RunE: func(cmd *cobra.Command, args []string) error {
		pidPath, err := DefaultPIDPath()
		if err != nil {
			return err
		}
		pid, ok := readPID(pidPath)
		if !ok {
			return errors.New("Argus is not running")
		}
		process, err := os.FindProcess(pid)
		if err != nil || process.Signal(syscall.SIGTERM) != nil {
			_ = os.Remove(pidPath)
			return fmt.Errorf("could not stop Argus process %d", pid)
		}
		_ = os.Remove(pidPath)
		fmt.Printf("Stopped Argus process %d\n", pid)
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show whether the background Argus server is running",
	RunE: func(cmd *cobra.Command, args []string) error {
		pidPath, err := DefaultPIDPath()
		if err != nil {
			return err
		}
		pid, ok := readPID(pidPath)
		if ok && processRunning(pid) {
			fmt.Printf("Argus is running with pid %d\n", pid)
			return nil
		}
		_ = os.Remove(pidPath)
		fmt.Println("Argus is not running")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd, statusCmd)
}
