import matplotlib.pyplot as plt
import numpy as np
import os
import re
import pandas as pd
import seaborn as sns

width = 6.5
height = 4
default_palette1 = ['#3B4970', '#697BAE', '#D4E4F8', '#F8EDE8', '#8C3A4B'] # ganyu
default_palette = ['#2F2321', '#AA4F23', '#FED875', '#F8EBDC', '#6C627A'] # zhongli

def getStorePath():
    # 获取当前Python文件的绝对路径
    current_file_path = os.path.abspath(__file__)
    # 获取当前Python文件所在的目录
    current_directory = os.path.dirname(current_file_path)
    # # 定义图片的保存路径
    # image_path = os.path.join(current_directory, 'figs')
    return current_directory

def motivation_security():
    '''
    用于在motivation中说明分片安全性不足的图
    '''
    plt.rcParams['pdf.fonttype'] = 42
    plt.rcParams['ps.fonttype'] = 42
    plt.rcParams['figure.figsize'] = (10, 4)

    ## 随着分片数量增加，攻击者令单个分片失效所需的力量逐渐减少
    plt.subplot(1,2,1)
    plt.text(0.42, -0.3, '(a)', transform=plt.gca().transAxes, size=18)
    ### 假设分片内运行PBFT协议
    shardNums = np.array(range(1,9))
    powers_to_hinder = 1/3 / shardNums
    powers_to_control = 2/3 / shardNums

    # plt.fill_between(shardNums, powers_to_control, color=default_palette1[4], alpha=0.4)
    # plt.fill_between(shardNums, powers_to_hinder, color=default_palette1[1], alpha=0.4)
    plt.plot(shardNums, powers_to_control, label="Level2: manipulate\nconsensus",color=default_palette[4], linewidth=3)
    plt.plot(shardNums, powers_to_hinder, label="Level1: hinder\nconsensus", color=default_palette[1], linewidth=3)

    plt.xticks(fontsize=16)
    plt.yticks([0.1,0.2,0.3,0.4,0.5,0.6],['10%','20%','30%','40%','50%','60%'],fontsize=16)
    plt.xlabel('# of shards', fontsize=18)
    plt.ylabel('votes to attack one shard     ', fontsize=18)
    plt.legend(fontsize=15, loc='center', bbox_to_anchor=(0.58,0.75))
    plt.tight_layout()

    ## 随着交易注入，失效分片导致的累积失效交易数量逐渐增多
    plt.subplot(1,2,2)
    plt.text(0.5, -0.3, '(b)', transform=plt.gca().transAxes, size=18)
    shardNums = np.array([1,2,4,8])
    data = np.array([
        [500,1000,1500,2000,2500,3000,3500,4000,4500,5000],
        [369,743,1108,1456,1813,2262,2664,3017,3387,3736],
        [207,469,719,918,1118,1512,1797,2017,2236,2463],
        [118,297,452,563,683,1047,1270,1400,1537,1685]
        ])
    for i in range(len(shardNums)):
        shardData = data[i]
        plt.fill_between(data[0], data[i], color=default_palette[i], alpha=0.4)
        plt.plot(data[0], data[i], label="ShardNum="+str(shardNums[i]), color=default_palette[i], alpha=1, linewidth=2)
    plt.xticks(fontsize=16)
    plt.yticks(fontsize=16)
    plt.xlabel('# of all txs', fontsize=18)
    plt.ylabel('# of directly affected txs    ', fontsize=18)
    plt.legend(fontsize=14, loc='center', bbox_to_anchor=(0.27,0.77))
    plt.tight_layout()

    plt.savefig('experiments/figs/motivation_security.png')
    plt.savefig('experiments/figs/motivation_security.pdf')
    plt.show()


def TPS(dataPath):
    plt.rcParams['pdf.fonttype'] = 42
    plt.rcParams['ps.fonttype'] = 42
    plt.rcParams['figure.figsize'] = (width, height)
    plt.grid('on', alpha=0.5, axis='y')
    shards = [2,8,16,24,32]
    # tps = [236,551,1236,1491,2365,2899]
    dataPath = os.path.join(getStorePath(), dataPath)
    tps,_,_,_ = getData(shards,dataPath)
    
    # colors = ['#26697d', '#82c6d5', '#bceed4', '#f2e2a9', '#fffff3']
    bar = plt.bar(x=shards, height=tps, width=3, alpha=1, color=default_palette)
    plt.bar_label(bar, labels=tps, fontsize=16, padding=10)
    plt.plot(shards, tps, '.-', color='tab:red', markersize=12)
    
    plt.xticks(shards, fontsize=16)
    plt.yticks(fontsize=16)
    # plt.yticks([0,2000,4000,6000,8000,10000],['0','2k','4k','6k','8k','10k'],fontsize=16)
    # plt.yticks([0,1000,2000,3000,4000,5000], ['0', '1k', '2k', '3k', '4k', '5k'], fontsize=16)
    plt.xlabel('Number of shards', fontsize=18)
    plt.ylabel('Throughput (TX/s)', fontsize=18)
    ax = plt.gca()
    ax.spines['right'].set_visible(False)
    ax.spines['top'].set_visible(False)
    ax.spines['left'].set_visible(False)
    plt.tick_params(bottom=False, top=False, left=False, right=False)
    plt.tight_layout()
    # plt.savefig('figs/throughput.pdf')
    storePath = os.path.join(getStorePath(), 'figs', 'throughput.png')
    # 确保目录存在
    if not os.path.exists(os.path.dirname(storePath)):
        os.makedirs(os.path.dirname(storePath))
    plt.savefig(storePath)
    storePath = os.path.join(getStorePath(), 'figs', 'throughput.pdf')
    plt.savefig(storePath)
    plt.show()

