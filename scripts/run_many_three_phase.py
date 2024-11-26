#!/usr/bin/python3

import fire
import os
import sys
import subprocess
from multiprocessing import Process
from enum import Enum, auto
import csv
from pprint import pprint
import re

# Only written for single Q
load_levels = [0.01, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 0.95, 0.99]
metrics = ['Count', 'Stolen', 'AVG', 'STDDev',
           '50th', '90th', '95th', '99th', 'Reqs/time_unit']


def run(num_cores, num_accelerators, bufferSize, mu, gen_type, phase_one_ratio, phase_two_ratio, phase_three_ratio, speedup,
        gpcore_offload_style, axcore_notify_recipient, gpcore_input_queue_selector, name):
    '''
    mu in us
    '''
    duration = 100000
    service_time_per_core_us = ((1 / mu) * .33)
    rps_capacity_per_core = 1 / service_time_per_core_us * 1000.0 * 1000.0
    total_rps_capacity = rps_capacity_per_core * num_cores
    load_levels = [i * 0.1 for i in range(1, 11)]  # Load levels from 0.1 to 1.0
    injected_rps = [load_lvl * total_rps_capacity for load_lvl in load_levels]
    lambdas = [rps / 1000.0 / 1000.0 for rps in injected_rps]
    res_file = f"{name}.txt"
    saturation_threshold = 100 * service_time_per_core_us  # Define saturation threshold

    with open(res_file, 'w') as f:
        for l in lambdas:
            cmd = (
                f"./xmp_sched_sim --topo=5 --mu={mu} --genType={gen_type} "
                f"--phase_one_ratio={phase_one_ratio} --phase_two_ratio={phase_two_ratio} "
                f"--phase_three_ratio={phase_three_ratio} --speedup={speedup} "
                f"--lambda={l} --num_cores={num_cores} --num_accelerators={num_accelerators} "
                f"--buffersize={bufferSize} "
                f"--duration={duration} "
                f"--gpcore_offload_style={gpcore_offload_style} "
                f"--axcore_notify_recipient={axcore_notify_recipient} "
                f"--gpcore_input_queue_selector={gpcore_input_queue_selector}"
            )
            print(f"Running... {cmd}")
            result = subprocess.run(cmd, capture_output=True, text=True, shell=True)
            f.write(result.stdout)
            result_str = result.stdout

            # Check for saturation point
            line = re.findall(r'\d+\s+\d+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+', result_str)
            # get the second element of the list
            if len(line) > 0:
                avg_latency = float(line[0].split()[2])
                if avg_latency > saturation_threshold:
                    print(f"Saturation point reached at lambda={l} with avg latency={avg_latency}")
                    break
            else:
                os.error("No data found in the output file")
        # keep running if the last lambda did not reach saturation
        if avg_latency <= saturation_threshold:
            for l in lambdas:
                cmd = (
                    f"./xmp_sched_sim --topo=5 --mu={mu} --genType={gen_type} "
                    f"--phase_one_ratio={phase_one_ratio} --phase_two_ratio={phase_two_ratio} "
                    f"--phase_three_ratio={phase_three_ratio} --speedup={speedup} "
                    f"--lambda={l + 1.0} --num_cores={num_cores} --num_accelerators={num_accelerators} "
                    f"--buffersize={bufferSize} "
                    f"--duration={duration} "
                    f"--gpcore_offload_style={gpcore_offload_style} "
                    f"--axcore_notify_recipient={axcore_notify_recipient} "
                    f"--gpcore_input_queue_selector={gpcore_input_queue_selector}"
                )
                print(f"Running... {cmd}")
                result = subprocess.run(cmd, capture_output=True, text=True, shell=True)
                f.write(result.stdout)


if __name__ == "__main__":
    fire.Fire({
        "run": run
    })
