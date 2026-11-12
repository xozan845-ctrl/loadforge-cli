package cli

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
)

func formatBatteryWarning(temp float64) string {
	return fmt.Sprintf("La batería está a %.1f °C: el rendimiento del cliente puede caer por throttling térmico. Enfría el dispositivo o acorta la prueba.", temp)
}

// acquireWakeLock keeps Android from suspending Termux during the test. It is
// a no-op outside Termux or when termux-api is not installed.
func acquireWakeLock() func() {
	if runtime.GOOS != "android" {
		return func() {}
	}
	lock, err := exec.LookPath("termux-wake-lock")
	if err != nil {
		return func() {}
	}
	unlock, err := exec.LookPath("termux-wake-unlock")
	if err != nil {
		return func() {}
	}
	if err := exec.Command(lock).Start(); err != nil {
		return func() {}
	}
	return func() { _ = exec.Command(unlock).Run() }
}

// termuxNetwork returns a coarse network type without exposing SSIDs or IPs.
func termuxNetwork() string {
	if runtime.GOOS != "android" {
		return ""
	}
	if bin, err := exec.LookPath("termux-wifi-connectioninfo"); err == nil {
		if out, err := exec.Command(bin).Output(); err == nil {
			var info struct {
				SSID string `json:"ssid"`
				IP   string `json:"ip"`
			}
			if json.Unmarshal(out, &info) == nil && (info.SSID != "" || info.IP != "") {
				return "wifi"
			}
		}
	}
	if bin, err := exec.LookPath("termux-telephony-network-info"); err == nil {
		if out, err := exec.Command(bin).Output(); err == nil {
			var info struct {
				Type string `json:"network_type"`
			}
			if json.Unmarshal(out, &info) == nil && info.Type != "" {
				return "celular (" + info.Type + ")"
			}
		}
	}
	return ""
}

// termuxBattery returns battery percentage and temperature in Celsius when
// termux-api is available.
func termuxBattery() (percent, temperature float64, ok bool) {
	if runtime.GOOS != "android" {
		return 0, 0, false
	}
	bin, err := exec.LookPath("termux-battery-status")
	if err != nil {
		return 0, 0, false
	}
	out, err := exec.Command(bin).Output()
	if err != nil {
		return 0, 0, false
	}
	var status struct {
		Percentage  float64 `json:"percentage"`
		Temperature float64 `json:"temperature"`
	}
	if err := json.Unmarshal(out, &status); err != nil {
		return 0, 0, false
	}
	// termux-api reports temperature in tenths of a degree Celsius.
	return status.Percentage, status.Temperature / 10, true
}