def getLatDetail(group, dataPath):
    path = os.path.join(getStorePath(), dataPath)
    
    files = []
    if "tps" in path:
        for num in group:
            files.append(path + 'shard' + str(num) + 'run1.csv')
    elif "injectSpeed" in path:
        for num in group:
            files.append(path + 'inject' + str(num) + '.csv')
    
    lats = []
    for filename in files:
        with open(filename) as f:
            content = f.read()
        lat = content.split(',')
        lat = list(map(int,lat))
        lats = lats + [lat]
    # print(len(lats))
    # print(lats[0][:10])
    return lats

# 数据来源：experiments/results/tps
def Latency(dataPath):
    plt.rcParams['pdf.fonttype'] = 42
    plt.rcParams['ps.fonttype'] = 42
    plt.rcParams['figure.figsize'] = (width, height)
    # plt.style.use('_mpl-gallery')
    shards = [2,8,16,24,32]
    
    lat_detail = getLatDetail(shards, dataPath)
    # palette = ['#26697d', '#82c6d5', '#bceed4', '#f2e2a9', '#fffff3']
    # palette1 = ['#512324', '#d9de5e', '#888201', '#46a8a8', '#a8c7cb']
    # zhongli
    palette = default_palette
    x = []
    y = []
    for i in range(len(lat_detail)):
        x = x + [shards[i]] * len(lat_detail[i])
        y = y + lat_detail[i]
    print(len(x),len(y))
    sns.boxplot(x=x,y=y, width=0.5, palette=palette)

    dataPath = os.path.join(getStorePath(), dataPath)
    _,latency,_,_ = getData(shards,dataPath)
    plt.plot([0,1,2,3,4], latency, '.-', color='tab:red', markersize=12, label='Average Latency')
    plt.xticks(fontsize=16)
    plt.yticks(fontsize=16)
    # plt.ylim(0,130)
    plt.xlabel('Number of shards', fontsize=18)
    plt.ylabel('Latency (s)', fontsize=18)
    plt.legend(fontsize=16, loc='center', bbox_to_anchor=(0.27,0.9))
    plt.tight_layout()
    # plt.savefig('figs/latency.pdf')
    storePath = os.path.join(getStorePath(), 'figs', 'latency.png')
    # 确保目录存在
    if not os.path.exists(os.path.dirname(storePath)):
        os.makedirs(os.path.dirname(storePath))
    plt.savefig(storePath)
    storePath = os.path.join(getStorePath(), 'figs', 'latency.pdf')
    plt.savefig(storePath)
    plt.show()

def Workload4shard(dataPath):
    plt.rcParams['pdf.fonttype'] = 42
    plt.rcParams['ps.fonttype'] = 42
    plt.rcParams['figure.figsize'] = (width, height)
    labels = ['S = 8', 'S = 16', 'S = 32']
    colors = [default_palette[1], default_palette[2], default_palette[4]]
    shards = [8,16,32]
    shardIndexs = []
    for shard in shards:
        shardIndexs.append([i for i in range(shard)])
    
    dataPath = os.path.join(getStorePath(), dataPath)
    _,_,_,workloadStrs = getData(shards, dataPath)
    workloads = []
    for workload in workloadStrs:
        workload = [int(num) for num in workload.split()]
        workloads.append(workload)
    # print(workloads)
    print(len(shardIndexs), len(workloads), len(labels), len(colors), len(shards))
    for i in range(len(shards)):
        print(shardIndexs[i], workloads[i])
        plt.plot(shardIndexs[i], workloads[i], label=labels[i], color=colors[i])

    
    plt.xticks([],fontsize=16)
    # plt.yticks([0,5e4,10e4,15e4,20e4],[0,'5w','10w','15w','20w'],fontsize=16)
    plt.yticks([0,1e5,2e5,3e5,4e5],[0,'10w','20w','30w','40w'],fontsize=16)
    plt.xlabel('\nSequantial IDs of shards', fontsize=18)
    plt.ylabel('Workload (# of TX)', fontsize=18)
    plt.legend(fontsize=16,loc='center', bbox_to_anchor=(0.7,0.7))
    plt.tight_layout()
    # plt.savefig('figs/workload.pdf')
    storePath = os.path.join(getStorePath(), 'figs', 'workload.png')
    # 确保目录存在
    if not os.path.exists(os.path.dirname(storePath)):
        os.makedirs(os.path.dirname(storePath))
    plt.savefig(storePath)
    storePath = os.path.join(getStorePath(), 'figs', 'workload.pdf')
    plt.savefig(storePath)
    plt.show()


