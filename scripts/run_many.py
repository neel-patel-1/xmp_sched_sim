#!/usr/bin/python3

import fire
import os
import sys
import subprocess
from multiprocessing import Process
from enum import Enum, auto
import csv
from pprint import pprint

# Only written for single Q
load_levels = [0.01, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 0.95, 0.99, 2, 4, 8, 10]
metrics = ['Count', 'Stolen', 'AVG', 'STDDev',
           '50th', '90th', '95th', '99th', 'Reqs/time_unit']


def run( num_cores, num_accelerators, bufferSize, topo, mu, gen_type, phase_one_ratio, phase_two_ratio, phase_three_ratio, speedup):
    '''
    mu in us
    '''
    duration=100000
    service_time_per_core_us = ((1 / mu) * phase_one_ratio) + ((1/mu * phase_two_ratio) / speedup ) + (1/mu * phase_three_ratio)
    rps_capacity_per_core = 1 / service_time_per_core_us * 1000.0 * 1000.0
    total_rps_capacity = rps_capacity_per_core * num_cores
    injected_rps = [load_lvl * total_rps_capacity for load_lvl in load_levels]
    lambdas = [rps / 1000.0 / 1000.0 for rps in injected_rps]
    res_file = "out.txt"

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
            subprocess.run(cmd, stdout=f, shell=True)

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
