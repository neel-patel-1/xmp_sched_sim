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
	Devices map[DeviceType]struct{}
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
	return m.Phases[m.Current].GetDelay()
}

func (m *MultiPhaseReq) GetServiceTime() float64 {
	return m.Phases[m.Current].GetServiceTime()
}

type ForwardDecisionProcedure func(outQueues []engine.QueueInterface, req *MultiPhaseReq) int

type gpCoreForwardDecisionProcedure func(p *GPCore, outQueues []engine.QueueInterface, req *MultiPhaseReq) int
type QueueChooseProcedure func(inQueues []engine.QueueInterface) int

type mpProcessor struct {
	engine.Actor
	reqDrain        blocks.RequestDrain
	ctxCost         float64
	offloadCost     float64
	deviceType      DeviceType
	speedup         float64
	forwardFunc     ForwardDecisionProcedure
	queueChooseFunc QueueChooseProcedure
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

// Block Until Success, Offloading Processor with three queues, one for each phase
type GPCore struct {
	mpProcessor
	gpCoreForwardFunc gpCoreForwardDecisionProcedure
	lastOutQueue      int
	outboundMax       int
}

// determine the idx of the queue to read from <- parameterizable (maybe we just have one queue)
// check the in queue corresponding to that idx
// wait for the full service time of this phase and increment the phase counter
// get the index of the outqueue to queue into
// if the index is nil
// 	wait for the full service time
// otherwise
// enqueue the request into the returned outqueue

func (p *GPCore) Run() {
	for {
	read_inqueue:
		inQueueIdx := p.queueChooseFunc(p.GetInQueues())
		req := p.ReadInQueueI(inQueueIdx)
		if multiPhaseReq, ok := req.(*MultiPhaseReq); ok {
			curPhase := multiPhaseReq.Current
		phase_exe:
			if curPhase < len(multiPhaseReq.Phases) {
				// Check if the device is in the set
				if _, exists := multiPhaseReq.Phases[multiPhaseReq.Current].Devices[Processor]; exists {
					// Processor is in the set
					fmt.Println("Processor is in the set")
					p.Wait(multiPhaseReq.GetServiceTime())
					multiPhaseReq.Current++
				}

				// Check if We just finished the last phase
				if multiPhaseReq.Current >= len(multiPhaseReq.Phases) {
					p.reqDrain.TerminateReq(req)
					goto read_inqueue
				}

				// Forward to the outgoing queue
				outQueueIdx := p.forwardFunc(p.GetOutQueues(), multiPhaseReq)
				if outQueueIdx == -1 {
					fmt.Printf("Waiting for the full service time for phase %v\n", multiPhaseReq.Current)
					p.Wait(multiPhaseReq.GetServiceTime())
					multiPhaseReq.Current++
					goto phase_exe
				} else {
					fmt.Printf("Enqueueing phase %v into outQueueIdx: %v\n", multiPhaseReq.Current, outQueueIdx)
					p.Wait(p.offloadCost)
					p.WriteOutQueueI(req, outQueueIdx)
				}
			} else {
				// Last phase, terminate the request
				log.Fatalf("Error: Received a request that has already completed all phases")
			}
		} else {
			// Handle non-multi-phase requests
			log.Fatalf("Error: NaiveOffloadingProcessor received a non-multi-phase request")
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

func fallback_chained_cores_single_queue_three_phase(interarrival_time, service_time, duration float64, speedup float64,
	num_cores int, num_accelerators int) {
	engine.InitSim()

	stats := &blocks.AllKeeper{}
	stats.SetName("Main Stats")
	engine.InitStats(stats)

	// Add generator && set up dispatcher
	g := blocks.NewDDGenerator(interarrival_time, service_time)
	g.SetCreator(&ThreePhaseReqCreator{phase_one_ratio: 0.1, phase_two_ratio: 0.6, phase_three_ratio: 0.3}) // Update-Filter-Histogram-1KB
	q := blocks.NewQueue()                                                                                  // arrival queue
	g.AddOutQueue(q)
	engine.RegisterActor(g)

	// create axCore
	gpCore := &GPCore{}
	gpCore.queueChooseFunc = func(inQueues []engine.QueueInterface) int {
		for i, q := range inQueues {
			if q.Len() > 0 {
				return i
			}
		}
		return -1
	}
	gpCore.gpCoreForwardFunc = func(p *GPCore, outQueues []engine.QueueInterface, req *MultiPhaseReq) int {
		// equally balance this GPCore's offloads across all available axCores
		outQueue := (p.lastOutQueue + 1) % len(outQueues)

		// if our preferred axCore is full, try the next one
		for tried_queues := 0; outQueues[outQueue].Len() == p.outboundMax && tried_queues < len(outQueues); tried_queues++ {
			p.Wait(p.offloadCost)
			outQueue = (outQueue + 1) % len(outQueues)
			if tried_queues == len(outQueues) {
				// until we've tried all and need fallback
				return -1
			}
		}
		p.lastOutQueue = outQueue
		return outQueue
	}

	// create an in queue used by the axCore to re-enqueue the third phase back at the GPCore
	for i := 0; i < num_cores; i++ {
		aq := blocks.NewQueue()
		p := &RTCMPProcessor{}

		p.AddOutQueue(aq)
		p.SetSpeedup(1)
		p.SetDeviceType(Processor)

	}

	engine.Run(duration)
}

func main() {
	// single_core_deterministic(10, 10, 110)
	chained_cores_multi_phase_deterministic(10, 10, 110, 2)
	fallback_chained_cores_single_queue_three_phase(10, 10, 110, 2, 2, 1)
}