def getData(group, path):
    tpss = []
    lats = []
    rollbackRates = []
    workloads = []
    
    files = []
    if "tps" in path or "rollback" in path or "reconfig" in path or 'workload' in path or 'confirmHeight' in path:
        for num in group:
            files.append(path + 'shard' + str(num) + 'run1.log')
    elif "injectSpeed" in path:
        for num in group:
            files.append(path + 'inject' + str(num) + '.log')
        
    for filename in files:
        with open(filename) as f:
            content = f.read()
        pat = re.compile(r'GetThroughtPutAndLatency.*caller')
        text = pat.findall(content)[0]
        pat = re.compile(r'thrput=([0-9]*\.?[0-9]+) avlatency=([0-9]*\.?[0-9]+) rollbackRate=([0-9]*\.?[0-9]+) overloads="\[(.*)\]"')
        res = pat.findall(text)

        # print(text)
        # print(res)
        tps = int(float(res[0][0]))
        latency = float(res[0][1])
        rollbackRate = float(res[0][2])
        workload = res[0][3]
        # print(tps,latency,rollbackRate,workload)
        lats = lats + [latency]
        tpss = tpss + [tps]
        rollbackRates = rollbackRates + [rollbackRate]
        workloads = workloads + [workload]
    # print(lats)
    return tpss,lats,rollbackRates,workloads

def TPS_inject(dataPath1):
    plt.rcParams['pdf.fonttype'] = 42
    plt.rcParams['ps.fonttype'] = 42
    plt.rcParams['figure.figsize'] = (width, height)
    speeds = [[2000,4000,8000,10000,12000,16000], [500,1000,2000,2500,3000,4000]]
    xticks = [['2k', '4k', '8k', '10k', '12k', '16k'], ['0.5k', '1k', '2k', '2.5k', '3k', '4k']]
    shards = [32]
    for i in range(len(shards)):
        dataPath = os.path.join(getStorePath(), dataPath1, 'shard'+str(shards[i])+'/')
        tps,_,_,_ = getData(speeds[i],dataPath)
        
        # colors = ['#26697d', '#82c6d5', '#bceed4', '#f2e2a9', '#fffff3']
        bar = plt.bar(x=speeds[i], height=tps, width=shards[i]*35, alpha=1, color=default_palette[4])
        plt.bar_label(bar, labels=tps, fontsize=16, padding=10)
        plt.plot(speeds[i], tps, '.-', color='tab:red', markersize=12)
        
        plt.xticks(speeds[i], xticks[i], fontsize=16)
        plt.yticks(fontsize=16)
        # plt.yticks([0,2000,4000,6000,8000,10000],['0','2k','4k','6k','8k','10k'],fontsize=16)
        # plt.yticks([0,1000,2000,3000,4000,5000], ['0', '1k', '2k', '3k', '4k', '5k'], fontsize=16)
        plt.xlabel('Inject speed (TX/s)', fontsize=18)
        plt.ylabel('Throughput (TX/s)', fontsize=18)

        plt.tight_layout()
        # plt.savefig('figs/throughput.pdf')
        storePath = os.path.join(getStorePath(), 'figs', 'injectSpeed_s' + str(shards[i]) + '.png')
        # 确保目录存在
        if not os.path.exists(os.path.dirname(storePath)):
            os.makedirs(os.path.dirname(storePath))
        plt.savefig(storePath)
        storePath = os.path.join(getStorePath(), 'figs', 'injectSpeed_s' + str(shards[i]) + '.pdf')
        plt.savefig(storePath)
        plt.show()




def TPS_inject1(dataPath1):
    plt.rcParams['pdf.fonttype'] = 42
    plt.rcParams['ps.fonttype'] = 42
    plt.rcParams['figure.figsize'] = (width, height)
    speeds = [[2000, 4000, 8000, 10000, 12000, 16000], [500, 1000, 2000, 2500, 3000, 4000]]
    xticks = [['0.25', '0.5', '1', '1.25', '1.5', '2'], ['0.25', '0.5', '1', '1.25', '1.5', '2']]
    shards = [32,8]
    colors = default_palette
    labels = ['S = 32', 'S = 8']
    
    for i in range(len(shards)):
        dataPath = os.path.join(getStorePath(), dataPath1, 'shard'+str(shards[i])+'/')
        tps, _, _, _ = getData(speeds[i], dataPath)
        
        # 画线和填充山坡
        plt.plot(speeds[0], tps, '.-', color=default_palette[i+2], linewidth=4, markersize=12, label=labels[i])  
        # plt.fill_between(speeds[i], tps, color=default_palette[2], alpha=0.8)

        # 在每个点上标注数值
        for x, y in zip(speeds[0], tps):
            plt.text(x, y, f'{y}', color='black', ha='center', va='bottom', fontsize=16)

        
    # 设置 x 和 y 轴标签和字体大小
    plt.xticks(speeds[0], xticks[0], fontsize=16)
    plt.yticks(fontsize=16)
    plt.xlabel('Inject Speed / Standard Inject Speed', fontsize=18)
    plt.ylabel('Throughput (TX/s)', fontsize=18)

    ax = plt.gca()
    ax.spines['top'].set_visible(False)
    ax.spines['right'].set_visible(False)
    ax.spines['left'].set_visible(False)
    # ax.spines['bottom'].set_visible(False)
    plt.tick_params(top=False, left=False, right=False)
    plt.legend(fontsize=16, loc='best')
    
    # 确保存储路径存在
    storePath = os.path.join(getStorePath(), 'figs', 'injectSpeed')
    os.makedirs(os.path.dirname(storePath), exist_ok=True)
    
    plt.tight_layout()
    # 保存图片
    plt.savefig(f'{storePath}.png')
    plt.savefig(f'{storePath}.pdf')
  
    # 显示图表
    plt.show()



    
