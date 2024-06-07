# Preface
This code is the implementation for the paper submitted to ICDE2024, "Stateless and Trustless: A Two-Layer Secure Sharding Blockchain" (hereinafter referred to as LessChain).
The code supports running on a single machine or multiple machines, and you can modify the relevant settings through the configuration file.

# Installation
1. Get the latest code from [https://github.com/xumy29/LessChain](https://github.com/xumy29/LessChain) (The recent version of the repository is already included in the current directory. Due to intellectual property protection, we have not made this repository public for the time being. If the paper is accepted, we will make the repository public.)
2. `cd LessChain & go build -o ./lessChain`
   The first compilation will automatically pull the required dependencies, which might take some time. (If the download is too slow, it might be necessary to change the source, try setting the goproxy: `export GOPROXY=https://goproxy.cn` and then pull the dependencies.)
3. Install geth. [geth_download](https://geth.ethereum.org/downloads)


# Configuration
+ **Download Dataset**: We use the data from Ethereum blocks number 14920000 to 14960000 (over 2 million transactions). After downloading, the data needs initial processing. You can directly download the processed data from this link: [data_from_google_drive](https://drive.google.com/file/d/1gIBGcneoUz9jaU48PYCjP6xjWegRlgE-/view?usp=sharing), or download the raw data from the source data download link: [XBlock](https://zhengpeilin.com/download.php?file=14750000to14999999_BlockTransaction.zip). Place the processed data file into `lessChain_dirname/data/`.



+ **Parameter File Settings**: `"lessChain_dirname/cfg/debug.json"`
   Assuming 4 shards are configured to run on 2 machines, here is the configuration file for the node with initial shard ID 0 and node ID 0
   (Note: json files do not support comments, remember to delete the comments)

```json
{
    // Number of machines running the program
    "MachineNum": 2,
    // Starting ID of the shard running on the current machine (multiple shards may run on one machine)
    "ShardStartIndex": 0,
    
    // Log information
    "LogLevel": 4,
    "LogFile": "",
    "IsProgressBar": true,
    "IsLogProgress": true,
    "LogProgressInterval": 20,

    // The role of the current node, possible roles are client, node, booter
    "Role": "node",

    // Number of clients
    "ClientNum": 1,
    // ID of the current client
    "ClientId": 0,

    // Number of shards
    "ShardNum": 4,
    // ID of the shard to which the node belongs
    "ShardId": 0,
    // Number of nodes in the current consensus shard
    "ShardSize": 8,
    // Number of signatories required for multi-signature in the consensus shard
    "MultiSignRequiredNum": 2,
    // Number of nodes in the current storage shard
    "ComAllNodeNum":8,
    // ID of the current node
    "NodeId": 0,


    // Total number of transactions to be injected
    "MaxTxNum": 240000,
    // Speed of transaction injection
    "InjectSpeed": 1000,
    // Block interval, must be greater than or equal to 3s, can be changed in worker.go (but may cause network issues)
    "RecommitInterval": 4,
    // Block capacity
    "MaxBlockTXSize": 1000,

    // reconfiguration interval
    "Height2Reconfig": 3,
    // Reconfiguration synchronization mode, includes lesssync, fastsync, fullsync, and tMPTsync
    "ReconfigMode": "lesssync",
    // When synchronization mode is fastsync, the number of recent blocks to be synchronized
    "FastsyncBlockNum":20,

    // Layer1 chain block interval
    "TbchainBlockIntervalSecs": 10,
    // Transaction timeout time
    "Height2Rollback": 2000,
    // Layer1 block confirmation height
    "Height2Confirm": 0,
    // Settings for Layer1 chain
    "BeaconChainMode": 2, // 2 stands for Ethereum private chain
    "BeaconChainPort": 8545,
    "BeaconChainID": 1337,

    "ExitMode": 1,

    "DatasetDir": "data/len3_data.csv"
}
```

+ **Machine Settings**: `"lessChain_dirname/cfg/node.go"`
   Set the IP addresses of each node to allow inter-node communication.

# Execution

When actually running, start the script `lessChain_dirname/start_lessChain_for_linux.sh` on each machine, ensuring that `ShardNum`, `MachineNum`, and `ShardStartIndex` are correctly assigned. This script will automatically start multiple processes, modify the configuration file in each process, and run the nodes in multiple shards. For example, under the setting of 4 shards and two machines, the configuration file for machine 1 is

```json
{
    "MachineNum": 2,
    "ShardStartIndex": 0,
    "Role": "node",
    "ShardNum": 4,
    ... // other parameters
}
```
and the configuration file for machine 2 is
```json
{
    "MachineNum": 2,
    "ShardStartIndex": 1,
    "Role": "node",
    "ShardNum": 4,
    ... // other parameters
}
```
Thus, machine 1 will run shards 0 and 2, while machine 2 will run shards 1 and 3.

In addition to running the shards, another machine is needed to run the Layer1 chain and the client. Just modify the Role in the configuration file and ensure that the geth parameters are correctly set, as follows:
```json
{
    "Role": "",
    "BeaconChainMode": 2, 
    "BeaconChainPort": 8545,
    "BeaconChainID": 1337,
    ... // other parameters
}
```

Then run the script `lessChain_dirname/start_lessChain_for_linux.sh`.

Once all three machines are properly configured and running the script, LessChain will start operating. You can check the node logs in the `lessChain_dirname/logs` folder on each machine. The overall transaction processing progress can be viewed in the `lessChain_dirname/logs/client.log` on the machine running the client.

# Module Overview

## beaconChain Module
Interacts with the Layer1 chain. There are three modes to choose from, which are the simulated chain (this mode has been deprecated, please do not use), Ethereum private chain deployed through ganache or geth. When choosing the latter two, you need to install the corresponding software first and set the private chain port number, chain ID, etc., in cfg/debug.json.

## cfg Module
Configuration files, set accounts, IP addresses, etc.

## client Module
Injects transactions into the committee, receives transaction receipts, and once the Layer1 chain confirms the block containing the transactions, the client records the transaction status in the result module and processes the next steps for cross-shard transactions (sends the latter part or rolls back the transaction).

## committee Module
Maintains the transaction pool, gets the status from the shard, packages blocks, executes transactions, sends receipts to the client, sends blocks to the shard, sends time beacons to the Layer1 chain, etc. Equivalent to the consensus shard in the paper.
The consensus shard is regularly reconfigured, during which the composition of various nodes is randomly shuffled and reassigned.

## controller Module
Controls the startup of the node, initializing according to the configuration file.

## core Module
Defines various basic types, such as transactions, blocks, blockchains, etc.

## data Module
Processes and distributes data initially.

## eth_chain Module
Implements logic for interacting with the Ethereum private chain, such as deploying, calling contracts, listening to events, etc. Can be called in beaconChain.
Currently supported Ethereum private chains include: Geth.

## experiments
Stores experimental results.

## log Module
Implements logging functionality, supporting different logging levels.

## logs Module
Records logs generated during program execution.

## messageHub Module
Various types of messages are passed through messageHub.

## node Module
Defines the basic composition and behavior of a blockchain node.

## pbft Module
Implementation of the pbft protocol.

## result Module
Records the execution status of each transaction, outputting latency, transaction volume, throughput, and other indicators.
A cross-shard transaction may have multiple statuses, such as [Cross1TXTypeSuccess, RollbackSuccess] indicating that the transaction was first chained in committee 1 and then timed out in committee 2, leading to its rollback.

## shard Module
Maintains the blockchain, stores blocks, states, and provides evidence that a particular transaction is on-chain (proof of inclusion) or not on-chain (proof of exclusion) in this shard. Equivalent to the storage shard in the paper.

## trie Module
Implementation of tMPT. Made some minor modifications based on geth.

## utils Module
Commonly used utility functions.

# Issues
For any issues, contact the author at xumy29@mail2.sysu.edu.cn
