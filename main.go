package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	ethclient "github.com/ethereum/go-ethereum/ethclient"
	"github.com/urfave/cli/v2"
)

// Send airdrop with BatchTransfer contract
// https://gist.github.com/gaozhengxin/f6fe3d28f9aace7abeebefdb8d833cd4

func main() {
	var configFilePath string
	var keyFilePath string
	var addressListFilePath string
	var logfile string

	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "config",
				Value:       "./config.json",
				Usage:       "airdrop config file",
				Destination: &configFilePath,
			},
			&cli.StringFlag{
				Name:        "key",
				Value:       "./key",
				Usage:       "airdrop private key",
				Destination: &keyFilePath,
			},
			&cli.StringFlag{
				Name:        "addrs",
				Value:       "./addrs.csv",
				Usage:       "airdrop address list",
				Destination: &addressListFilePath,
			},
			&cli.StringFlag{
				Name:        "log",
				Value:       "./airdrop.log",
				Usage:       "log output file",
				Destination: &logfile,
			},
		},
		Action: func(cCtx *cli.Context) (actionError error) {
			defer func() {
				if r := recover(); r != nil {
					actionError = fmt.Errorf("%v", r)
				}
			}()

			f, err := os.OpenFile(logfile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
			if err != nil {
				log.Fatalf("error opening file: %v", err)
			}
			defer f.Close()

			multi := io.MultiWriter(f, os.Stdout)
			log.SetOutput(multi)

			checkError := func(err error) {
				if err != nil {
					log.Println(err)
				}
			}

			config, err := loadConfig(configFilePath)
			checkError(err)
			log.Printf("config : %+v\n", config)

			sk, err := loadKey(keyFilePath)
			checkError(err)
			senderAddr := crypto.PubkeyToAddress(sk.PublicKey)
			log.Printf("sender address : %v\n", senderAddr)

			addrs, err := loadAddressList(addressListFilePath)
			checkError(err)
			log.Printf("loaded address list length %v\n", len(addrs))

			err = runAirdrop(config, sk, addrs)
			checkError(err)

			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

type Config struct {
	RPC           string         `json:rpc`
	ChainId       uint64         `json:chainId`
	Token         common.Address `json:token`
	Amount        BigInt         `json:amount`
	BatchTransfer common.Address `json:"batchTransfer"`
}

type BigInt struct {
	big.Int
}

func (b BigInt) MarshalJSON() ([]byte, error) {
	return []byte(b.String()), nil
}

func (b *BigInt) UnmarshalJSON(p []byte) error {
	if string(p) == "null" {
		return nil
	}
	var z big.Int
	_, ok := z.SetString(string(p), 10)
	if !ok {
		return fmt.Errorf("not a valid big integer: %s", p)
	}
	b.Int = z
	return nil
}

func loadConfig(configFilePath string) (*Config, error) {
	dat, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, err
	}
	config := new(Config)
	err = json.Unmarshal(dat, config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func loadKey(keyFilePath string) (*ecdsa.PrivateKey, error) {
	dat, err := os.ReadFile(keyFilePath)
	if err != nil {
		return nil, err
	}
	keyhex := strings.ReplaceAll(string(dat), "\n", "")
	return crypto.HexToECDSA(keyhex)
}

func loadAddressList(addressListFilePath string) ([]common.Address, error) {
	dat, err := os.ReadFile(addressListFilePath)
	if err != nil {
		return nil, err
	}
	strs := strings.Split(string(dat), "\n")
	addrs := make([]common.Address, len(strs))
	for i, s := range strs {
		if (common.HexToAddress(s) == common.Address{}) {
			continue
		}
		addrs[i] = common.HexToAddress(s)
	}
	return addrs, nil
}

var (
	jsonabi_batchTransfer string = `[{"inputs":[],"stateMutability":"nonpayable","type":"constructor"},{"inputs":[{"internalType":"address","name":"token","type":"address"},{"internalType":"address[]","name":"receivers","type":"address[]"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"batchTransfer","outputs":[],"stateMutability":"payable","type":"function"}]`
)

func runAirdrop(config *Config, sk *ecdsa.PrivateKey, addrs []common.Address) error {
	ctx := context.Background()
	cli, err := ethclient.DialContext(ctx, config.RPC)
	if err != nil {
		return err
	}

	chainId, err := cli.ChainID(ctx)
	if err != nil {
		return err
	}

	signer := types.NewLondonSigner(chainId)

	if chainId.Uint64() != config.ChainId {
		return errors.New("wrong chain id")
	}

	addrsBatchs := make([][]common.Address, len(addrs)/20+1)
	for i := 0; i < len(addrs)/20+1; i++ {
		if (i+1)*20 < len(addrs) {
			addrsBatchs[i] = addrs[i*20 : (i+1)*20]
		} else {
			addrsBatchs[i] = addrs[i*20:]
		}
	}

	abi_batchTransfer, _ := abi.JSON(strings.NewReader(jsonabi_batchTransfer))
	log.Printf("token : %v\n", config.Token)

	senderAddress := crypto.PubkeyToAddress(sk.PublicKey)

	for _, addrs := range addrsBatchs {
		calldata, err := abi_batchTransfer.Pack(
			"batchTransfer",
			config.Token,
			addrs,
			&config.Amount.Int,
		)
		if err != nil {
			log.Printf("pack calldata error : %v\nbatch : %v\n", err, addrs)
			continue
		}

		value := big.NewInt(0)
		if config.Token == (common.Address{}) {
			value = new(big.Int).Mul(&config.Amount.Int, big.NewInt(int64(len(addrs))))
		}

		gasLimit, err := cli.EstimateGas(context.Background(), ethereum.CallMsg{
			From:  senderAddress,
			To:    &config.BatchTransfer,
			Data:  calldata,
			Value: value,
		})
		if err != nil {
			log.Printf("estimate gas error : %v\nbatch : %v\n", err, addrs)
			continue
		}

		nonce, err := cli.NonceAt(ctx, senderAddress, nil)
		if err != nil {
			log.Printf("get nonce error : %v\nbatch : %v\n", err, addrs)
			continue
		}

		gasPrice, err := cli.SuggestGasPrice(ctx)
		if err != nil {
			log.Printf("get gas price error : %v\nbatch : %v\n", err, addrs)
			continue
		}

		tx := types.NewTransaction(nonce, config.BatchTransfer, value, gasLimit, gasPrice, calldata)

		signedTx, err := types.SignTx(tx, signer, sk)
		if err != nil {
			log.Printf("sign transaction error : %v\nbatch : %v\n", err, addrs)
			continue
		}

		err = cli.SendTransaction(ctx, signedTx)
		if err != nil {
			log.Printf("send transaction error : %v\nbatch : %v\n", err, addrs)
			continue
		}
		log.Printf("send transaction success : %x\n", signedTx.Hash())

		for {
			i := 0
			_, isPending, err := cli.TransactionByHash(ctx, signedTx.Hash())
			if isPending == false && err == nil {
				break
			}
			time.Sleep(time.Second * 5)
			i++
			if i > 30 {
				break
			}
		}

		txRes, err := cli.TransactionReceipt(ctx, signedTx.Hash())
		if err != nil {
			log.Printf("get transaction receipt error : %v\nbatch : %v\n", err, addrs)
			continue
		}
		if txRes.Status != 1 {
			log.Printf("transaction execution error : %v\nbatch : %v\n", txRes, addrs)
			continue
		}
	}

	return nil
}
