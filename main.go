package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/neel-patel-1/xmp_sched_sim/blocks"
	"github.com/neel-patel-1/xmp_sched_sim/engine"
)

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

	var post_qs = make([]engine.QueueInterface, num_cores)

	for i := 0; i < num_cores; i++ {
		gpCore := &GPCore{}
		gpCore.outboundMax = axCoreQueueSize
		gpCore.queueChooseFunc = firstNonEmptyQueue
		gpCore.gpCoreForwardFunc = tryAxCoreOutqueueThenFallback
		gpCore.gpCoreIdx = i
		post_qs[i] = blocks.NewQueue()
		gpCore.AddInQueue(post_qs[i])
		gpCore.AddOutQueue(ax_q)
		gpCore.AddInQueue(q)
		gpCore.SetReqDrain(stats)
		engine.RegisterActor(gpCore)
	}

	for j := 0; j < num_accelerators; j++ {
		axCore := &AXCore{}
		axCore.forwardFunc = forwardToOffloader
		axCore.speedup = speedup
		for i := 0; i < num_cores; i++ {
			axCore.AddOutQueue(post_qs[i])
		}
		axCore.AddInQueue(ax_q)
		engine.RegisterActor(axCore)
	}

	engine.RegisterActor(g)

	fmt.Printf("Cores:%d\tAccelerators:%d\tMu:%f\tLambda:%f\taxCoreQueueSize:%d\taxCoreSpeedup:%f\tgenType:%d\tphase_one_ratio:%f\tphase_two_ratio:%f\tphase_three_ratio:%f\n", num_cores, num_accelerators, mu, lambda, axCoreQueueSize, speedup, genType, phase_one_ratio, phase_two_ratio, phase_three_ratio)
	engine.Run(duration)

}

func multi_gpcore_multi_axcore_three_phase(duration float64, speedup float64,
	num_cores int, num_accelerators int, axCoreQueueSize int, lambda, mu float64, genType int,
	phase_one_ratio float64, phase_two_ratio float64, phase_three_ratio float64, axCoreForwardFunc ForwardDecisionProcedure,
	gpCoreForwardFunc gpCoreForwardDecisionProcedure, gpCoreQueueChooseFunc QueueChooseProcedure) {

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
	// g = blocks.NewDDGenerator(1/lambda, 1/mu)
	g.SetCreator(&ThreePhaseReqCreator{phase_one_ratio: phase_one_ratio, phase_two_ratio: phase_two_ratio, phase_three_ratio: phase_three_ratio})
	q := blocks.NewQueue()
	c_post_q := blocks.NewQueue()
	g.AddOutQueue(q)

	ax_q := blocks.NewQueue()

	var post_qs = make([]engine.QueueInterface, num_cores)

	for i := 0; i < num_cores; i++ {
		gpCore := &GPCore{}
		gpCore.outboundMax = axCoreQueueSize
		gpCore.queueChooseFunc = gpCoreQueueChooseFunc
		gpCore.gpCoreForwardFunc = gpCoreForwardFunc
		gpCore.gpCoreIdx = i
		post_qs[i] = blocks.NewQueue()
		gpCore.AddInQueue(post_qs[i])
		gpCore.AddInQueue(c_post_q)
		gpCore.AddOutQueue(ax_q)
		gpCore.AddInQueue(q)
		gpCore.SetReqDrain(stats)
		engine.RegisterActor(gpCore)
	}

	for j := 0; j < num_accelerators; j++ {
		axCore := &AXCore{}
		axCore.forwardFunc = axCoreForwardFunc
		axCore.speedup = speedup
		axCore.AddOutQueue(c_post_q)
		axCore.AddOutQueue(q)
		for i := 0; i < num_cores; i++ {
			axCore.AddOutQueue(post_qs[i])
		}
		axCore.AddInQueue(ax_q)
		engine.RegisterActor(axCore)
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

	var gpcore_offload_style = flag.Int("gpcore_offload_style", 0, "gpcore offload style")
	var axcore_notify_recipient = flag.Int("axcore_notify_recipient", 0, "axcore notify recipient")
	var gpcore_input_queue_selector = flag.Int("gpcore_input_queue_selector", 0, "gpcore input queue selector")

	var axCoreForwardFunc ForwardDecisionProcedure
	var gpCoreForwardFunc gpCoreForwardDecisionProcedure
	var gpCoreQueueChooseFunc QueueChooseProcedure

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
	if *topo == 4 {
		multi_gpcore_multi_axcore_prefn_centralized_axfn_centralized_postfn_returntosender(*duration, *speedup, *num_cores, *num_accelerators, *bufferSize, *lambda, *mu, *genType, *phase_one_ratio, *phase_two_ratio, *phase_three_ratio)
	}

	if *gpcore_offload_style == 0 {
		gpCoreForwardFunc = tryAxCoreOutqueueThenFallback
	}
	if *gpcore_offload_style == 1 {
		gpCoreForwardFunc = blockUntilAxcoreAccepts
	}

	if *axcore_notify_recipient == 0 {
		axCoreForwardFunc = forwardToCentralizedPostProcThreePhase
		fmt.Printf("axCoreForwardFunc: %v\n", axCoreForwardFunc)
	}
	if *axcore_notify_recipient == 1 {
		axCoreForwardFunc = forwardToCentralizedPreProcThreePhase
		fmt.Printf("axCoreForwardFunc: %v\n", axCoreForwardFunc)
	}
	if *axcore_notify_recipient == 2 {
		axCoreForwardFunc = forwardToOffloaderThreePhase
		fmt.Printf("axCoreForwardFunc: %v\n", axCoreForwardFunc)
	}

	if *gpcore_input_queue_selector == 0 {
		gpCoreQueueChooseFunc = firstNonEmptyQueue
	}

	if *topo == 5 {
		multi_gpcore_multi_axcore_three_phase(
			*duration,
			*speedup,
			*num_cores,
			*num_accelerators,
			*bufferSize,
			*lambda,
			*mu,
			*genType,
			*phase_one_ratio,
			*phase_two_ratio,
			*phase_three_ratio,
			axCoreForwardFunc,
			gpCoreForwardFunc,
			gpCoreQueueChooseFunc,
		)
	}

}
