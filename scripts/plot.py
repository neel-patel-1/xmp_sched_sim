import matplotlib.pyplot as plt
import re
import argparse
import os


def parse_data(file_path):
    with open(file_path, 'r') as file:
        data = file.read()

    # Extract data using regex
    lambda_values = [float(x) for x in re.findall(r'Lambda:(\d+\.\d+)', data)]
    avg_latency = [float(x.split()[2]) for x in re.findall(r'\d+\s+\d+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+', data)]
    percentile_99_latency = [float(x.split()[7]) for x in re.findall(r'\d+\s+\d+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+', data)]

    return lambda_values, avg_latency, percentile_99_latency

def plot_data(directory, file_suffix):
    plt.figure(figsize=(10, 5))

    for filename in os.listdir(directory):
        if filename.endswith(file_suffix + ".txt"):
            file_path = os.path.join(directory, filename)
            lambda_values, avg_latency, percentile_99_latency = parse_data(file_path)

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
            plt.subplot(1, 2, 1)
            plt.plot(filtered_lambda_values, filtered_avg_latency, marker='o', label=filename.replace(f'{file_suffix}.txt', ''))
            plt.xlabel('Load (MRPS)')
            plt.ylabel('Average Latency (µs)')
            # plt.title('Lambda vs. Average Latency')
            plt.ylim(bottom=0, top=20 * zero_load_latency)

            # Plot lambda vs. 99th percentile latency
            plt.subplot(1, 2, 2)
            plt.plot(filtered_lambda_values, filtered_percentile_99_latency, marker='o', label=filename.replace(f'{file_suffix}.txt', ''))
            plt.xlabel('Load(MRPS)')
            plt.ylabel('99th Percentile Latency (µs)')
            # plt.title('Lambda vs. 99th Percentile Latency')
            plt.ylim(bottom=0, top=20 * zero_load_latency)

    plt.subplot(1, 2, 1)
    plt.legend()
    plt.subplot(1, 2, 2)
    plt.legend()
    plt.tight_layout()
    plt.savefig('load_vs_latency.png')

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Plot Lambda vs. Latency from data files in a directory.')
    parser.add_argument('directory', type=str, help='Path to the directory containing data files')
    parser.add_argument('suffix', type=str, nargs='?', default='', help='Suffix of the files to be plotted (without .txt)')
    args = parser.parse_args()

    plot_data(args.directory, args.suffix)