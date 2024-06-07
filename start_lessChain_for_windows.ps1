# windows上运行的脚本
# 注：路径上不要包含中文或特殊符号！！！

# 定义执行命令的函数
function RunInNewTerminal {
    param (
        [string]$command
    )
    Start-Process powershell.exe -ArgumentList ("-NoExit", "-Command $command")
}


$SHARD_NUM=2
$SHARD_ALL_NODE_NUM=8

# 获取脚本所在的路径
$scriptPath = Split-Path -Path $MyInvocation.MyCommand.Definition -Parent
$ethChainDir = Join-Path -Path $scriptPath -ChildPath "eth_chain\geth-chain-data"

# 删除旧的内容并初始化
if (Test-Path "$ethChainDir\data\geth") {
    Remove-Item -Path "$ethChainDir\data\geth" -Recurse -Force
}
# Set-Location -Path $ethChainDir
# & geth --datadir ./data init genesis.json


RunInNewTerminal "Set-Location $ethChainDir; geth --datadir ./data init genesis.json; geth --datadir ./data --syncmode full --port 30310 --http --http.addr localhost --http.port 8545 --http.api personal,eth,net,web3,txpool,miner --ws --ws.port 8545 --allow-insecure-unlock --networkid 1337 -unlock 0x9128d0f6f5e04bd43305f7a323a67309c694a8f4 --password ./emptyPsw.txt --mine --miner.etherbase=0x9128d0f6f5e04bd43305f7a323a67309c694a8f4"
Start-Sleep -Seconds 3

# 启动其他终端并运行相应的命令
RunInNewTerminal "Set-Location $scriptPath; go build -o ./lessChain.exe; ./lessChain.exe -r booter -S $SHARD_NUM"
Start-Sleep -Seconds 10
RunInNewTerminal "Set-Location $scriptPath; ./lessChain.exe -r client -S $SHARD_NUM"
Start-Sleep -Seconds 1

# 启动终端，运行节点命令
0..($SHARD_NUM-1) | ForEach-Object {
    $shardIndex = $_
    RunInNewTerminal "Set-Location $scriptPath; ./lessChain.exe -r node -S $SHARD_NUM -s $shardIndex -n 0"
    # Start-Sleep -Seconds 2
    1..($SHARD_ALL_NODE_NUM-1) | ForEach-Object {
        $nodeIndex = $_
        RunInNewTerminal "Set-Location $scriptPath; ./lessChain.exe -r node -S $SHARD_NUM -s $shardIndex -n $nodeIndex"
        Start-Sleep -Seconds 0.2
    }
}

