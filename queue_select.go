package main

import "github.com/neel-patel-1/xmp_sched_sim/engine"

func firstNonEmptyQueue(inQueues []engine.QueueInterface) int {
	// fmt.Println("GPCore: Choosing inQueue")
	for i, q := range inQueues {
		if q.Len() > 0 {
			return i
		}
	}
	return -1
}
