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


def run(num_cores, num_accelerators, bufferSize, topo, mu, gen_type, phase_one_ratio, phase_two_ratio, phase_three_ratio, speedup):
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
    res_file = "out.txt"
    saturation_threshold = 100 * service_time_per_core_us  # Define saturation threshold

    with open(res_file, 'w') as f:
        for l in lambdas:
            cmd = (
                f"./xmp_sched_sim --topo={topo} --mu={mu} --genType={gen_type} "
                f"--phase_one_ratio={phase_one_ratio} --phase_two_ratio={phase_two_ratio} "
                f"--phase_three_ratio={phase_three_ratio} --speedup={speedup} "
                f"--lambda={l} --num_cores={num_cores} --num_accelerators={num_accelerators} "
                f"--buffersize={bufferSize} "
                f"--duration={duration}"
            )
            print(f"Running... {cmd}")
            result = subprocess.run(cmd, capture_output=True, text=True, shell=True)
            f.write(result.stdout)
            result_str = result.stdout

            # Extract data using regex
            # ['48490\t0\t7.526024073868476\t10.277456027506025\t5.23973179954919\t17.32075368418009\t22.312293480688822\t34.441257869053516\t0.4848990791066668']
            #split this string

            # Check for saturation point
            line = re.findall(r'\d+\s+\d+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+', result_str)
            # get the second element of the list
            if len(line) > 0:
                avg_latency = float(line[0].split()[2])
                if avg_latency > saturation_threshold:
                    print(f"Saturation point reached at lambda={l} with avg latency={avg_latency}")
                    break
        # keep running if the last lambda did not reach saturation
        if avg_latency <= saturation_threshold:
            for l in lambdas:
                cmd = (
                    f"./xmp_sched_sim --topo={topo} --mu={mu} --genType={gen_type} "
                    f"--phase_one_ratio={phase_one_ratio} --phase_two_ratio={phase_two_ratio} "
                    f"--phase_three_ratio={phase_three_ratio} --speedup={speedup} "
                    f"--lambda={l + 1.0} --num_cores={num_cores} --num_accelerators={num_accelerators} "
                    f"--buffersize={bufferSize} "
                    f"--duration={duration}"
                )
                print(f"Running... {cmd}")
                result = subprocess.run(cmd, capture_output=True, text=True, shell=True)
                f.write(result.stdout)



# Selected topology: 4
# Cores:16        Accelerators:8  Mu:0.100000     Lambda:0.021333 axCoreQueueSize:128     axCoreSpeedup:2.000000  genType:0       phase_one_ratio:0.250000        phase_two_ratio:0.500000        phase_three_ratio:0.250000
# Stats collector: Main Stats
# Count   Stolen  AVG     STDDev  50th    90th    95th    99th    Reqs/time_unit
# 2178    0       7.577755927922724       10.425712204294848      5.162795236057718       17.610952936767717      23.057183618526324      36.77609786470566       0.021769663390111535

def out_to_csv():
    results = []
    with open("out.txt", 'r') as f:
        csv_reader = csv.reader(f, delimiter='\t')
        current_result = {}
        for row in csv_reader:
            if len(row) > 0 and row[0].startswith("# Selected topology"):
                if current_result:
                    results.append(current_result)
                current_result = {"topology": row[0].split(":")[1].strip()}
            elif len(row) > 0 and row[0].startswith("# Cores"):
                parts = row[0].split()
                current_result.update({
                    "Cores": parts[1],
                    "Accelerators": parts[3],
                    "Mu": parts[5],
                    "Lambda": parts[7],
                    "axCoreQueueSize": parts[9],
                    "axCoreSpeedup": parts[11],
                    "genType": parts[13],
                    "phase_one_ratio": parts[15],
                    "phase_two_ratio": parts[17],
                    "phase_three_ratio": parts[19]
                })
            elif len(row) > 0 and row[0].startswith("# Count"):
                metrics_row = row
            elif len(row) > 0 and row[0].isdigit():
                for i, metric in enumerate(metrics):
                    current_result[metric] = row[i]
        if current_result:
            results.append(current_result)

    pprint(results)

    with open("out.csv", 'w') as f:
        writer = csv.writer(f, delimiter='\t')
        writer.writerow(["topology", "Cores", "Accelerators", "Mu", "Lambda", "axCoreQueueSize", "axCoreSpeedup", "genType", "phase_one_ratio", "phase_two_ratio", "phase_three_ratio"] + metrics)
        for result in results:
            writer.writerow([result.get("topology"), result.get("Cores"), result.get("Accelerators"), result.get("Mu"), result.get("Lambda"), result.get("axCoreQueueSize"), result.get("axCoreSpeedup"), result.get("genType"), result.get("phase_one_ratio"), result.get("phase_two_ratio"), result.get("phase_three_ratio")] + [result.get(metric) for metric in metrics])

if __name__ == "__main__":
    fire.Fire({
        "run": run,
        "csv": out_to_csv
    })
