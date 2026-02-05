package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procGetTickCount64       = kernel32.NewProc("GetTickCount64")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
)

type MEMORYSTATUSEX struct {
	dwLength                uint32
	dwMemoryLoad            uint32
	ullTotalPhys            uint64
	ullAvailPhys            uint64
	ullTotalPage            uint64
	ullAvailPage            uint64
	ullTotalVirtual         uint64
	ullAvailVirtual         uint64
	ullAvailExtendedVirtual uint64
}

const (
	Reset      = "\033[0m"
	Red        = "\033[31m"
	Green      = "\033[32m"
	Yellow     = "\033[33m"
	Blue       = "\033[34m"
	Purple     = "\033[35m"
	Cyan       = "\033[36m"
	NeonGreen  = "\033[92m"
	CyanBright = "\033[96m"
	LightBlue  = "\033[94m"
	BrightRed  = "\033[91m"
)

const bannerArt = `
⠀⠀⠀⠀⠀⠀⠀⠀⢀⠔⠊⠉⠐⢆⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⢀⠏⠀⠀⠀⠀⠘⡆⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⣸⠀⠀⠀⠀⠀⠀⢡⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⡏⠀⠀⠀⠀⠀⠀⠘⡆⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⢰⠁⢀⠔⠀⠒⢤⡔⠈⠉⠢⡀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⣾⠀⡇⠀⠀⠂⢀⠂⠀⠂⠀⡅⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⡇⠀⠑⠤⠀⠠⠊⠐⠤⠤⢞⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⢰⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀⠘⡄⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⣼⠀⠀⠀⣀⣴⣶⣿⣿⣷⣦⡀⢱⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⡇⠀⠀⣴⣿⣿⣿⣿⣿⣿⣿⣷⡌⡇⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⢰⠁⠀⣾⣿⣿⣿⣿⣿⣿⣿⣿⣿⣷⡸⡀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⡾⠀⢸⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡇⢣⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⢠⠇⠀⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠈⢆⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⢀⡎⠀⠀⠛⠻⠿⠿⠿⠿⠿⣿⣿⠛⠛⠛⠉⠀⢰⢆⠀⠀⠀⠀⠀⠀⠀⠀
⠀⢠⠏⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠐⠀⠈⠻⣆⠀⠀⠀⠀⠀⠀⠀
⢠⠇⠀⠀⠀⣠⠆⠀⠀⠀⠀⢁⣀⡠⢤⣼⣛⣲⣯⣭⣭⣭⠿⣆⠀⠀⠀⠀⠀⠀
⡎⠀⠀⠀⡰⠃⠀⣀⡤⣖⠪⢿⣶⣿⣿⣿⣿⣿⣿⣿⠿⠟⠀⠘⡄⠀⠀⠀⠀⠀
⣇⠀⠀⡼⣁⢴⡪⠗⠉⠀⠀⠀⠻⠟⢋⣿⣿⣿⣿⣿⡆⠀⠀⠀⢳⠀⠀⠀⠀⠀
⠘⢤⣼⣋⠗⠁⠀⠀⠀⠀⠀⠀⠀⣤⣘⣿⣿⡿⣿⣟⣥⠖⠀⠀⢨⣿⣦⣀⠀⠀
⠀⢸⡗⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⠙⠛⢿⡿⠟⠋⠁⠀⠀⠀⡼⠁⠀⠈⠑⢆
⠀⠀⠳⣀⠀⠀⠀⠀⠀⠀⣠⣦⡀⢠⣄⣠⠤⠷⣀⡠⠶⢄⣀⣼⣀⠀⠀⣀⣀⠜
⠀⠀⠀⠈⠉⠒⠤⠄⣀⣰⣿⣿⣷⣿⡟⠁⠀⠀⠈⠱⡄⠀⠀⠀⠉⠉⠉⠁⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⠉⠙⠛⠿⠤⢀⣀⣀⣀⡴⠃⠀⠀⠀
`

type SystemInfo struct {
	CPU             string
	GPU             string
	Memory          string
	CPUUsage        float64
	CPUUsagePrev    float64
	GPUUsage        float64
	Network         string
	LocalIP         string
	PublicIP        string
	WiFiName        string
	WiFiAdapterName string
	Monitor         string
	Battery         string
	Disks           string
	LastIPUpdate    time.Time
	LastNetUpdate   time.Time
	mu              sync.RWMutex
}

var sysInfo = &SystemInfo{}