def Latency_inject(dataPath1):
    plt.rcParams['pdf.fonttype'] = 42
    plt.rcParams['ps.fonttype'] = 42
    plt.rcParams['figure.figsize'] = (width, height)
    # plt.style.use('_mpl-gallery')
    shards = [32,8]
    speeds = [[2000,4000,8000,10000,12000,16000], [500,1000,2000,2500,3000,4000]]
    
    palette = ['#26697d', '#82c6d5', '#bceed4', '#f2e2a9', '#fffff3', '#ce7ed4']
    # palette1 = ['#512324', '#d9de5e', '#888201', '#46a8a8', '#a8c7cb']
    
    for i in range(len(shards)):
        
        lat_detail = getLatDetail(speeds[i], os.path.join(dataPath1, 'shard'+str(shards[i])+'/'))
        x = []
        y = []
        for j in range(len(lat_detail)):
            x = x + [speeds[i][j]] * len(lat_detail[j])
            y = y + lat_detail[j]
        print(len(x),len(y))
        sns.boxplot(x=x,y=y, width=0.5, palette=palette)

        dataPath = os.path.join(getStorePath(), dataPath1, 'shard'+str(shards[i])+'/')
        _,latency,_,_ = getData(speeds[i],dataPath)
        plt.plot([0,1,2,3,4,5], latency, '.-', color='tab:red', markersize=12, label='Average Latency')
        plt.xticks(fontsize=16)
        plt.yticks(fontsize=16)
        # plt.ylim(0,130)
        plt.xlabel('Inject speed (TX/s)', fontsize=18)
        plt.ylabel('Latency (s)', fontsize=18)
        plt.legend(fontsize=16, loc='center', bbox_to_anchor=(0.27,0.9))
        plt.tight_layout()
        # plt.savefig('figs/latency.pdf')
        storePath = os.path.join(getStorePath(), 'figs', 'injectSpeed_latency_s' + str(shards[i]) + '.png')
        # 确保目录存在
        if not os.path.exists(os.path.dirname(storePath)):
            os.makedirs(os.path.dirname(storePath))
        plt.savefig(storePath)
        storePath = os.path.join(getStorePath(), 'figs', 'injectSpeed_latency_s' + str(shards[i]) + '.pdf')
        plt.savefig(storePath)
        plt.show()

# 数据来源：experiments/results/
def RollBackRate(dataPath):
    plt.rcParams['pdf.fonttype'] = 42
    plt.rcParams['ps.fonttype'] = 42
    plt.rcParams['figure.figsize'] = (width, height)
    plt.grid('on', alpha=0.5)
    shards = [8,16,32]
    rollback_height = [6,9,12,18,2000]
    xticks = [2,4,6,8,10]
    labels = ['S = 8', 'S = 16', 'S = 32']
    # colors = ['#82c6d5', '#f2e2a9', '#fffff3']
    colors = [default_palette[1], default_palette[2], default_palette[4]]
    linestyles = ['>-', '.-', 's-']
    markersizes = [7,12,6]
    
    for i in range(len(shards)):
        rates = []
        for j in range(len(rollback_height)):
            path = os.path.join(getStorePath(), dataPath+'height'+str(rollback_height[j])+'/')
            cur_shard = [shards[i]]
            tps,latency,rate,_ = getData(cur_shard, path)
            # rates.append(late)
            rates.append(rate)
        plt.plot(xticks, rates, linestyles[i], label=labels[i], color=colors[i], markersize=markersizes[i])
    plt.xticks(xticks, ['6', '9', '12', '18', 'infinite'], fontsize=16)
    plt.yticks(fontsize=16)
    plt.xlabel('Timeout time', fontsize=18)
    plt.ylabel('Prop. of TXs rollback', fontsize=18)
    # plt.ylabel('Latency (s)', fontsize=18)
    # plt.ylabel('Throughput (TX/s)', fontsize=18)
    legend = plt.legend(fontsize=16, loc='center',bbox_to_anchor=(0.7,0.45))
    legend.set_draggable(True)
    plt.tight_layout()
    # plt.savefig('figs/rollbackRate.pdf')
    storePath = os.path.join(getStorePath(), 'figs/rollbackRate.png')
    # storePath = os.path.join(getStorePath(), 'figs/rollbackRate_latency.png')
    plt.savefig(storePath)
    storePath = os.path.join(getStorePath(), 'figs/rollbackRate.pdf')
    plt.savefig(storePath)
    plt.show()
    
