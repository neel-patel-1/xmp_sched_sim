package main

import (
	"log"

	"github.com/neel-patel-1/xmp_sched_sim/engine"
)

func forwardToOffloader(outQueues []engine.QueueInterface, req *MultiPhaseReq) int {
	// re-enqueue at the offloading gpCore
	outQueueIdx := req.lastGPCoreIdx
	return outQueueIdx
}

func forwardToCentralized(outQueues []engine.QueueInterface, req *MultiPhaseReq) int {
	// re-enqueue at the centralized processor
	return 0
}

func tryAxCoreOutqueueThenFallback(p *GPCore, outQueues []engine.QueueInterface, req *MultiPhaseReq) int {
	if len(outQueues) > 1 {
		log.Fatal("GPCore: More than one axCore is not supported")
	}

	if outQueues[0].Len() < p.outboundMax {
		return 0
	}
	return -1
}
