#!/bin/bash
source env/bin/activate
python3 scripts/run_until_saturation.py run 16 8 128 4 0.1 0 .25 .5 .25 2 c_post_exp
python3 scripts/run_until_saturation.py run 16 8 128 4 0.1 2 .25 .5 .25 2 c_post_bimodal
python3 scripts/run_until_saturation.py run 16 8 128 3 0.1 2 .25 .5 .25 2 d_post_bimodal
python3 scripts/run_until_saturation.py run 16 8 128 3 0.1 0 .25 .5 .25 2 d_post_exp
python3 scripts/plot.py .