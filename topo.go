package main

import (
	"fmt"
	"log"

	"github.com/neel-patel-1/xmp_sched_sim/blocks"
	"github.com/neel-patel-1/xmp_sched_sim/engine"
)

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
			gpCore.gpCoreForwardFunc = tryAxCoreOutqueueThenFallback
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

	fmt.Printf("Cores:%d\tAccelerators:%d\tMu:%f\tLambda:%f\taxCoreQueueSize:%d\taxCoreSpeedup:%f\tgenType:%d\tphase_one_ratio:%f\tphase_two_ratio:%f\tphase_three_ratio:%f\n", num_cores, num_accelerators, mu, lambda, axCoreQueueSize, speedup, genType, phase_one_ratio, phase_two_ratio, phase_three_ratio)
	engine.Run(duration)

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
	gpCore.gpCoreForwardFunc = tryAxCoreOutqueueThenFallback

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