func main() {
	enableVirtualTerminalProcessing()

	osName := getOSName()

	sysInfo.mu.Lock()
	sysInfo.CPU = getCPUName()
	sysInfo.GPU = getGPUName()
	sysInfo.Monitor = getMonitorInfo()
	sysInfo.Battery = getBatteryInfo()
	sysInfo.Disks = getDisksInfo()
	sysInfo.LastIPUpdate = time.Now()
	sysInfo.LastNetUpdate = time.Now()
	sysInfo.mu.Unlock()

	go updateSystemInfoRealtime()
	go updateNetworkInfoAsync()
	go updatePublicIPAsync()

	user := os.Getenv("USERNAME")
	host, _ := os.Hostname()

	fmt.Print("\033[2J\033[H")

	bannerLines := strings.Split(strings.TrimPrefix(bannerArt, "\n"), "\n")

	ticker := time.NewTicker(33 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		fmt.Print("\033[H")

		sysInfo.mu.RLock()

		maxLines := len(bannerLines)
		for i := 0; i < maxLines+3; i++ {
			bannerLine := ""
			if i < len(bannerLines) {
				bannerLine = fmt.Sprintf("%s%-60s%s", LightBlue, bannerLines[i], Reset)
			} else {
				bannerLine = strings.Repeat(" ", 60)
			}

			var infoLine string
			switch {
			case i < 3:
				infoLine = ""
			case i == 3:
				infoLine = fmt.Sprintf("%s%s%s@%s%s%s", NeonGreen, user, Reset, NeonGreen, host, Reset)
			case i == 4:
				infoLine = strings.Repeat("─", len(user)+len(host)+1)
			default:
				infoIndex := i - 5
				infoLines := []struct {
					Label string
					Value string
				}{
					{"OS", osName},
					{"Architecture", runtime.GOARCH},
					{"Uptime", getUptime()},
					{"CPU", sysInfo.CPU},
					{"CPU Usage", fmt.Sprintf("%.1f%%", sysInfo.CPUUsage)},
					{"GPU", sysInfo.GPU},
					{"GPU Usage", fmt.Sprintf("%.1f%%", sysInfo.GPUUsage)},
					{"Monitor", sysInfo.Monitor},
					{"Battery", sysInfo.Battery},
					{"Disks", sysInfo.Disks},
					{"Memory", sysInfo.Memory},
					{"Network", sysInfo.Network},
					{"WiFi Name", sysInfo.WiFiName},
					{"WiFi Adapter", sysInfo.WiFiAdapterName},
					{"Local IP", sysInfo.LocalIP},
					{"Public IP", sysInfo.PublicIP},
				}

				if infoIndex < len(infoLines) {
					infoLine = fmt.Sprintf("%s%s:%s %s", Yellow, infoLines[infoIndex].Label, Reset, infoLines[infoIndex].Value)
				}
			}
			fmt.Printf("%s%s %s\n", bannerLine, infoLine, Reset)
		}
		sysInfo.mu.RUnlock()
	}
}

func initSystemInfo() {
	sysInfo.mu.Lock()
	defer sysInfo.mu.Unlock()

	sysInfo.CPU = getCPUName()
	sysInfo.GPU = getGPUName()
	sysInfo.Memory = getMemory()
	sysInfo.Network = getNetworkName()
	sysInfo.LocalIP = getLocalIP()
	sysInfo.WiFiName = getWiFiSSID()
	sysInfo.WiFiAdapterName = getWiFiAdapterName()
	sysInfo.Monitor = getMonitorInfo()
	sysInfo.Battery = getBatteryInfo()
	sysInfo.Disks = getDisksInfo()
	sysInfo.CPUUsage = 0
	sysInfo.GPUUsage = 0
}

func updateSystemInfoRealtime() {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		sysInfo.mu.Lock()
		sysInfo.Memory = getMemory()
		sysInfo.CPUUsage = getCPUUsage()
		sysInfo.GPUUsage = getGPUUsage()
		sysInfo.mu.Unlock()
	}
}

func updateNetworkInfoAsync() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		sysInfo.mu.Lock()
		sysInfo.Network = getNetworkName()
		sysInfo.WiFiName = getWiFiSSID()
		sysInfo.WiFiAdapterName = getWiFiAdapterName()
		sysInfo.LocalIP = getLocalIP()
		sysInfo.Monitor = getMonitorInfo()
		sysInfo.Battery = getBatteryInfo()
		sysInfo.Disks = getDisksInfo()
		sysInfo.LastNetUpdate = time.Now()
		sysInfo.mu.Unlock()
	}
}

func updatePublicIPAsync() {
	for {
		time.Sleep(30 * time.Second)
		publicIP := getPublicIP()
		sysInfo.mu.Lock()
		sysInfo.PublicIP = publicIP
		sysInfo.LastIPUpdate = time.Now()
		sysInfo.mu.Unlock()
	}
}

func getOSName() string {
	out, err := exec.Command("cmd", "/c", "ver").Output()
	if err != nil {
		return "Windows"
	}
	return strings.TrimSpace(string(out))
}

func getUptime() string {
	r1, _, _ := procGetTickCount64.Call()
	millis := int64(r1)
	days := millis / (1000 * 60 * 60 * 24)
	hours := (millis / (1000 * 60 * 60)) % 24
	mins := (millis / (1000 * 60)) % 60
	if days > 0 {
		return fmt.Sprintf("%d days, %d hours, %d mins", days, hours, mins)
	}
	return fmt.Sprintf("%d hours, %d mins", hours, mins)
}

