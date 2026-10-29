package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("veriforge mock init started")

	// PID 1 must not exit; returning from main triggers a kernel panic.
	for {
		time.Sleep(2 * time.Second)
		fmt.Println("mock init heartbeat")
	}
}
