package main

import (
	"log"

	"github.com/neel-patel-1/xmp_sched_sim/engine"
)

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