# 数据来源：experiments/results/
def RollBackRateV2(dataPath):
    plt.rcParams['pdf.fonttype'] = 42
    plt.rcParams['ps.fonttype'] = 42
    plt.rcParams['figure.figsize'] = (width, height)
    plt.grid('on', alpha=0.5)
    shards = [8,16,32]
    rollback_height = [6,9,12,18,2000]
    labels = ['H = 6', 'H = 9', 'H = 12', 'H = 18', 'H = infinite']
    # colors = ['#82c6d5', '#f2e2a9', '#fffff3']
    colors = ['#2878b5', '#9ac9db', '#f8ac8c', '#c82423', '#ff8884']
    linestyles = ['x-', '.-', 's-', '>-', '^-']
    markersizes = [7,12,6,6,6]
    
    for i in range(len(rollback_height)):
        path = os.path.join(getStorePath(), dataPath+'height'+str(rollback_height[i])+'/')
        _,_,rates,_ = getData(shards, path)
        plt.plot(shards, rates, linestyles[i], label=labels[i], color=colors[i], markersize=markersizes[i])
    plt.xticks(shards, fontsize=16)
    plt.yticks(fontsize=16)
    plt.xlabel('# of shards', fontsize=18)
    plt.ylabel('Prop. of TXs rollback', fontsize=18)
    legend = plt.legend(fontsize=16, loc='center',bbox_to_anchor=(0.5,0.45))
    legend.set_draggable(True)
    plt.tight_layout()
    # plt.savefig('figs/rollbackRate.pdf')
    storePath = os.path.join(getStorePath(), 'figs/rollbackRateV2.png')
    plt.savefig(storePath)
    storePath = os.path.join(getStorePath(), 'figs/rollbackRateV2.pdf')
    plt.savefig(storePath)
    plt.show()


def Reconfig(dataPath):  
    plt.rcParams['pdf.fonttype'] = 42
    plt.rcParams['ps.fonttype'] = 42
    plt.rcParams['figure.figsize'] = (width, height)
    
    shards = [8,32]
    reconfig_height = [4,6,8,2000]
    x_ticks = [2,4,6,8]
    labels = ['S = 8','S = 32']
    colors = default_palette
    linestyles = ['.-', 's-', '>--', 'x-']
    markersizes = [16,6,8,8]
    
    for i in range(len(shards)):
        fig, ax1 = plt.subplots()  # 创建原始的figure和axis
        ax2 = ax1.twinx()  # 创建第二个y轴
        tpss = []
        delays = []
        for j in range(len(reconfig_height)):
            path = os.path.join(getStorePath(), dataPath+'height'+str(reconfig_height[j])+'/')
            cur_shards = [shards[i]]
            tps, delay, _, _ = getData(cur_shards, path)
            tpss.append(tps)
            delays.append(delay)
            
        ax1.plot(x_ticks, tpss, linestyles[0], label="S = "+str(shards[i])+ " Throughput", color=colors[i], markersize=markersizes[0]) 
        ax2.plot(x_ticks, delays, linestyles[2], label="S = "+str(shards[i])+ " Latency", color=colors[i], markersize=markersizes[2]) 
    
        ax1.set_xticks([2,4,6,8])
        ax1.set_xticklabels(['4', '6', '8', 'infinite'])
        ax1.tick_params(axis='both', labelsize=16)
        ax1.set_xlabel('Reconfiguration interval', fontsize=18)
        ax1.set_ylabel('Throughput (TX/s)', fontsize=18)
        
        ax2.set_ylabel('Latency (s)', fontsize=18)
        ax2.tick_params(axis='y', labelsize=16)

        # 合并两个轴的图例
        lines, labels = ax1.get_legend_handles_labels()
        lines2, labels2 = ax2.get_legend_handles_labels()
        legend = ax2.legend(lines + lines2, labels + labels2, fontsize=14, loc='center',bbox_to_anchor=(0.52,0.17))
        legend.set_draggable(True)
        
        plt.tight_layout()
    
        storePath = os.path.join(getStorePath(), 'figs/reconfig_shard'+str(shards[i])+'.png')
        plt.savefig(storePath)
        storePath = os.path.join(getStorePath(), 'figs/reconfig_shard'+str(shards[i])+'.pdf')
        plt.savefig(storePath)
        plt.show()

