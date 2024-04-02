package db

import "time"

type HealthCheck struct {
	CTime time.Time `bson:"ctime"`
	RAM   string    `bson:"ram"`
	CPU   string    `bson:"cpu"`
	Disk  string    `bson:"disk"`
}