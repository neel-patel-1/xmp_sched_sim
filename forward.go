package main

import (
	"log"

	"github.com/neel-patel-1/xmp_sched_sim/engine"
)

type ForwardDecisionProcedure func(outQueues []engine.QueueInterface, req *MultiPhaseReq) int

type QueueChooseProcedure func(inQueues []engine.QueueInterface) int

func forwardToOffloader(outQueues []engine.QueueInterface, req *MultiPhaseReq) int {
	// re-enqueue at the offloading gpCore
	outQueueIdx := req.lastGPCoreIdx
	return outQueueIdx
}

func forwardToCentralized(outQueues []engine.QueueInterface, req *MultiPhaseReq) int {
	// re-enqueue at the centralized processor
	return 0
}

// Fully Connected Network With Queue Indices Selected According to topology specified in multi_gpcore_multi_axcore_three_phase
func forwardToCentralizedPostProcThreePhase(outQueues []engine.QueueInterface, req *MultiPhaseReq) int {
	// re-enqueue at the centralized processor
	return 0
}

func forwardToCentralizedPreProcThreePhase(outQueues []engine.QueueInterface, req *MultiPhaseReq) int {
	// re-enqueue at the centralized processor
	return 1
}

func forwardToOffloaderThreePhase(outQueues []engine.QueueInterface, req *MultiPhaseReq) int {
	// re-enqueue at the offloading gpCore
	outQueueIdx := req.lastGPCoreIdx + 2
	return outQueueIdx
}

type gpCoreForwardDecisionProcedure func(p *GPCore, outQueues []engine.QueueInterface, req *MultiPhaseReq) int

func tryAxCoreOutqueueThenFallback(p *GPCore, outQueues []engine.QueueInterface, req *MultiPhaseReq) int {
	if len(outQueues) > 1 {
		log.Fatal("GPCore: More than one axCore is not supported")
	}

	if outQueues[0].Len() < p.outboundMax {
		return 0
	}
	return -1
}

func blockUntilAxcoreAccepts(p *GPCore, outQueues []engine.QueueInterface, req *MultiPhaseReq) int {
	if len(outQueues) > 1 {
		log.Fatal("GPCore: More than one axCore is not supported")
	}

	for outQueues[0].Len() >= p.outboundMax {
		p.Wait(p.offloadCost)
	}
	return 0
}
