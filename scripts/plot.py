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

    # Determine the zero load latency
    zero_load_latency = avg_latency[0]

    # Filter data to stop plotting once data is more than 100 times the zero load latency
    filtered_lambda_values = []
    filtered_avg_latency = []
    filtered_percentile_99_latency = []

    for i in range(len(lambda_values)):
        if avg_latency[i] <= 100 * zero_load_latency:
            filtered_lambda_values.append(lambda_values[i])
            filtered_avg_latency.append(avg_latency[i])
            filtered_percentile_99_latency.append(percentile_99_latency[i])
        else:
            break
    # add the last point to the filtered data
    filtered_lambda_values.append(lambda_values[i])
    filtered_avg_latency.append(avg_latency[i])
    filtered_percentile_99_latency.append(percentile_99_latency[i])


    # Plot lambda vs. average latency
    plt.figure(figsize=(10, 5))
    plt.subplot(1, 2, 1)
    plt.plot(filtered_lambda_values, filtered_avg_latency, marker='o')
    plt.xlabel('Lambda')
    plt.ylabel('Average Latency')
    plt.title('Lambda vs. Average Latency')
    plt.ylim(bottom=0, top=20 * zero_load_latency)

    # Plot lambda vs. 99th percentile latency
    plt.subplot(1, 2, 2)
    plt.plot(filtered_lambda_values, filtered_percentile_99_latency, marker='o')
    plt.xlabel('Lambda')
    plt.ylabel('99th Percentile Latency')
    plt.title('Lambda vs. 99th Percentile Latency')
    plt.ylim(bottom=0, top=20 * zero_load_latency)

    plt.tight_layout()
    plt.savefig(name + '.png')

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Plot Lambda vs. Latency from data file.')
    parser.add_argument('file', type=str, help='Path to the data file')
    args = parser.parse_args()

    lambda_values, avg_latency, percentile_99_latency = parse_data(args.file)
    file_name = args.file.rsplit('.', 1)[0]
    plot_data(lambda_values, avg_latency, percentile_99_latency, file_name)