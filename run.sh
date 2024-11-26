#!/bin/bash
source env/bin/activate
python3 scripts/run_many_three_phase.py run 16 8 128 0.1 0 .25 .5 .25 2 0 2 0 c_post_exp_three_phase
python3 scripts/run_many_three_phase.py run 16 8 128 0.1 2 .25 .5 .25 2 0 2 0 c_post_bimodal_three_phase
python3 scripts/run_many_three_phase.py run 16 8 128 0.1 2 .25 .5 .25 2 0 2 0 d_post_bimodal_three_phase
python3 scripts/run_many_three_phase.py run 16 8 128 0.1 0 .25 .5 .25 2 0 2 0 d_post_exp_three_phase
python3 scripts/plot.py . _three_phase