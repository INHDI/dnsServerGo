package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/miekg/dns"
)

func killProcessPort(port string) {
	cmd := exec.Command("lsof", "-i", ":"+port)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error running lsof command:", err)
		return
	}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "(LISTEN)") {
			fields := strings.Fields(line)
			pid := fields[1]
			// Kết thúc quy trình đang sử dụng port 53
			killCmd := exec.Command("kill", "-9", pid)
			if err := killCmd.Run(); err != nil {
				fmt.Println("Error killing process:", err)
				return
			}

			fmt.Println("Process", pid, "terminated successfully.")
			break
		}
	}
}

func handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	var blacklistedDomains = []string{"fb.com", "google.com"}
	// Lấy thời gian hiện tại
	currentTime := time.Now().Format("Jan 02 15:04:05")

	// Lấy địa chỉ IP từ địa chỉ IP và cổng
	remoteAddr := strings.Split(w.RemoteAddr().String(), ":")[0]

	fmt.Printf("(%s) Received DNS request from: %s\n", currentTime, remoteAddr)

	// Kiểm tra xem domain yêu cầu có trong danh sách đen không
	for _, q := range r.Question {
		for _, blacklisted := range blacklistedDomains {
			fmt.Printf("%s -- %s ", q.Name, blacklisted)
			fmt.Printf("%t\n", strings.HasSuffix(q.Name, blacklisted))
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

	// Tạo một yêu cầu DNS đến máy chủ DNS của Google
	c := new(dns.Client)
	googleDNS := "8.8.8.8:53" // Địa chỉ IP và cổng của máy chủ DNS Google
	resp, _, err := c.Exchange(r, googleDNS)
	if err != nil {
		fmt.Println("Error sending DNS request to Google DNS:", err)
		return
	}

	// Ghi thông tin vào file log
	logFile, err := os.OpenFile("dns.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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

	// Gửi phản hồi từ máy chủ DNS Google cho máy khách ban đầu
	w.WriteMsg(resp)
}

func main() {
	var port string = "53"
	killProcessPort(port) // Kết thúc quy trình sử dụng port 53 trước khi khởi động máy chủ DNS

	server := &dns.Server{Addr: ":53", Net: "udp"}
	dns.HandleFunc(".", handleDNSRequest)

	fmt.Println("DNS resolver listening on port 53...")

	err := server.ListenAndServe()
	if err != nil {
		fmt.Println("Error starting DNS server:", err)
		return
	}
}
