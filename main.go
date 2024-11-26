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

func multi_gpcore_multi_axcore_multi_centralized(duration float64, speedup float64,
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

	ax_q := blocks.NewQueue()
	post_q := blocks.NewQueue()

	for j := 0; j < num_accelerators; j++ {
		axCore := &AXCore{}
		axCore.forwardFunc = forwardToCentralized
		axCore.speedup = speedup
		axCore.AddInQueue(ax_q)
		axCore.AddOutQueue(post_q)
		engine.RegisterActor(axCore)
	}

	for i := 0; i < num_cores; i++ {
		gpCore := &GPCore{}
		gpCore.outboundMax = axCoreQueueSize
		gpCore.queueChooseFunc = firstNonEmptyQueue
		gpCore.gpCoreForwardFunc = tryAxCoreOutqueueThenFallback
		gpCore.gpCoreIdx = i
		gpCore.AddInQueue(post_q)
		gpCore.AddOutQueue(ax_q)
		gpCore.AddInQueue(q)
		gpCore.SetReqDrain(stats)
		engine.RegisterActor(gpCore)
	}

	engine.RegisterActor(g)

	fmt.Printf("Cores:%d\tAccelerators:%d\tMu:%f\tLambda:%f\taxCoreQueueSize:%d\taxCoreSpeedup:%f\tgenType:%d\tphase_one_ratio:%f\tphase_two_ratio:%f\tphase_three_ratio:%f\n", num_cores, num_accelerators, mu, lambda, axCoreQueueSize, speedup, genType, phase_one_ratio, phase_two_ratio, phase_three_ratio)
	engine.Run(duration)

}

func multi_gpcore_multi_axcore_prefn_centralized_axfn_centralized_postfn_returntosender(duration float64, speedup float64,
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

	ax_q := blocks.NewQueue()
	post_q := blocks.NewQueue()

	for j := 0; j < num_accelerators; j++ {
		axCore := &AXCore{}
		axCore.forwardFunc = forwardToOffloader
		axCore.speedup = speedup
		axCore.AddInQueue(ax_q)
		axCore.AddOutQueue(post_q)
		engine.RegisterActor(axCore)
	}

	for i := 0; i < num_cores; i++ {
		gpCore := &GPCore{}
		gpCore.outboundMax = axCoreQueueSize
		gpCore.queueChooseFunc = firstNonEmptyQueue
		gpCore.gpCoreForwardFunc = tryAxCoreOutqueueThenFallback
		gpCore.gpCoreIdx = i
		gpCore.AddInQueue(post_q)
		gpCore.AddOutQueue(ax_q)
		gpCore.AddInQueue(q)
		gpCore.SetReqDrain(stats)
		engine.RegisterActor(gpCore)
	}

	engine.RegisterActor(g)

	fmt.Printf("Cores:%d\tAccelerators:%d\tMu:%f\tLambda:%f\taxCoreQueueSize:%d\taxCoreSpeedup:%f\tgenType:%d\tphase_one_ratio:%f\tphase_two_ratio:%f\tphase_three_ratio:%f\n", num_cores, num_accelerators, mu, lambda, axCoreQueueSize, speedup, genType, phase_one_ratio, phase_two_ratio, phase_three_ratio)
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
	var speedup = flag.Float64("speedup", 1.0, "speedup factor")

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
		fallback_multi_gpcore_axcore_three_phase(*duration, *speedup, *num_cores, *num_accelerators, *bufferSize, *lambda, *mu, *genType, *phase_one_ratio, *phase_two_ratio, *phase_three_ratio)
	}
	if *topo == 3 {
		multi_gpcore_multi_axcore_multi_centralized(*duration, *speedup, *num_cores, *num_accelerators, *bufferSize, *lambda, *mu, *genType, *phase_one_ratio, *phase_two_ratio, *phase_three_ratio)
	}

}
