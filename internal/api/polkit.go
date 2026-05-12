package api

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/godbus/dbus/v5"
)

const (
	polkitInterface = "org.freedesktop.PolicyKit1.Authority"
	polkitPath      = "/org/freedesktop/PolicyKit1/Authority"
	polkitDest      = "org.freedesktop.PolicyKit1"
)

func checkAuthorization(pid int, action string) (bool, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return false, fmt.Errorf("failed to connect to system bus: %v", err)
	}

	startTime, err := getProcessStartTime(pid)
	if err != nil {
		return false, fmt.Errorf("failed to get process start time: %v", err)
	}

	subject := struct {
		Kind    string            `dbus:"kind"`
		Details map[string]dbus.Variant `dbus:"details"`
	}{
		Kind: "unix-process",
		Details: map[string]dbus.Variant{
			"pid":        dbus.MakeVariant(uint32(pid)),
			"start-time": dbus.MakeVariant(startTime),
		},
	}

	var result struct {
		IsAuthorized bool              `dbus:"is_authorized"`
		IsChallenge  bool              `dbus:"is_challenge"`
		Details      map[string]string `dbus:"details"`
	}

	obj := conn.Object(polkitDest, polkitPath)
	err = obj.Call(polkitInterface+".CheckAuthorization", 0, subject, action, map[string]string{}, uint32(1), "").Store(&result)
	if err != nil {
		return false, fmt.Errorf("polkit call failed: %v", err)
	}

	return result.IsAuthorized, nil
}

func getProcessStartTime(pid int) (uint64, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, err
	}

	fields := strings.Fields(string(data))
	if len(fields) < 22 {
		return 0, fmt.Errorf("invalid stat file for pid %d", pid)
	}

	// The 22nd field is the start time in jiffies after system boot
	startTime, err := strconv.ParseUint(fields[21], 10, 64)
	if err != nil {
		return 0, err
	}

	return startTime, nil
}
