## 批量转账
### 1. 部署 BatchTransfer 合约
Contract source code : https://gist.github.com/gaozhengxin/f6fe3d28f9aace7abeebefdb8d833cd4
| Chain | BatchTransfer |
|-------|:-------------:|
| fantom testnet | 0x98F32AB8f99e2C40a341E6c3b2A193AA3272129B |

### 2. 把空投地址列表保存在 addrs.csv 文件中备孕
```csv
0xA78b5e7699eD8dFC97Bd7999ab1B82Cca2F1c715
0x68b94cD8aAe8Db246a8e5E09Fdd516b0e77D0a68
0xbe1fb701Fa983736dcB613e95900d6D4E239De31
```

### 3. 把私钥保存在 key 文件中备孕

### 4. 编辑 config 文件
```json
{
  "rpc": "https://rpc.ankr.com/fantom_testnet",
  "chainId": 4002,
  "token": "0x4C4BC2Ba97b9bc8C88C42d3aEfE775b62d45eFFD",
  "amount": 20000000000000000,
  "batchTransfer": "0x98F32AB8f99e2C40a341E6c3b2A193AA3272129B"
}
```
`token` 是 erc20 token 合约地址

`"token": "0x0000000000000000000000000000000000000000"` 表示 gas token

`amount` 是数量

### 5. 运行 airdrop 脚本
```shell
go run ./ --config ./config.json --key ./key --addrs ./addrs.csv --log ./airdrop.log
```

### 6. 检查结果
查看 airdrop.log 文件