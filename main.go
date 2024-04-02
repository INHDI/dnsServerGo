package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"dnsServerGo/internal/db"

	"github.com/miekg/dns"
)

var (
	runMode  bool
	stopMode bool
	helpMode bool
)

const (
	maxRetries    int    = 5
	retryInterval        = 2 * time.Second
	port          string = "53"
	primaryDNS    string = "8.8.8.8:53"
	backupDNS     string = "8.8.4.4:53"
)

var blacklistedDomains = []string{"xxx.com", "zzz.con"}

const MONGO_DB string = "DnsNodeI"

func init() {
	flag.BoolVar(&runMode, "run", false, "Run the DNS resolver service")
	flag.BoolVar(&stopMode, "stop", false, "Stop the DNS resolver service and restore system DNS")
	flag.BoolVar(&helpMode, "help", false, "Display usage help")

	// flag.StringVar(&dnsServer, "dns", "8.8.8.8:53", "Primary DNS server address")
	// flag.StringVar(&backupDNS, "backup-dns", "8.8.4.4:53", "Backup DNS server address")
	flag.Parse()
}

func stopSystemdResolved() error {
	// Tạo bản sao của tệp cấu hình DNS hiện tại
	err := exec.Command("sudo", "cp", "/etc/resolv.conf", "/etc/resolv.conf.backup").Run()
	if err != nil {
		return fmt.Errorf("error creating backup of system DNS configuration: %v", err)
	}

	// Tắt dịch vụ systemd-resolved
	stopCmd := exec.Command("sudo", "systemctl", "stop", "systemd-resolved")
	if err := stopCmd.Run(); err != nil {
		return fmt.Errorf("error stopping systemd-resolved: %v", err)
	}

	// Vô hiệu hóa dịch vụ systemd-resolved
	disableCmd := exec.Command("sudo", "systemctl", "disable", "systemd-resolved")
	if err := disableCmd.Run(); err != nil {
		return fmt.Errorf("error disabling systemd-resolved: %v", err)
	}

	// Xóa symlink /etc/resolv.conf
	rmResolvCmd := exec.Command("sudo", "rm", "/etc/resolv.conf")
	if err := rmResolvCmd.Run(); err != nil {
		return fmt.Errorf("error removing /etc/resolv.conf: %v", err)
	}

	return nil
}

func restoreSystemDNS() error {
	// Khôi phục lại tệp cấu hình DNS của hệ thống từ một bản sao đã lưu trữ trước đó.
	// Đối với Ubuntu, thường có thể sao lưu tệp cấu hình DNS tại '/etc/resolv.conf.backup'.

	// Kiểm tra xem tệp cấu hình DNS đã được sao lưu hay chưa
	if _, err := os.Stat("/etc/resolv.conf.backup"); os.IsNotExist(err) {
		return errors.New("system DNS configuration backup file not found")
	}

	// Thực hiện thay thế tệp cấu hình DNS hiện tại bằng tệp cấu hình đã sao lưu
	err := exec.Command("sudo", "cp", "/etc/resolv.conf.backup", "/etc/resolv.conf").Run()
	if err != nil {
		return fmt.Errorf("error restoring system DNS: %v", err)
	}

	fmt.Println("System DNS configuration restored successfully.")

	// Enable và start lại dịch vụ systemd-resolved
	startCmd := exec.Command("sudo", "systemctl", "start", "systemd-resolved")
	if err := startCmd.Run(); err != nil {
		return fmt.Errorf("error starting systemd-resolved: %v", err)
	}

	enableCmd := exec.Command("sudo", "systemctl", "enable", "systemd-resolved")
	if err := enableCmd.Run(); err != nil {
		return fmt.Errorf("error enabling systemd-resolved: %v", err)
	}

	return nil
}

func killProcessPort(port string) error {
	// Tìm tiến trình đang sử dụng port
	cmd := exec.Command("lsof", "-i", ":"+port)
	output, _ := cmd.CombinedOutput()

	// Phân tích kết quả lsof
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "(LISTEN)") {
			fields := strings.Fields(line)
			pid := fields[1]

			// Kết thúc tiến trình sử dụng port
			killCmd := exec.Command("sudo", "kill", "-9", pid)
			if err := killCmd.Run(); err != nil {
				return fmt.Errorf("error killing process: %v", err)
			}

			fmt.Println("Process", pid, "terminated successfully.")
			break
		}
	}

	return nil
}

func handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {

	currentTime := time.Now().Format("Jan 02 15:04:05")

	remoteAddr, _, _ := net.SplitHostPort(w.RemoteAddr().String())

	fmt.Printf("(%s) Received DNS request from: %s\n", currentTime, remoteAddr)

	// Kiểm tra xem domain yêu cầu có trong danh sách đen không
	for _, q := range r.Question {
		for _, blacklisted := range blacklistedDomains {
			dnsNameClient := strings.TrimSuffix(q.Name, ".")
			if strings.HasSuffix(dnsNameClient, blacklisted) {
				fmt.Printf("Domain %s is blacklisted\n", q.Name)
				// Trả về một phản hồi tùy chỉnh cho domain bị cấm
				// Ví dụ: trả về một phản hồi rỗng hoặc phản hồi lỗi
				errorResponse := new(dns.Msg)
				errorResponse.SetRcode(r, dns.RcodeRefused)
				w.WriteMsg(errorResponse)
				return
			}
		}
	}

	// Tạo một yêu cầu DNS đến máy chủ DNS chính
	var resp *dns.Msg
	var err error
	for i := 0; i < maxRetries; i++ {
		c := new(dns.Client)
		resp, _, err = c.Exchange(r, primaryDNS)
		if err == nil {
			break
		}
		fmt.Printf("Error sending DNS request to primary DNS server (attempt %d/%d): %v\n", i+1, maxRetries, err)
		time.Sleep(retryInterval)
	}

	if err != nil {
		fmt.Println("All attempts to send DNS request to primary DNS server failed.")
		fmt.Println("Trying with backup DNS server...")
		c := new(dns.Client)
		resp, _, err = c.Exchange(r, backupDNS)
		if err != nil {
			fmt.Println("Error sending DNS request to backup DNS server:", err)
			return
		}
	}

	// Ghi thông tin vào file log
	logFile, err := os.OpenFile("log/dns.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening log file:", err)
		return
	}
	defer logFile.Close()

	logFile.WriteString(fmt.Sprintf("(%s) Received DNS request from: %s\n", currentTime, remoteAddr))

	for _, q := range r.Question {
		logFile.WriteString(fmt.Sprintf("Name: %s, Type: %s\n", q.Name, dns.TypeToString[q.Qtype]))
	}

	for _, a := range resp.Answer {
		logFile.WriteString(fmt.Sprintf("%s\n", a.String()))
	}

	// Gửi phản hồi từ máy chủ DNS cho máy khách ban đầu
	w.WriteMsg(resp)
}



func main() {
	client, err := db.ConnectDB()
	if err != nil {
        fmt.Println("Failed to connect to MongoDB:", err)
        return
    }
	defer db.CloseDB(client)

	db.CreateDatabase(client, MONGO_DB, "test")

	db.StartHealthCheckWriter(client, MONGO_DB)
	

	// Kiểm tra chế độ hoạt động được chọn
	if helpMode {
		flag.Usage()
		return
	}

	if stopMode {
		if err := restoreSystemDNS(); err != nil {
			fmt.Println("Error restoring system DNS:", err)
			return
		}
		fmt.Println("System DNS restored successfully.")
		return
	}

	if runMode {
		if err := stopSystemdResolved(); err != nil {
			fmt.Println("Error stopping systemd-resolved:", err)
			return
		}
		if err := killProcessPort(port); err != nil {
			fmt.Println("Error killing process on port "+port+":", err)
			return
		}
		time.Sleep(5 * time.Second)
		fmt.Println("Port " + port + " released successfully.")

		server := &dns.Server{Addr: ":" + port + "", Net: "udp"}
		dns.HandleFunc(".", handleDNSRequest)

		fmt.Println("DNS resolver listening on port " + port + "...")

		err := server.ListenAndServe()
		if err != nil {
			log.Fatalf("Error starting DNS server: %s", err)
		}
		return
	}

	// If no mode specified or invalid combination of modes, print usage
	flag.Usage()
}
