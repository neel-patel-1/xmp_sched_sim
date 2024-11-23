package main

type Device int

const (
	CPU Device = iota
	Accel
)

type SubTask struct {
	PrefDev     Device
	ServiceTime float64
}

type XMPRequest struct {
	InitTime    float64
	Subtasks    []SubTask
	CurrentTask int
}
