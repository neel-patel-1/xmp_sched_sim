package main

import (
	"fmt"
	"log"

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
	Devices []DeviceType
}

type MultiPhaseReq struct {
	blocks.Request
	Phases  []Phase
	Current int
}

type MultiPhaseReqCreator struct{}

// NewRequest returns a new MultiPhaseReq
func (m MultiPhaseReqCreator) NewRequest(serviceTime float64) engine.ReqInterface {
	return &MultiPhaseReq{
		Phases: []Phase{
			{
				Request: blocks.Request{InitTime: engine.GetTime(), ServiceTime: serviceTime},
				Devices: []DeviceType{Processor},
			},
			{
				Request: blocks.Request{InitTime: engine.GetTime(), ServiceTime: serviceTime},
				Devices: []DeviceType{Processor, Accelerator},
			},
		},
		Current: 0,
	}
}

type ThreePhaseReqCreator struct{}

func (m ThreePhaseReqCreator) NewRequest(serviceTime float64) engine.ReqInterface {
	return &MultiPhaseReq{
		Phases: []Phase{
			{
				Request: blocks.Request{InitTime: engine.GetTime(), ServiceTime: serviceTime},
				Devices: []DeviceType{Processor},
			},
			{
				Request: blocks.Request{InitTime: engine.GetTime(), ServiceTime: serviceTime},
				Devices: []DeviceType{Processor, Accelerator},
			},
			{
				Request: blocks.Request{InitTime: engine.GetTime(), ServiceTime: serviceTime},
				Devices: []DeviceType{Processor},
			},
		},
		Current: 0,
	}
}

func (m *MultiPhaseReq) GetDelay() float64 {
	return m.Phases[m.Current].GetDelay()
}

func (m *MultiPhaseReq) GetServiceTime() float64 {
	return m.Phases[m.Current].GetServiceTime()
}

type ForwardDecisionProcedure func(outQueues []engine.QueueInterface, req *MultiPhaseReq) int

type mpProcessor struct {
	engine.Actor
	reqDrain           blocks.RequestDrain
	ctxCost            float64
	offloadCost        float64
	deviceType         DeviceType
	speedup            float64
	OutBoundProcessors []*mpProcessor
	forwardFunc        ForwardDecisionProcedure
}

func (p *mpProcessor) SetReqDrain(rd blocks.RequestDrain) {
	p.reqDrain = rd
}

func (p *mpProcessor) SetCtxCost(cost float64) {
	p.ctxCost = cost
}

func (p *mpProcessor) SetOffloadCost(cost float64) {
	p.offloadCost = cost
}

func (p *mpProcessor) GetDeviceType() DeviceType {
	return Processor
}

func (p *mpProcessor) SetDeviceType(deviceType DeviceType) {
	p.deviceType = deviceType
}

func (p *mpProcessor) SetSpeedup(speedup float64) {
	p.speedup = speedup
}

// RTCMPProcessor is a run to completion multi-phase processor
type RTCMPProcessor struct {
	mpProcessor
}

// Run is the main processor loop
func (p *RTCMPProcessor) Run() {
	for {
		req := p.ReadInQueue()
		actualServiceTime := req.GetServiceTime() / p.speedup
		p.Wait(actualServiceTime + p.ctxCost)
		if multiPhaseReq, ok := req.(*MultiPhaseReq); ok {
			if multiPhaseReq.Current < len(multiPhaseReq.Phases)-1 {
				// Move to the next phase
				multiPhaseReq.Current++
				// Forward to the outgoing queue
				outQueueIdx := p.forwardFunc(p.GetOutQueues(), multiPhaseReq)
				p.WriteOutQueueI(req, outQueueIdx)
			} else {
				// Last phase, terminate the request
				p.reqDrain.TerminateReq(req)
			}
		} else {
			// Handle non-multi-phase requests
			log.Fatalf("Error: RTCMPProcessor received a non-multi-phase request")
		}

	}
}

func single_core_deterministic(interarrival_time, service_time, duration float64) {
	engine.InitSim()

	stats := &blocks.AllKeeper{}
	stats.SetName("Main Stats")
	engine.InitStats(stats)

	// Add generator
	g := blocks.NewDDGenerator(interarrival_time, service_time)
	g.SetCreator(&blocks.SimpleReqCreator{})

	// Create queues
	q := blocks.NewQueue()

	// Create processors
	p := &blocks.RTCProcessor{}
	p.AddInQueue(q)
	p.SetReqDrain(stats)
	engine.RegisterActor(p)

	g.AddOutQueue(q)

	// Register the generator
	engine.RegisterActor(g)

	fmt.Printf("Cores:%v\tservice_time:%v\tinterarrival_rate:%v\n", 1, service_time, interarrival_time)
	engine.Run(duration)
}

func chained_cores_multi_phase_deterministic(interarrival_time, service_time, duration float64, speedup float64) {
	engine.InitSim()

	stats := &blocks.AllKeeper{}
	stats.SetName("Main Stats")
	engine.InitStats(stats)

	// Add generator
	g := blocks.NewDDGenerator(interarrival_time, service_time)
	g.SetCreator(&MultiPhaseReqCreator{})

	q := blocks.NewQueue()
	q2 := blocks.NewQueue()

	// Create processors
	p := &RTCMPProcessor{}
	p.SetSpeedup(1)
	p.SetOffloadCost(0)
	p.SetDeviceType(Processor)
	p.SetCtxCost(0)
	p.AddInQueue(q)
	p.AddOutQueue(q2)

	p2 := &RTCMPProcessor{}
	p2.AddInQueue(q2)
	p2.SetDeviceType(Accelerator)
	p2.SetCtxCost(0)
	p2.SetSpeedup(speedup)
	p2.SetOffloadCost(0)
	p2.SetReqDrain(stats)

	p.forwardFunc = func(outQueues []engine.QueueInterface, req *MultiPhaseReq) int {
		return 0
	}
	engine.RegisterActor(p)

	engine.RegisterActor(p2)

	g.AddOutQueue(q)

	engine.RegisterActor(g)
	engine.Run(duration)
}

func naive_chained_cores_single_queue_three_phase(interarrival_time, service_time, duration float64, speedup float64,
	num_cores int, num_accelerators int) {
	engine.InitSim()

	stats := &blocks.AllKeeper{}
	stats.SetName("Main Stats")
	engine.InitStats(stats)

	// Add generator && set up dispatcher
	g := blocks.NewDDGenerator(interarrival_time, service_time)
	g.SetCreator(&ThreePhaseReqCreator{})
	q := blocks.NewQueue() // arrival queue
	g.AddOutQueue(q)
	// aq := blocks.NewQueue() // recirculated ax queue (second phase)
	// pq := blocks.NewQueue() // recirculated proc queue (third phase)
	engine.RegisterActor(g)

	engine.Run(duration)
}

func main() {
	// single_core_deterministic(10, 10, 110)
	// chained_cores_multi_phase_deterministic(10, 10, 110, 2)
	naive_chained_cores_single_queue_three_phase(10, 10, 110, 2, 2, 1)
}
