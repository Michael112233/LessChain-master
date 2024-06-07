import re
import os
import csv

def parse_log_line(line):
    # 解析 shardID 和其他感兴趣的数据
    shard_id_match = re.search(r"shardID: (\d+)", line)
    shard_id = shard_id_match.group(1) if shard_id_match else None

    # 这里添加其他需要从日志行中解析的字段
    msg_type_match = re.search(r"msgType: (\w+)", line)
    msg_type = msg_type_match.group(1) if msg_type_match else None

    # 示例：解析 states 和 blocks 的大小
    states_size_match = re.search(r"sizeof states\(bytes\): (\d+)", line)
    states_size = states_size_match.group(1) if states_size_match else None

    blocks_size_match = re.search(r"sizeof blocks\(bytes\): (\d+)", line)
    blocks_size = blocks_size_match.group(1) if blocks_size_match else None
    
    pooltx_size_match = re.search(r"sizeof poolTx\(bytes\): (\d+)", line)
    pooltx_size = pooltx_size_match.group(1) if pooltx_size_match else None
    
    sync_time_match = re.search(r"sync time: (\d+)", line)
    sync_time = sync_time_match.group(1) if sync_time_match else None

    # 返回解析的数据
    return shard_id, [shard_id, msg_type, states_size, blocks_size, pooltx_size, sync_time]

def write_logs_to_csv(file_path, logs):
    with open(file_path, 'w', newline='') as file:
        writer = csv.writer(file)
        writer.writerow(["shardID", "msgType", "sizeofStates", "sizeofBlocks", "sizeofPoolTxs", "syncTime(ms)"])  # 标题行
        for log in logs:
            writer.writerow(log)
            


def read_csv_data(file_path):
    with open(file_path, 'r') as file:
        reader = csv.reader(file)
        next(reader)  # Skip the header
        return list(reader)

def calculate_average_rows(data_sets):
    min_rows = min(len(data) for data in data_sets)
    num_columns = len(data_sets[0][0])
    average_rows = []

    for i in range(min_rows):
        row_average = []
        for j in range(num_columns):
            if j < 2:  # Skip the first two columns (shardID, msgType)
                row_average.append(data_sets[0][i][j])  # Use the first file's non-numeric data
            else:
                column_data = [float(data[i][j]) for data in data_sets]
                average_value = round(sum(column_data) / len(data_sets))  # Round to nearest integer
                row_average.append(average_value)
        average_rows.append(row_average)
    return average_rows

def write_average_to_csv(file_path, data, header):
    with open(file_path, 'w', newline='') as file:
        writer = csv.writer(file)
        writer.writerow(header)
        for row in data:
            writer.writerow(row)


def process_logs_to_csv(log_file_path, output_directory):
    # Function to process log file and output CSV files for each shard
    with open(log_file_path, 'r') as file:
        lines = file.readlines()

    shard_logs = {}
    for line in lines:
        if "msg=\"shardID:" in line:
            shard_id, parsed_data = parse_log_line(line)
            if shard_id:
                shard_logs.setdefault(shard_id, []).append(parsed_data)

    for shard_id, logs in shard_logs.items():
        os.makedirs(output_directory, exist_ok=True)
        csv_file_path = os.path.join(output_directory, f"shard_{shard_id}.csv")
        write_logs_to_csv(csv_file_path, logs)


def average_csv_data(input_directory, output_file):
    # Function to read CSV files, calculate averages, and write to a new CSV
    csv_files = [os.path.join(input_directory, file) for file in os.listdir(input_directory) if file.endswith('.csv') and file.startswith("shard")]
    # 打印检查 CSV 文件列表
    print("CSV files:", csv_files)
    all_data = [read_csv_data(file) for file in csv_files]
    # 打印检查读取的数据
    for data in all_data:
        print("Data from file:", data)
    average_data = calculate_average_rows(all_data)
    # 检查计算后的平均数据
    print("Average data:", average_data)
    
    header = ["shardID", "msgType", "Average of sizeofStates(bytes)", "Average of sizeofBlocks(bytes)", "Average of sizeofPoolTxs(bytes)", "Average of syncTime(ms)"]
    write_average_to_csv(output_file, average_data, header)



def main(syncmode):
    # 使用例子
    bandwidth = 5
    log_file_path = "experiments/results/reconfigSyncDifferBandwidth/" + syncmode + "/bandwidth" + str(bandwidth) + "MB/client.log"
    output_directory = "experiments/results/reconfigSyncDifferBandwidth/" + syncmode + "/bandwidth" + str(bandwidth) + "MB"
    # log_file_path = "experiments/results/reconfigSyncData/" + syncmode + "/client.log"
    # output_directory = "experiments/results/reconfigSyncData/" + syncmode
    process_logs_to_csv(log_file_path, output_directory)

    average_output_file = os.path.join(output_directory, "average_data.csv")
    average_csv_data(output_directory, average_output_file)

if __name__ == "__main__":
    # main("lesssync")
    main("tMPTsync")