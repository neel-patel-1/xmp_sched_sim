package main

import (
	"fmt"
)

func gen_subtask(pref_dev Device, service_time float64) SubTask {
	return SubTask{PrefDev: pref_dev, ServiceTime: service_time}
}

func main() {
	// Create Subtasks
	subtask1 := gen_subtask(CPU, 1.0)
	subtask2 := gen_subtask(Accel, 2.0)
	subtask3 := gen_subtask(CPU, 3.0)

	// Create an XMPRequest with multiple Subtasks
	request := XMPRequest{
		InitTime: 0.0,
		Subtasks: []SubTask{subtask1, subtask2, subtask3},
	}

	// Print the request to verify
	fmt.Printf("XMPRequest: %+v\n", request)
}
