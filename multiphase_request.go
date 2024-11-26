package main

import (
	"github.com/neel-patel-1/xmp_sched_sim/blocks"
	"github.com/neel-patel-1/xmp_sched_sim/engine"
)

type DeviceType int

const (
	Processor DeviceType = iota
	Accelerator
)

type Phase struct {
	blocks.Request
	Devices map[DeviceType]struct{}
}

type MultiPhaseReq struct {
	blocks.Request
	Phases        []Phase
	Current       int
	lastGPCoreIdx int
}

type MultiPhaseReqCreator struct{}

// NewRequest returns a new MultiPhaseReq
func (m MultiPhaseReqCreator) NewRequest(serviceTime float64) engine.ReqInterface {
	return &MultiPhaseReq{
		Phases: []Phase{
			{
				Request: blocks.Request{InitTime: engine.GetTime(), ServiceTime: serviceTime},
				Devices: map[DeviceType]struct{}{Processor: {}},
			},
			{
				Request: blocks.Request{InitTime: engine.GetTime(), ServiceTime: serviceTime},
				Devices: map[DeviceType]struct{}{
					Processor:   {},
					Accelerator: {},
				},
			},
		},
		Current: 0,
	}
}

type ThreePhaseReqCreator struct {
	phase_one_ratio   float64
	phase_two_ratio   float64
	phase_three_ratio float64
}

func (m ThreePhaseReqCreator) NewRequest(serviceTime float64) engine.ReqInterface {
	return &MultiPhaseReq{
		Phases: []Phase{
			{
				Request: blocks.Request{InitTime: engine.GetTime(), ServiceTime: serviceTime * m.phase_one_ratio},
				Devices: map[DeviceType]struct{}{Processor: {}},
			},
			{
				Request: blocks.Request{InitTime: -1, ServiceTime: serviceTime * m.phase_two_ratio},
				Devices: map[DeviceType]struct{}{Processor: {}, Accelerator: {}},
			},
			{
				Request: blocks.Request{InitTime: -1, ServiceTime: serviceTime * m.phase_three_ratio}, // placeholder, users may want to track each phase's init time
				Devices: map[DeviceType]struct{}{Processor: {}},
			},
		},
		Current: 0,
	}
}

func (m *MultiPhaseReq) GetDelay() float64 {
	return engine.GetTime() - m.Phases[0].InitTime
}

func (m *MultiPhaseReq) GetServiceTime() float64 {
	return m.Phases[m.Current].GetServiceTime()
}
