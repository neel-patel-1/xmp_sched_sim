import matplotlib.pyplot as plt
import re
import argparse

def parse_data(file_path):
    with open(file_path, 'r') as file:
        data = file.read()

    # Extract data using regex
    lambda_values = [float(x) for x in re.findall(r'Lambda:(\d+\.\d+)', data)]
    avg_latency = [float(x.split()[2]) for x in re.findall(r'\d+\s+\d+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+', data)]
    percentile_99_latency = [float(x.split()[7]) for x in re.findall(r'\d+\s+\d+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+', data)]

    return lambda_values, avg_latency, percentile_99_latency

def plot_data(lambda_values, avg_latency, percentile_99_latency, name):
    # Ensure all lists have the same length
    assert len(lambda_values) == len(avg_latency) == len(percentile_99_latency), "Data lists have different lengths"

    # Plot lambda vs. average latency
    plt.figure(figsize=(10, 5))
    plt.subplot(1, 2, 1)
    plt.plot(lambda_values, avg_latency, marker='o')
    plt.xlabel('Lambda')
    plt.ylabel('Average Latency')
    plt.title('Lambda vs. Average Latency')
    plt.ylim(bottom=0)

    # Plot lambda vs. 99th percentile latency
    plt.subplot(1, 2, 2)
    plt.plot(lambda_values, percentile_99_latency, marker='o')
    plt.xlabel('Lambda')
    plt.ylabel('99th Percentile Latency')
    plt.title('Lambda vs. 99th Percentile Latency')
    plt.ylim(bottom=0)

    plt.tight_layout()
    plt.savefig(name + '.png')

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Plot Lambda vs. Latency from data file.')
    parser.add_argument('file', type=str, help='Path to the data file')
    args = parser.parse_args()

    lambda_values, avg_latency, percentile_99_latency = parse_data(args.file)
    file_name = args.file.rsplit('.', 1)[0]
    plot_data(lambda_values, avg_latency, percentile_99_latency, file_name)