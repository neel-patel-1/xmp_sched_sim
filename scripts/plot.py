#!/bin/python3
import matplotlib.pyplot as plt
import re

# Selected topology: 4
# Cores:16        Accelerators:8  Mu:0.100000     Lambda:0.021333 axCoreQueueSize:128     axCoreSpeedup:2.000000  genType:0       phase_one_ratio:0.250000        phase_two_ratio:0.500000        phase_three_ratio:0.250000
# Stats collector: Main Stats
# Count   Stolen  AVG     STDDev  50th    90th    95th    99th    Reqs/time_unit
# 2178    0       7.577755927922724       10.425712204294848      5.162795236057718       17.610952936767717      23.057183618526324      36.77609786470566       0.021769663390111535
# Selected topology: 4
# Cores:16        Accelerators:8  Mu:0.100000     Lambda:0.426667 axCoreQueueSize:128     axCoreSpeedup:2.000000  genType:0       phase_one_ratio:0.250000        phase_two_ratio:0.500000        phase_three_ratio:0.250000
# Stats collector: Main Stats
# Count   Stolen  AVG     STDDev  50th    90th    95th    99th    Reqs/time_unit
# 42451   0       7.53938019861487        10.260698006392934      5.259818495193031       17.3550882383297        22.389556452117176      34.330565130261675      0.42450750612795
# Selected topology: 4
# Cores:16        Accelerators:8  Mu:0.100000     Lambda:0.640000 axCoreQueueSize:128     axCoreSpeedup:2.000000  genType:0       phase_one_ratio:0.250000        phase_two_ratio:0.500000        phase_three_ratio:0.250000
# Stats collector: Main Stats
# Count   Stolen  AVG     STDDev  50th    90th    95th    99th    Reqs/time_unit
# 64651   0       7.56497874140818        10.303733053105766      5.252098402517731       17.4512989582272        22.458943500299938      34.67088314820239       0.6465083366081079
# Selected topology: 4
# Cores:16        Accelerators:8  Mu:0.100000     Lambda:0.853333 axCoreQueueSize:128     axCoreSpeedup:2.000000  genType:0       phase_one_ratio:0.250000        phase_two_ratio:0.500000        phase_three_ratio:0.250000
# Stats collector: Main Stats
# Count   Stolen  AVG     STDDev  50th    90th    95th    99th    Reqs/time_unit
# 85152   0       7.678546909774991       10.528049884047446      5.29722603301343        17.7490579168807        23.023445414713933      35.545139506246414      0.8515183575834937
# Selected topology: 4
# Cores:16        Accelerators:8  Mu:0.100000     Lambda:1.066667 axCoreQueueSize:128     axCoreSpeedup:2.000000  genType:0       phase_one_ratio:0.250000        phase_two_ratio:0.500000        phase_three_ratio:0.250000
# Stats collector: Main Stats
# Count   Stolen  AVG     STDDev  50th    90th    95th    99th    Reqs/time_unit
# 106533  0       7.799268511512185       10.668007208127623      5.4125289297735435      17.990115623615566      23.482614957058104      35.75505138041626       1.065321915365549

data = """
Selected topology: 4
Cores:16        Accelerators:8  Mu:0.100000     Lambda:0.021333 axCoreQueueSize:128     axCoreSpeedup:2.000000  genType:0       phase_one_ratio:0.250000        phase_two_ratio:0.500000        phase_three_ratio:0.250000
Stats collector: Main Stats
Count   Stolen  AVG     STDDev  50th    90th    95th    99th    Reqs/time_unit
2178    0       7.577755927922724       10.425712204294848      5.162795236057718       17.610952936767717      23.057183618526324      36.77609786470566       0.021769663390111535
Selected topology: 4
Cores:16        Accelerators:8  Mu:0.100000     Lambda:0.426667 axCoreQueueSize:128     axCoreSpeedup:2.000000  genType:0       phase_one_ratio:0.250000        phase_two_ratio:0.500000        phase_three_ratio:0.250000
Stats collector: Main Stats
Count   Stolen  AVG     STDDev  50th    90th    95th    99th    Reqs/time_unit
42451   0       7.53938019861487        10.260698006392934      5.259818495193031       17.3550882383297        22.389556452117176      34.330565130261675      0.42450750612795
Selected topology: 4
Cores:16        Accelerators:8  Mu:0.100000     Lambda:0.640000 axCoreQueueSize:128     axCoreSpeedup:2.000000  genType:0       phase_one_ratio:0.250000        phase_two_ratio:0.500000        phase_three_ratio:0.250000
Stats collector: Main Stats
Count   Stolen  AVG     STDDev  50th    90th    95th    99th    Reqs/time_unit
64651   0       7.56497874140818        10.303733053105766      5.252098402517731       17.4512989582272        22.458943500299938      34.67088314820239       0.6465083366081079
Selected topology: 4
Cores:16        Accelerators:8  Mu:0.100000     Lambda:0.853333 axCoreQueueSize:128     axCoreSpeedup:2.000000  genType:0       phase_one_ratio:0.250000        phase_two_ratio:0.500000        phase_three_ratio:0.250000
Stats collector: Main Stats
Count   Stolen  AVG     STDDev  50th    90th    95th    99th    Reqs/time_unit
85152   0       7.678546909774991       10.528049884047446      5.29722603301343        17.7490579168807        23.023445414713933      35.545139506246414      0.8515183575834937
Selected topology: 4
Cores:16        Accelerators:8  Mu:0.100000     Lambda:1.066667 axCoreQueueSize:128     axCoreSpeedup:2.000000  genType:0       phase_one_ratio:0.250000        phase_two_ratio:0.500000        phase_three_ratio:0.250000
Stats collector: Main Stats
Count   Stolen  AVG     STDDev  50th    90th    95th    99th    Reqs/time_unit
106533  0       7.799268511512185       10.668007208127623      5.4125289297735435      17.990115623615566      23.482614957058104      35.75505138041626       1.065321915365549
"""

# Extract data using regex
lambda_values = [float(x) for x in re.findall(r'Lambda:(\d+\.\d+)', data)]
avg_latency = [float(x.split()[2]) for x in re.findall(r'\d+\s+\d+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+', data)]
percentile_99_latency = [float(x.split()[7]) for x in re.findall(r'\d+\s+\d+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+', data)]
print(lambda_values)
print(avg_latency)
print(percentile_99_latency)

# Ensure all lists have the same length
assert len(lambda_values) == len(avg_latency) == len(percentile_99_latency), "Data lists have different lengths"

# Plot lambda vs. average latency
plt.figure(figsize=(10, 5))
plt.subplot(1, 2, 1)
plt.plot(lambda_values, avg_latency, marker='o')
plt.xlabel('Lambda')
plt.ylabel('Average Latency')
plt.title('Lambda vs. Average Latency')

# Plot lambda vs. 99th percentile latency
plt.subplot(1, 2, 2)
plt.plot(lambda_values, percentile_99_latency, marker='o')
plt.xlabel('Lambda')
plt.ylabel('99th Percentile Latency')
plt.title('Lambda vs. 99th Percentile Latency')

plt.tight_layout()
plt.savefig('plot.png')