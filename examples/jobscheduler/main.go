package main

import (
	"fmt"
	"time"

	"github.com/fsandov/go-sdk/pkg/jobscheduler"
)

func main() {
	scheduler := jobscheduler.NewMemoryScheduler()
	scheduler.Add("@every 2s", func() {
		fmt.Println("Job executed at:", time.Now())
	})
	scheduler.Start()
	time.Sleep(7 * time.Second)
	scheduler.Stop()
	fmt.Println("Scheduler stopped")
}
