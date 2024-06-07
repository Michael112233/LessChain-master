# 写在前面
这份代码是ICDE2024的投稿论文 "Stateless and Trustless: A Two-Layer Secure Sharding Blockchain" （以下简称为LessChain）的实现。
该代码支持在单机或多机上运行，你可以通过配置文件来修改相应的设置。

# 安装
1. 从[https://github.com/xumy29/LessChain](https://github.com/xumy29/LessChain)获取最新代码（该仓库的近期代码版本已经包含在当前目录下。出于知识产权保护，我们暂时未公开该仓库。如果论文被接收，我们会将仓库公开）
2. `cd LessChain & go build -o ./lessChain`
第一次编译时会自动拉取所需的依赖，可能需要一点时间。（如果下载太慢可能是需要换源，可以尝试设置goproxy：`export GOPROXY=https://goproxy.cn` 后再拉取依赖。
3. 安装geth。[geth_download](https://geth.ethereum.org/downloads)


# 配置
+ **下载数据集**： 我们采用以太坊第14920000个区块到第14960000个区块的数据（200w+条交易），下载后需要对数据进行初步处理。你可以直接从以下地址下载处理后的数据：[data_from_google_drive](https://drive.google.com/file/d/1gIBGcneoUz9jaU48PYCjP6xjWegRlgE-/view?usp=sharing), 也可以从源数据下载地址下载：[XBlock](https://zhengpeilin.com/download.php?file=14750000to14999999_BlockTransaction.zip)。 将处理好的数据文件放入项目的data文件夹下即可。


+ **参数文件设置**: `"lessChain_dirname/cfg/debug.json"`
假设需要配置4个分片，在2台机器上运行，以下是初始时分片ID为0、节点ID为0的节点的配置文件
(注：json文件中不支持注释，记得将注释删掉)

```json
{
    // 运行程序的机器数量
    "MachineNum": 2,
    // 当前机器上运行分片的起始ID（一台机器上可能同时运行多个分片）
    "ShardStartIndex": 0,
    
    // 日志信息
    "LogLevel": 4,
    "LogFile": "",
    "IsProgressBar": true,
    "IsLogProgress": true,
    "LogProgressInterval": 20,

    // 当前节点的角色，节点角色可能是 client、node、booter
    "Role": "node",

    // 客户端数量
    "ClientNum": 1,
    // 当前客户端的ID
    "ClientId": 0,

    // 分片数量
    "ShardNum": 4,
    // 节点所属分片的ID
    "ShardId": 0,
    // 当前所属共识分片中的节点数量
    "ShardSize": 8,
    // 共识分片多签名所需的签名者数量
    "MultiSignRequiredNum": 2,
    // 当前所属存储分片中的节点数量
    "ComAllNodeNum":8,
    // 当前节点的ID
    "NodeId": 0,


    // 注入交易总量
    "MaxTxNum": 240000,
    // 交易注入速度
    "InjectSpeed": 1000,
    // 出块间隔，必须大于等于3s，可在worker.go中更改此限制（但可能会产生网络问题）
    "RecommitInterval": 4,
    // 区块容量
    "MaxBlockTXSize": 1000,

    // 重组高度
    "Height2Reconfig": 3,
    // 重组同步方式，有lesssync、fastsync、fullsync和tMPTsync
    "ReconfigMode": "lesssync",
    // 当同步方式为 fastsync 时，同步的最近区块数量
    "FastsyncBlockNum":20,

    // Layer1链的出块间隔
    "TbchainBlockIntervalSecs": 10,
    // 交易超时时间
    "Height2Rollback": 2000,
    // Layer1区块确认高度
    "Height2Confirm": 0,
    // Layer1链的设置
    "BeaconChainMode": 2, // 2指以太坊私链
    "BeaconChainPort": 8545,
    "BeaconChainID": 1337,

    "ExitMode": 1,

    "DatasetDir": "data/len3_data.csv"
}
```
+ **机器设置**: `"lessChain_dirname/cfg/node.go"`
设置各个节点的ip地址，以允许节点间的通信。


# 运行

实际运行时，以每台机器为单位，在确保`ShardNum`、`MachineNum`和`ShardStartIndex`正确赋值以后，在一台机器上运行脚本`lessChain_dirname/start_lessChain_for_linux.sh`。该脚本会自动开启多个进程，修改每个进程中的配置文件，运行多个分片中的节点。比如4个分片两台机器的设置下，机器1配置文件为
```json
{
    "MachineNum": 2,
    "ShardStartIndex": 0,
    "Role": "node",
    "ShardNum": 4,
    ... // other parameters
}
```
机器2配置文件为
```json
{
    "MachineNum": 2,
    "ShardStartIndex": 1,
    "Role": "node",
    "ShardNum": 4,
    ... // other parameters
}
```
那么机器1会运行分片0、2，机器2会运行分片1、3。

除了运行各个分片，还需要另外一个机器来运行Layer1链以及客户端。只需修改配置文件中的Role，并且确保geth的参数被正确的设置，如下：
```json
{
    "Role": "",
    "BeaconChainMode": 2, 
    "BeaconChainPort": 8545,
    "BeaconChainID": 1337,
    ... // other parameters
}
```
再运行脚本`lessChain_dirname/start_lessChain_for_linux.sh`。

以上三台机器都正常配置和运行脚本之后，LessChain就会开始运行起来了。你可以在各个机器的`lessChain_dirname/logs`文件夹下查看节点的日志。可以通过查看运行客户端的机器的`lessChain_dirname/logs/client.log`查看整体的交易处理进度。

# 模块简介

## beaconChain 模块
与Layer1的链对接的模块。有三种模式可供选择，分别是模拟链（该模式已废弃，请勿使用）、通过ganache或geth部署的以太坊的私链。选择后两者时需要先安装对应软件并在cfg/debug.json中设置私链端口号、私链ID等参数。

## cfg 模块
配置文件，配置账户、ip地址等。

## client 模块
向委员会注入交易，接收交易收据，当Layer1链对交易所在区块确认后，客户端将交易状态记录到result模块，并对跨分片交易做下一步处理（发送后半部分或者回滚交易）。

## committee 模块
维护交易池，从shard中获取状态，打包区块，执行交易，发送收据到客户端，发送区块到shard，发送时间信标到Layer1链，等等。相当于论文中的共识分片。
共识分片会定期重组，其间组成各节点被随机打乱和重新分配。

## controller 模块
节点的启动控制，根据配置文件进行初始化。

## core 模块
定义了各种基础的类型，如交易、区块、区块链等。

## data 模块
对数据进行初步的处理和分配。

## eth_chain模块
实现了与以太坊私链交互的逻辑，如部署、调用合约、监听事件等。可以在beaconChain中调用。
当前支持的以太坊私链有：Geth。

## experiments
存放实验结果。

## log 模块
实现了日志的功能，支持不同的日志级别。

## logs 模块
记录程序运行中产生的日志。

## messageHub 模块
多种类型的消息通过 messageHub 进行传递。

## node 模块
定义了一个区块链节点的基本组成和行为。

## pbft 模块
pbft协议的实现。

## result 模块
记录每笔交易的执行状态，输出时延、交易量、吞吐量等指标。
一笔跨分片交易可能有多个状态，比如 [Cross1TXTypeSuccess, RollbackSuccess] 代表该交易先在委员会1上链，而后在委员会2上超时，进而被回滚。

## shard 模块
维护区块链，存储区块、状态，提供某笔交易在本分片区块链上链（proof of inclusion）或没有上链（proof of exclusion）的证据。相当于论文中的存储分片。

## trie 模块
对tMPT的实现。在geth基础上做了些小修改。

## utils 模块
常用的工具函数。








## controller 模块
节点的启动控制，根据配置文件进行初始化。


# 遇到问题
联系作者 xumy29@mail2.sysu.edu.cn








