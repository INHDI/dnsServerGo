package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
	"go.mongodb.org/mongo-driver/mongo"
)

// StartHealthCheckWriter khởi động một goroutine để ghi thông tin sức khỏe vào cơ sở dữ liệu mỗi 3 phút
func StartHealthCheckWriter(client *mongo.Client, dbName string) {
	go func() {
		for {
			err := WriteHealthCheck(client, dbName)
			if err != nil {
				log.Println("Error writing health check:", err)
			}
			time.Sleep(1 * time.Minute) // Chờ 3 phút trước khi ghi sức khỏe tiếp theo
		}
	}()
}

// Create a health check document
func WriteHealthCheck(client *mongo.Client, dbName string) error {
	collection := "health_check"
	// Connect to the collection
	coll, err := ConnectCollection(client, dbName, collection)
	if err != nil {
		return err
	}
	newHealthCheck, err := GetHardwareHealthCheck()
	if err != nil {
		log.Fatal("Failed to get hardware health check:", err)
	}

	// Insert the document into the collection
	_, err = coll.InsertOne(context.Background(), newHealthCheck)
	if err != nil {
		log.Fatal("Failed to insert document into collection:", err)
		return err
	}

	// Return nil error
	return nil
}

// GetHardwareHealthCheck lấy thông tin phần cứng thực tế của máy chủ và tạo một đối tượng HealthCheck mới
func GetHardwareHealthCheck() (HealthCheck, error) {
	var healthCheck HealthCheck

	// Lấy thông tin RAM
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return healthCheck, err
	}
	ramPercentage := fmt.Sprintf("%.2f", memInfo.UsedPercent)
	healthCheck.RAM = ramPercentage + "%"

	// Lấy thông tin CPU
	cpuInfo, err := cpu.Percent(time.Second, false)
	if err != nil {
		return healthCheck, err
	}
	cpuPercentage := fmt.Sprintf("%.2f", cpuInfo[0])
	healthCheck.CPU = cpuPercentage + "%"

	// Lấy thông tin Disk
	diskInfo, err := disk.Usage("/")
	if err != nil {
		return healthCheck, err
	}
	diskPercentage := fmt.Sprintf("%.2f", diskInfo.UsedPercent)
	healthCheck.Disk = diskPercentage + "%"

	// Lấy thời gian hiện tại
	healthCheck.CTime = time.Now()

	return healthCheck, nil
}
