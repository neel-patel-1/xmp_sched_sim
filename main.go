package main

import (
	"flag"
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

type AXCore struct {
	mpProcessor
}

// axCore main loop:
//
//	check in queue
//	wait for the full service time of this phase
//	check the coreid of the request to know which in queue to re-enqueue
//	wait for the notification overhead time
//
// write to the outqueue at offset coreid to re-enqueue at the offloading core
func (p *AXCore) Run() {
	for {
		req := p.ReadInQueue()
		//logPrintf("AXCore: Read request %v", req)
		if multiPhaseReq, ok := req.(*MultiPhaseReq); ok {
			curPhase := multiPhaseReq.Current
			//logPrintf("AXCore: Starting phase %v", curPhase)
			if _, exists := multiPhaseReq.Phases[curPhase].Devices[Accelerator]; exists {
				// Accelerator is in the set
				actualServiceTime := req.GetServiceTime() / p.speedup
				p.Wait(actualServiceTime)
				multiPhaseReq.Current++
				//logPrintf("AXCore: Finished phase %v", curPhase)
			} else {
				log.Fatalf("Error: Accelerator is not in the set")
			}
			if multiPhaseReq.Current >= len(multiPhaseReq.Phases) {
				log.Fatalf("Error: Accelerator cannot terminate a request")
			}
			// Forward to the outgoing queue
			outQueueIdx := p.forwardFunc(p.GetOutQueues(), multiPhaseReq)
			p.WriteOutQueueI(req, outQueueIdx)
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
	gpCoreIdx         int
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
		var req engine.ReqInterface
		inQueueIdx := p.queueChooseFunc(p.GetInQueues())
		if inQueueIdx == -1 {
			// dequeue from first non-empty
			req, inQueueIdx = p.ReadInQueues()
			// fmt.Println("GPCore routine said no, so we ReadInQueues -- Read from inQueueIdx: ", inQueueIdx)
		} else {
			req = p.ReadInQueueI(inQueueIdx)
		}
		//fmt.Println("GPCore: Read from inQueueIdx: ", inQueueIdx)
		//fmt.Println(req)
		if multiPhaseReq, ok := req.(*MultiPhaseReq); ok {
			curPhase := multiPhaseReq.Current
		phase_exe:
			if curPhase < len(multiPhaseReq.Phases) {
				// Check if the device is in the set
				if _, exists := multiPhaseReq.Phases[multiPhaseReq.Current].Devices[Processor]; exists {
					// Processor is in the set
					p.Wait(multiPhaseReq.GetServiceTime())
					multiPhaseReq.Current++
					multiPhaseReq.lastGPCoreIdx = p.gpCoreIdx
					//fmt.Printf("GPCore: Finished phase %v\n", curPhase)
				} else {
					log.Fatalf("Error: Processor is not in the set")
				}

				// Check if We just finished the last phase
				if multiPhaseReq.Current >= len(multiPhaseReq.Phases) {
					//fmt.Println("GPCore: Last phase, terminating request")
					p.reqDrain.TerminateReq(req)
					goto read_inqueue
				}

				// Forward to the outgoing queue
				outQueueIdx := p.gpCoreForwardFunc(p, p.GetOutQueues(), multiPhaseReq)
				if outQueueIdx == -1 {
					//fmt.Printf("Waiting for the full service time for phase %v\n", multiPhaseReq.Current)
					p.Wait(multiPhaseReq.GetServiceTime())
					multiPhaseReq.Current++
					goto phase_exe
				} else {
					//fmt.Printf("Enqueueing phase %v into outQueueIdx: %v\n", multiPhaseReq.Current, outQueueIdx)
					p.Wait(p.offloadCost)
					p.WriteOutQueueI(req, outQueueIdx)
				}
			} else {
				// Last phase, terminate the request
				log.Fatalf("Error: Received a request that has already completed all phases")
			}
		} else {
			// Handle non-multi-phase requests
			//fmt.Println(multiPhaseReq)
			log.Fatalf("Error: NaiveOffloadingProcessor received a non-multi-phase request")

		}
	}
}

func priqueue_choose(inQueues []engine.QueueInterface) int {
	for i, q := range inQueues {
		if q.Len() > 0 {
			return i
		}
	}
	return -1
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

func firstNonEmptyQueue(inQueues []engine.QueueInterface) int {
	// fmt.Println("GPCore: Choosing inQueue")
	for i, q := range inQueues {
		if q.Len() > 0 {
			return i
		}
	}
	return -1
}

func tryAllThenFallback(p *GPCore, outQueues []engine.QueueInterface, req *MultiPhaseReq) int {
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

func forwardToOffloader(outQueues []engine.QueueInterface, req *MultiPhaseReq) int {
	// re-enqueue at the offloading gpCore
	outQueueIdx := req.lastGPCoreIdx
	return outQueueIdx
}

func fallback_gpcore_core_three_phase_single(interarrival_time, service_time, duration float64, speedup float64,
	num_cores int, num_accelerators int, axCoreQueueSize int) {
	engine.InitSim()

	stats := &blocks.AllKeeper{}
	stats.SetName("Main Stats")
	engine.InitStats(stats)

	// Add generator && set up dispatcher
	g := blocks.NewDDGenerator(interarrival_time, service_time)
	// g.SetCreator(&ThreePhaseReqCreator{phase_one_ratio: 0.1, phase_two_ratio: 0.6, phase_three_ratio: 0.3}) // Update-Filter-Histogram-1KB
	g.SetCreator(&ThreePhaseReqCreator{phase_one_ratio: 0.25, phase_two_ratio: 0.5, phase_three_ratio: 0.25}) // dummy for testing
	q := blocks.NewQueue()                                                                                    // arrival queue

	// create gpCore
	gpCore := &GPCore{}
	gpCore.outboundMax = axCoreQueueSize
	gpCore.queueChooseFunc = firstNonEmptyQueue
	gpCore.gpCoreForwardFunc = tryAllThenFallback

	//create axCore
	axCore := &AXCore{}
	axCore.forwardFunc = forwardToOffloader
	axCore.speedup = speedup

	// link post-processing queue of gpCore to output of axCore
	postQueue := blocks.NewQueue()
	// add this one first -- highest priority
	axCore.AddOutQueue(postQueue)
	gpCore.gpCoreIdx = 0         // indicates the outgoing queue index to use to re-enqueue at this gpCore
	gpCore.AddInQueue(postQueue) // post-processing input queue (produced by axCore)
	gpCore.SetReqDrain(stats)

	axQueue := blocks.NewQueue()
	gpCore.AddOutQueue(axQueue) // axCore input queue (produced by gpCore)
	axCore.AddInQueue(axQueue)  // axCore input queue (produced by gpCore)

	gpCore.AddInQueue(q) // pre-processing queue (produced by load gen)

	engine.RegisterActor(gpCore)
	engine.RegisterActor(axCore)

	g.AddOutQueue(q)
	engine.RegisterActor(g)

	// create an in queue used by the axCore to re-enqueue the third phase back at the GPCore

	engine.Run(duration)
}

func fallback_multi_gpcore_axcore_three_phase(duration float64, speedup float64,
	num_cores int, num_accelerators int, axCoreQueueSize int, lambda, mu float64, genType int,
	phase_one_ratio float64, phase_two_ratio float64, phase_three_ratio float64) {

	engine.InitSim()
	stats := &blocks.AllKeeper{}
	stats.SetName("Main Stats")
	engine.InitStats(stats)

	var g blocks.Generator
	if genType == 0 {
		g = blocks.NewMMRandGenerator(lambda, mu)
	} else if genType == 1 {
		g = blocks.NewMDRandGenerator(lambda, 1/mu)
	} else if genType == 2 {
		g = blocks.NewMBRandGenerator(lambda, 1, 10*(1/mu-0.9), 0.9)
	} else if genType == 3 {
		g = blocks.NewMBRandGenerator(lambda, 1, 1000*(1/mu-0.999), 0.999)
	}
	g.SetCreator(&ThreePhaseReqCreator{phase_one_ratio: phase_one_ratio, phase_two_ratio: phase_two_ratio, phase_three_ratio: phase_three_ratio})
	q := blocks.NewQueue()
	g.AddOutQueue(q)

	// determine how many gpCores to an axCore
	num_gpCores := num_cores / num_accelerators
	if num_cores%num_accelerators != 0 {
		log.Fatalf("Error: Number of cores must be divisible by the number of accelerators")
	}

	num_clusters := num_accelerators
	for i := 0; i < num_clusters; i++ {
		axQueue := blocks.NewQueue()

		//create axCore
		axCore := &AXCore{}
		axCore.forwardFunc = forwardToOffloader
		axCore.speedup = speedup
		axCore.AddInQueue(axQueue)

		for j := 0; j < num_gpCores; j++ {
			postQueue := blocks.NewQueue()

			// create gpCore
			gpCore := &GPCore{}
			gpCore.outboundMax = axCoreQueueSize
			gpCore.queueChooseFunc = firstNonEmptyQueue
			gpCore.gpCoreForwardFunc = tryAllThenFallback
			gpCore.gpCoreIdx = j

			// link post-processing queue of gpCore to output of axCore
			axCore.AddOutQueue(postQueue)
			gpCore.gpCoreIdx = j
			gpCore.AddInQueue(postQueue)
			gpCore.SetReqDrain(stats)

			gpCore.AddOutQueue(axQueue)

			gpCore.AddInQueue(q)

			engine.RegisterActor(gpCore)
		}

		engine.RegisterActor(axCore)
	}

	engine.RegisterActor(g)
	engine.Run(duration)

}

func main() {
	var topo = flag.Int("topo", 0, "topology selector")
	var mu = flag.Float64("mu", 0.02, "mu service rate") // default 50usec
	var lambda = flag.Float64("lambda", 0.005, "lambda poisson interarrival")
	var genType = flag.Int("genType", 0, "type of generator")
	var duration = flag.Float64("duration", 10000000, "experiment duration")
	var bufferSize = flag.Int("buffersize", 32, "size of each axCore's buffer")
	var num_cores = flag.Int("num_cores", 16, "number of cores")
	var num_accelerators = flag.Int("num_accelerators", 8, "number of accelerators")

	var phase_one_ratio = flag.Float64("phase_one_ratio", 0.25, "phase one ratio")
	var phase_two_ratio = flag.Float64("phase_two_ratio", 0.5, "phase two ratio")
	var phase_three_ratio = flag.Float64("phase_three_ratio", 0.25, "phase three ratio")

	flag.Parse()
	fmt.Printf("Selected topology: %v\n", *topo)

	if *topo == 0 {
		// single_core_deterministic(*lambda, *mu, *duration)
		chained_cores_multi_phase_deterministic(*lambda, *mu, *duration, 2)
	}
	if *topo == 1 {
		fallback_gpcore_core_three_phase_single(*lambda, *mu, *duration, 2, *num_cores, *num_accelerators, *bufferSize)
	}
	if *topo == 2 {
		fallback_multi_gpcore_axcore_three_phase(*duration, 2, *num_cores, *num_accelerators, *bufferSize, *lambda, *mu, *genType, *phase_one_ratio, *phase_two_ratio, *phase_three_ratio)
	}

}
