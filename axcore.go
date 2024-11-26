package main

import (
	"log"
)

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
			// fmt.Println(p.GetOutQueues())
			p.WriteOutQueueI(req, outQueueIdx)
		} else {
			// Handle non-multi-phase requests
			log.Fatalf("Error: RTCMPProcessor received a non-multi-phase request")
		}
	}
}
