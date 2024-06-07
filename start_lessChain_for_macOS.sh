O#!/bin/bash
## 启动lessChain的脚本
## 注：针对Windows的部分使用了osascript，这实际上是macOS特有的，并不能在Windows上运行。
## 如果需要在Windows上执行类似的自动化任务，需要使用其他工具或方法。

# 一个简单的函数，计算每个新窗口的位置。
get_window_position() {
    local index=$1
    local xoffset=200
    local yoffset=30
    local xindex=index%6
    local yindex=index/6
    local x=$((xindex * xoffset + 30))
    local y=$((yindex * yoffset + 30))
    echo "$x,$y"
}

# 使用循环启动终端并设置每个窗口的位置
counter=0
SHARD_NUM=2
SHARD_ALL_NODE_NUM=8

# 获取脚本所在的目录
SCRIPT_DIR="$( cd "$( dirname "$0" )" && pwd )"

GETH_DIR="$SCRIPT_DIR/eth_chain/geth-chain-data"

# 跳转到指定目录
cd $GETH_DIR

# 删除历史内容
rm -r ./data/geth

# 初始化
geth --datadir ./data init genesis.json
sleep 2

# 启动信标链（以太坊私链）
# 判断操作系统并执行相应的命令
if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS
    osascript -e 'tell app "Terminal" to do script "cd '$GETH_DIR' && geth --datadir ./data --syncmode '\''full'\'' --port 30310 --http --http.addr '\''localhost'\'' --http.port 8545 --http.api '\''personal,eth,net,web3,txpool,miner'\'' --ws --ws.port 8545 --allow-insecure-unlock --networkid 1337 -unlock '\''0x9128d0f6f5e04bd43305f7a323a67309c694a8f4'\'' --password ./emptyPsw.txt --mine --miner.etherbase=0x9128d0f6f5e04bd43305f7a323a67309c694a8f4"'
    position=$(get_window_position $counter)
    osascript -e "tell application \"Terminal\" to set position of front window to {$position}"
elif [[ "$OSTYPE" == "cygwin" ]] || [[ "$OSTYPE" == "msys" ]] || [[ "$OSTYPE" == "win32" ]]; then
    # Windows
    osascript -e 'tell app "Terminal" to do script "cd '$GETH_DIR' && geth --datadir ./data --syncmode full --port 30310 --http --http.addr localhost --http.port 8545 --http.api personal,eth,net,web3,txpool,miner --ws --ws.port 8545 --allow-insecure-unlock --networkid 1337 -unlock 0x9128d0f6f5e04bd43305f7a323a67309c694a8f4 --password ./emptyPsw.txt --mine --miner.etherbase=0x9128d0f6f5e04bd43305f7a323a67309c694a8f4"'
else
    echo "Unknown OS type: $OSTYPE"
    exit 1
fi
sleep 2

# 启动booter终端
osascript -e 'tell app "Terminal" to do script "cd '$SCRIPT_DIR' && go build -o ./lessChain && ./lessChain -r booter -S '$SHARD_NUM'"'
((counter++))
position=$(get_window_position $counter)
osascript -e "tell application \"Terminal\" to set position of front window to {$position}"
sleep 2

# 启动client终端
osascript -e 'tell app "Terminal" to do script "cd '$SCRIPT_DIR' && ./lessChain -r client -S '$SHARD_NUM'"'
((counter++))
position=$(get_window_position $counter)
osascript -e "tell application \"Terminal\" to set position of front window to {$position}"
sleep 1

for((j=0;j<$SHARD_NUM;j++));
do
    # 每个分片启动若干个终端，每个终端代表一个节点
    for((i=0;i<$SHARD_ALL_NODE_NUM;i++));
    do
        osascript -e 'tell app "Terminal" to do script "cd '$SCRIPT_DIR' && ./lessChain -r node -S '$SHARD_NUM' -s '$j' -n '$i'"'
        ((counter++))
        position=$(get_window_position $counter)
        osascript -e "tell application \"Terminal\" to set position of front window to {$position}"

    done
    # sleep 1
done
