#!/bin/bash

# NUM_CORES NUM_ACCELERATORS BUFFERSIZE MU GEN_TYPE PHASE_ONE_RATIO PHASE_TWO_RATIO PHASE_THREE_RATIO SPEEDUP GPCORE_OFFLOAD_STYLE AXCORE_NOTIFY_RECIPIENT GPCORE_INPUT_QUEUE_SELECTOR DURATION NAME

source env/bin/activate
python3 scripts/run_many_three_phase.py run 16 8 128 0.1 0 .25 .5 .25 2 0 0 0 1000 c_post_exp_three_phase
python3 scripts/run_many_three_phase.py run 16 8 128 0.1 2 .25 .5 .25 2 0 0 0 1000 c_post_bimodal_three_phase
python3 scripts/run_many_three_phase.py run 16 8 128 0.1 2 .25 .5 .25 2 0 2 0 1000 d_post_bimodal_three_phase
python3 scripts/run_many_three_phase.py run 16 8 128 0.1 0 .25 .5 .25 2 0 2 0 1000 d_post_exp_three_phase
python3 scripts/plot.py . _three_phase