def Reconfig1(dataPath):  
    plt.rcParams['pdf.fonttype'] = 42
    plt.rcParams['ps.fonttype'] = 42
    plt.rcParams['figure.figsize'] = (width, height)
    
    shards = [32,8]
    reconfig_height = [4,6,8,2000]
    x_ticks = [2,4,6,8]
    labels = ['S = 32','S = 8']
    colors = default_palette
    linestyles = ['.-', 's-', '>--', 'x-']
    markersizes = [16,6,8,8]
    
    for i in range(1):
        fig, ax1 = plt.subplots()  # 创建原始的figure和axis
        ax2 = ax1.twinx()  # 创建第二个y轴
        tpss = []
        delays = []
        for j in range(len(reconfig_height)):
            path = os.path.join(getStorePath(), dataPath+'height'+str(reconfig_height[j])+'/')
            cur_shards = [shards[i]]
            tps, delay, _, _ = getData(cur_shards, path)
            tpss.append(tps[0])
            delays.append(delay[0])

        print(tpss)
        print(delays)
        # 填充Throughput的山坡
        ax1.fill_between(x_ticks, tpss, color=colors[0], alpha=0.3)
        # 填充Latency的山坡
        ax2.fill_between(x_ticks, delays, color=colors[1], alpha=0.3)

        ax1.plot(x_ticks, tpss, linestyles[0], label="S = "+str(shards[i])+ " Throughput", color=colors[0], markersize=markersizes[0]) 
        ax2.plot(x_ticks, delays, linestyles[2], label="S = "+str(shards[i])+ " Latency", color=colors[1], markersize=markersizes[2]) 
    
        ax1.set_xticks([2,4,6,8])
        ax1.set_xticklabels(['4', '6', '8', 'infinite'])
        ax1.tick_params(axis='both', labelsize=16)
        ax1.set_xlabel('Reconfiguration interval', fontsize=18)
        ax1.set_ylabel('Throughput (TX/s)', fontsize=18)
        
        ax2.set_ylabel('Latency (s)', fontsize=18)
        ax2.tick_params(axis='y', labelsize=16)

        # 合并两个轴的图例
        lines, labels = ax1.get_legend_handles_labels()
        lines2, labels2 = ax2.get_legend_handles_labels()
        legend = ax2.legend(lines + lines2, labels + labels2, fontsize=14, loc='center',bbox_to_anchor=(0.52,0.17))
        legend.set_draggable(True)
        
        plt.tight_layout()
    
        storePath = os.path.join(getStorePath(), 'figs/reconfig_shard'+str(shards[i])+'.png')
        plt.savefig(storePath)
        storePath = os.path.join(getStorePath(), 'figs/reconfig_shard'+str(shards[i])+'.pdf')
        plt.savefig(storePath)
        plt.show()


def ConfirmHeight(dataPath):
    plt.rcParams['pdf.fonttype'] = 42
    plt.rcParams['ps.fonttype'] = 42
    plt.rcParams['figure.figsize'] = (width, height)
    
    shards = [8,16,32]
    ConfirmHeight = [0,2,4,6]
    labels = ['S = 8', 'S = 16', 'S = 32']
    colors = ['tab:red', 'tab:orange', 'tab:blue']
    linestyles = ['.-', 's-', '*-', 'x-']
    markersizes = [8,6,8,8]

    fig, ax1 = plt.subplots()  # 创建原始的figure和axis
    # ax2 = ax1.twinx()  # 创建第二个y轴
    
    for i in range(len(shards)):
        tpss = []
        delays = []
        for j in range(len(ConfirmHeight)):
            path = os.path.join(getStorePath(), dataPath+'height'+str(ConfirmHeight[j])+'/')
            cur_shards = [shards[i]]
            tps, delay, _, _ = getData(cur_shards, path)
            tpss.append(tps)
            delays.append(delay)
            
        ax1.plot(ConfirmHeight, tpss, linestyles[i], label=labels[i], color=colors[i], markersize=markersizes[i])
        # ax2.plot(ConfirmHeight, delays, linestyles[2], label=labels[i] + " Latency", color=colors[i], markersize=markersizes[2])
    
    ax1.set_xticks(ConfirmHeight)
    ax1.set_xticklabels(['1', '3', '5', '7'])
    ax1.tick_params(axis='both', labelsize=16)
    ax1.set_xlabel('Height to confirm', fontsize=18)
    ax1.set_ylabel('Throughput (TX/s)', fontsize=18)
    # ax1.set_ylabel('Latency (s)', fontsize=18)
    

    legend = ax1.legend(fontsize=14, loc='center',bbox_to_anchor=(0.5,0.55))
    legend.set_draggable(True)
    
    plt.tight_layout()
    
    storePath = os.path.join(getStorePath(), 'figs/confirmHeight_tps.png')
    plt.savefig(storePath)
    storePath = os.path.join(getStorePath(), 'figs/confirmHeight_tps.pdf')
    plt.savefig(storePath)
    plt.show()