func getMemory() string {
	var memStatus MEMORYSTATUSEX
	memStatus.dwLength = uint32(unsafe.Sizeof(memStatus))
	ret, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&memStatus)))
	if ret == 0 {
		return "Unknown"
	}
	totalGB := float64(memStatus.ullTotalPhys) / 1024 / 1024 / 1024
	usedGB := float64(memStatus.ullTotalPhys-memStatus.ullAvailPhys) / 1024 / 1024 / 1024
	percent := memStatus.dwMemoryLoad
	return fmt.Sprintf("%.1fGB / %.1fGB (%d%%)", usedGB, totalGB, percent)
}

func getCPUName() string {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "Get-WmiObject -Class Win32_Processor | Select-Object -ExpandProperty Name")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err == nil {
		result := strings.TrimSpace(out.String())
		if result != "" {
			return result
		}
	}
	return "Unknown CPU"
}

func getCPUUsage() float64 {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "Get-WmiObject -Class Win32_Processor | Select-Object -ExpandProperty LoadPercentage")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	var usage float64
	if err := cmd.Run(); err == nil {
		fmt.Sscanf(strings.TrimSpace(out.String()), "%f", &usage)
	}
	return usage
}

func getGPUName() string {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "Get-WmiObject -Class Win32_VideoController | Select-Object -ExpandProperty Name | Select-Object -First 1")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err == nil {
		result := strings.TrimSpace(out.String())
		if result != "" {
			return result
		}
	}
	return "Default GPU"
}

func getGPUUsage() float64 {

	return 0.0
}

func getNetworkName() string {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "Get-NetConnectionProfile | Select-Object -ExpandProperty Name 2>$null | Select-Object -First 1")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err == nil {
		result := strings.TrimSpace(out.String())
		if result != "" {
			return result
		}
	}
	return "Unknown"
}

func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "Unknown"
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

func getMonitorInfo() string {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "Get-WmiObject -Class Win32_VideoController | Select-Object -First 1 -Property CurrentHorizontalResolution,CurrentVerticalResolution,CurrentRefreshRate | ForEach-Object { \"$($_.CurrentHorizontalResolution)x$($_.CurrentVerticalResolution) @ $($_.CurrentRefreshRate)Hz\" }")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err == nil {
		res := strings.TrimSpace(out.String())
		if res != "" {
			return res
		}
	}
	return "Unknown"
}

func getBatteryInfo() string {
	ps := "$b=Get-WmiObject -Class Win32_Battery -ErrorAction SilentlyContinue | Select-Object -First 1; if ($b -eq $null) { 'No Battery' } else { $s = switch ($b.BatteryStatus) { 1 { 'Discharging' } 2 { 'AC - Charging' } 3 { 'Fully Charged' } default { 'Unknown' } }; \"$($b.EstimatedChargeRemaining)% ($s)\" }"
	cmd := exec.Command("powershell", "-NoProfile", "-Command", ps)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err == nil {
		res := strings.TrimSpace(out.String())
		if res != "" {
			return res
		}
	}
	return "No Battery"
}

func getDisksInfo() string {
	ps := "$res = Get-WmiObject Win32_LogicalDisk -Filter \"DriveType=3\" | ForEach-Object { $used = [math]::Round((($_.Size - $_.FreeSpace)/1GB),1); $total = [math]::Round(($_.Size/1GB),1); \"$($_.DeviceID) $used/$total GB\" }; if ($res) { $res -join '; ' } else { 'No Disks' }"
	cmd := exec.Command("powershell", "-NoProfile", "-Command", ps)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err == nil {
		res := strings.TrimSpace(out.String())
		if res != "" {
			return res
		}
	}
	return "No Disks"
}

func getPublicIP() string {
	services := []string{
		"https://api.ipify.org",
		"https://ifconfig.me/ip",
		"https://ident.me",
		"https://icanhazip.com",
	}

	client := &http.Client{Timeout: 5 * time.Second}
	for _, url := range services {
		resp, err := client.Get(url)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err == nil && len(body) > 0 {
			return strings.TrimSpace(string(body))
		}
	}
	return "Unavailable"
}

func getWiFiSSID() string {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "netsh wlan show interfaces 2>$null | Select-String 'SSID' | Select-Object -First 1 | ForEach-Object { $_.ToString().Split(':')[1].Trim() }")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err == nil {
		result := strings.TrimSpace(out.String())
		if result != "" {
			return result
		}
	}
	return "Not Connected"
}

func getWiFiAdapterName() string {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "Get-NetAdapter -Physical 2>$null | Where-Object { $_.InterfaceDescription -match 'Wireless|WiFi|802.11' } | Select-Object -ExpandProperty Name | Select-Object -First 1")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err == nil {
		result := strings.TrimSpace(out.String())
		if result != "" {
			return result
		}
	}
	return "Unknown"
}

func enableVirtualTerminalProcessing() {
	handle, _, _ := kernel32.NewProc("GetStdHandle").Call(uintptr(windows.STD_OUTPUT_HANDLE))
	var mode uint32
	getMode, _, _ := kernel32.NewProc("GetConsoleMode").Call(handle, uintptr(unsafe.Pointer(&mode)))
	if getMode != 0 {
		kernel32.NewProc("SetConsoleMode").Call(handle, uintptr(mode|0x0004))
	}
}