def reconfig_security_S_n():
    plt.rcParams['pdf.fonttype'] = 42
    plt.rcParams['ps.fonttype'] = 42
    # 获取当前文件的完整路径
    current_file_path = os.path.realpath(__file__)
    # 获取当前文件所在的目录
    current_dir = os.path.dirname(current_file_path)
    security = pd.read_csv(current_dir + '/SECURITY-malicious_rate=0.20-tolerance=0.33.csv', index_col=0)
    # print(security)
    # 去掉分片数为2的行
    security = security[1:]

    security = security.T
    # print(security)
    columns = security.columns.tolist()
    rows = [] # 失败概率
    indices = [] # 分片大小
    for c in columns:
        row = security[(security[c]<1e-6) & (security[c]>0)]
        index = row.index.tolist()
        index = list(map(int,index))
        rows = rows + [row.iloc[0][c]]
        indices = indices + [index[0]]
    print(rows)
    print(indices)
    shards = list(map(int,columns))

    colors = default_palette

    # 子图 1
    fig = plt.figure()
    ax1 = fig.add_subplot(111)
    ax1.plot(shards, rows, '.-', color=colors[0], label='Failure probability', markersize=10)
    ax1.set_xlabel('Number of Shards', fontsize=18)
    ax1.set_ylabel('Failure probability', fontsize=18)
    ax1.set_xticklabels(list(map(int,ax1.get_xticks())), fontsize=16)
    # ax1.yaxis.get_major_formatter().set_powerlimits((1, 2))  # 将坐标轴的base number设置为一位，指数不超过两位
    # print(ax1.get_yticks())

    # ax1.set_yticklabels([0,2,4,6,8,10], fontsize=16)
    # ax1.text(-5,1e-6,'1e-7',fontsize=16)

    
    ax1.set_ylim(-1e-7,1.7e-6)
    ax1.set_yticklabels([0.0,0.2,0.4,0.6,0.8,1.0,1.2,1.4,1.6], fontsize=16)
    ax1.text(-5,1.75e-6,'1e-6',fontsize=16)
    

    # 子图 2
    ax2 = plt.twinx()
    ax2.plot(shards, indices, 's--', color=colors[1], label='# of nodes in each shard')
    ax2.set_ylabel('# of nodes in each committee', fontsize=18)
    ax2.set_yticklabels(list(map(int,ax2.get_yticks())), fontsize=16)

    # 直线
    ax1.axhline(1e-6, linestyle = '--', color=colors[0], linewidth=2)
    ax1.text(70,0.85e-6,'Theoretic upper bound', fontsize=16, color=colors[0])
    # ax1.arrow(70,0.8e-6,70,0e-6,width=1e-7,shape='full')
    # ax1.arrow(60, 0.8e-6, 0, 0.1e-6) # x,y,dx,dy

    legend = fig.legend(loc='center', bbox_to_anchor=(0.55,0.1), bbox_transform=ax1.transAxes, fontsize=16)
    legend.set_draggable(True)
    
    plt.tight_layout()
    plt.savefig(current_dir + '/figs/security_S_n.pdf')
    plt.savefig(current_dir + '/figs/security_S_n.png')
    plt.show()
    
    # for i in range(len(rows)):
    #     plt.plot(shards, rows[i], label=labels[i])
    # plt.legend(fontsize=17, loc='center', bbox_to_anchor=(0.3,0.4))
    # plt.xticks(fontsize=18)
    # plt.yticks(fontsize=18)
    # plt.xlabel('Number of shards', fontsize=20)
    # plt.ylabel('Security', fontsize=20)
    # plt.tight_layout()
    # # plt.savefig('security.pdf')
    # # plt.savefig('security.png')
    # plt.show() 


def reconfig_datasize(dataPath):
    plt.rcParams['pdf.fonttype'] = 42
    plt.rcParams['ps.fonttype'] = 42
    plt.rcParams['figure.figsize'] = (width, height)
    
    sync_methods = ['fullsync', 'fastsync', 'tMPTsync', 'lesssync']
    labels = ['Full sync (Ethereum)', 'Fast sync (Ethereum)', 'tMPT [14]', 'LessChain (ours)']
    colors = ['#2F2321', '#AA4F23', '#FED875', '#6C627A', '#F8EBDC']
    # patterns = ['/', '\\', '|']  # sizeofStates, sizeofBlocks, sizeofPoolTxs 对应的花纹

    # 读取和处理每种同步方法的数据
    for i, method in enumerate(sync_methods):
        file_path = os.path.join(getStorePath(), dataPath + method + '/average_data.csv')
        data = pd.read_csv(file_path)

        sizeofStates = data['Average of sizeofStates(bytes)'] / 1000
        sizeofBlocks = data['Average of sizeofBlocks(bytes)'] / 1000
        sizeofPoolTxs = data['Average of sizeofPoolTxs(bytes)'] / 1000
        totalSize = sizeofStates + sizeofBlocks + sizeofPoolTxs

        row_numbers = [x + i * 0.2 for x in range(1, len(data) + 1)]  # 调整每组数据的位置

        plt.bar(row_numbers, totalSize, width=0.2, color=colors[i], label=labels[i])
        # plt.bar(row_numbers, sizeofBlocks, width=0.2, color=colors[i], hatch=patterns[1], bottom=sizeofStates, label='sizeofBlocks' if i==0 else "")
        # plt.bar(row_numbers, sizeofPoolTxs, width=0.2, color=colors[i], hatch=patterns[2], bottom=sizeofStates + sizeofBlocks, label='sizeofPoolTxs' if i==0 else "")

    plt.xticks([r + 0.3 for r in range(1, len(data) + 1)], [r for r in range(1, len(data) + 1)], fontsize=16)
    plt.yticks(fontsize=16)
    plt.xlabel('Reconfiguration times', fontsize=18)
    plt.ylabel('Sync Datasize (KB)', fontsize=18)
    legend = plt.legend(fontsize=14, loc='best')
    legend.set_draggable(True)
    plt.tight_layout()
    
    storePath = os.path.join(getStorePath(), 'figs', 'reconfig_datasize.png')
    # 确保目录存在
    if not os.path.exists(os.path.dirname(storePath)):
        os.makedirs(os.path.dirname(storePath))
    plt.savefig(storePath)
    storePath = os.path.join(getStorePath(), 'figs', 'reconfig_datasize.pdf')
    plt.savefig(storePath)
    
    plt.show()

def reconfig_synctime(dataPath):
    plt.rcParams['pdf.fonttype'] = 42
    plt.rcParams['ps.fonttype'] = 42
    plt.rcParams['figure.figsize'] = (width, height)
    
    sync_methods = ['fullsync', 'fastsync', 'tMPTsync', 'lesssync']
    labels = ['Full sync (Ethereum)', 'Fast sync (Ethereum)', 'tMPT [14]', 'LessChain (ours)']
    colors = ['#2F2321', '#AA4F23', '#FED875', '#6C627A', '#F8EBDC']
    # patterns = ['/', '\\', '|']  # sizeofStates, sizeofBlocks, sizeofPoolTxs 对应的花纹

    # 读取和处理每种同步方法的数据
    for i, method in enumerate(sync_methods):
        file_path = os.path.join(getStorePath(), dataPath + method + '/average_data.csv')
        data = pd.read_csv(file_path)

        syncTime = data['Average of syncTime(ms)']

        row_numbers = [x + i * 0.2 for x in range(1, len(data) + 1)]  # 调整每组数据的位置

        plt.bar(row_numbers, syncTime, width=0.2, color=colors[i], label=labels[i])
        # plt.bar(row_numbers, sizeofBlocks, width=0.2, color=colors[i], hatch=patterns[1], bottom=sizeofStates, label='sizeofBlocks' if i==0 else "")
        # plt.bar(row_numbers, sizeofPoolTxs, width=0.2, color=colors[i], hatch=patterns[2], bottom=sizeofStates + sizeofBlocks, label='sizeofPoolTxs' if i==0 else "")

    plt.xticks([r + 0.3 for r in range(1, len(data) + 1)], [r for r in range(1, len(data) + 1)], fontsize=16)
    plt.yticks(fontsize=16)
    plt.xlabel('Reconfiguration times', fontsize=18)
    plt.ylabel('Sync Time (ms)', fontsize=18)
    legend = plt.legend(fontsize=14, loc='best')
    legend.set_draggable(True)
    plt.tight_layout()
    
    storePath = os.path.join(getStorePath(), 'figs', 'reconfig_synctime.png')
    # 确保目录存在
    if not os.path.exists(os.path.dirname(storePath)):
        os.makedirs(os.path.dirname(storePath))
    plt.savefig(storePath)
    storePath = os.path.join(getStorePath(), 'figs', 'reconfig_synctime.pdf')
    plt.savefig(storePath)
    
    plt.show()

def reconfig_synctime_bandwidth(dataPath):
    plt.rcParams['pdf.fonttype'] = 42
    plt.rcParams['ps.fonttype'] = 42
    plt.rcParams['figure.figsize'] = (width, height) # Set appropriate figure size

    sync_methods = ['fullsync', 'fastsync', 'tMPTsync', 'lesssync']
    labels = ['Full sync (Ethereum)', 'Fast sync (Ethereum)', 'tMPT [14]', 'LessChain (ours)']
    bandwidths = [5, 25, 50, 100]
    colors = ['#2F2321', '#AA4F23', '#FED875', '#6C627A', '#F8EBDC']
    markers = ['x', '.', '>', 's']

    for i, method in enumerate(sync_methods):
        syncTimes = []
        for j, bw in enumerate(bandwidths):
            file_path = os.path.join(getStorePath(), dataPath, method, f'bandwidth{bw}MB', 'average_data.csv')
            data = pd.read_csv(file_path)
            syncTime = data['Average of syncTime(ms)'].mean()
            syncTimes.append(syncTime)

        # Plotting logic here may need to be adjusted
        plt.plot(bandwidths, syncTimes, color=colors[i], label=labels[i], marker=markers[i])

    # 由于一台机子上实际运行4个共识节点，所以一个节点的带宽应该是机器总带宽除以4
    nodeBandwidths = [i/4 for i in bandwidths]
    plt.xticks(bandwidths, nodeBandwidths, fontsize=16)
    plt.xlabel('Bandwidth (Mbits/s)',fontsize=18)
    plt.yticks(fontsize=18)
    plt.ylabel('Sync Time (ms)',fontsize=18)
    plt.legend(fontsize=14,loc='best')
    plt.tight_layout()
    
    storePath = os.path.join(getStorePath(), 'figs', 'reconfig_synctime_bandwidth.png')
    # 确保目录存在
    if not os.path.exists(os.path.dirname(storePath)):
        os.makedirs(os.path.dirname(storePath))
    plt.savefig(storePath)
    storePath = os.path.join(getStorePath(), 'figs', 'reconfig_synctime_bandwidth.pdf')
    plt.savefig(storePath)
    
    plt.show()

def main():
    motivation_security()

    # TPS('results/tps1/')
    # Latency('results/tps1/')
    # Workload4shard('results/workload1/')

    # TPS_inject1('results/injectSpeed1/')
    # TPS_inject('results/injectSpeed1/')
    # RollBackRate('results/rollback1/')
    # Reconfig('results/reconfig/')
    # Reconfig1('results/reconfig/')

    # reconfig_security_S_n()
    
    # reconfig_datasize('results/reconfigSyncData/')
    # reconfig_synctime('results/reconfigSyncData/')
    # reconfig_synctime_bandwidth('results/reconfigSyncDifferBandwidth/')

if __name__ == "__main__":
    main()