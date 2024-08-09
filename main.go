package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type Position struct {
	Liquidity                *big.Int
	FeeGrowthInside0LastX128 *big.Int
	FeeGrowthInside1LastX128 *big.Int
	TokensOwed0              *big.Int
	TokensOwed1              *big.Int
}

const (
	// public node from https://chainlist.org/chain/42161
	nodeAddr = "https://arbitrum.llamarpc.com"

	abiUniV3Pool    = `[{"inputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"name":"positions","outputs":[{"internalType":"uint128","name":"liquidity","type":"uint128"},{"internalType":"uint256","name":"feeGrowthInside0LastX128","type":"uint256"},{"internalType":"uint256","name":"feeGrowthInside1LastX128","type":"uint256"},{"internalType":"uint128","name":"tokensOwed0","type":"uint128"},{"internalType":"uint128","name":"tokensOwed1","type":"uint128"}],"stateMutability":"view","type":"function"}]`
	positionsMethod = "positions"
)

// https://app.uniswap.org/explore/pools
// https://arbiscan.io/address/0xc6962004f452be9203591991d15f6b388e09e8d0#readContract
var poolAddress = common.HexToAddress("0xc6962004f452be9203591991d15f6b388e09e8d0")

var (
	//Random minter from logs pool
	ownerPositionAddress       = common.HexToAddress("0xF829c130478599E4EF49F6e02EDaA1F8736E9B00")
	tickLower            int32 = -197740
	tickUpper            int32 = -197640
)

func main() {
	positionKey, err := calcPositionKey(ownerPositionAddress, tickLower, tickUpper)
	if err != nil {
		log.Fatal("calc position key:", err)
	}
	contract, _ := abi.JSON(strings.NewReader(abiUniV3Pool))
	calldata, _ := contract.Pack(positionsMethod, positionKey)

	client, err := ethclient.Dial(nodeAddr)
	if err != nil {
		log.Fatal("conenct to node:", err)
	}

	response, err := client.CallContract(context.Background(), ethereum.CallMsg{To: &poolAddress, Data: calldata}, nil)
	if err != nil {
		log.Fatal("call contract:", err)
	}

	var position Position

	if err := contract.UnpackIntoInterface(&position, positionsMethod, response); err != nil {
		log.Fatal("parse result contract: ", err, "response: ", string(response))
	}

	fmt.Printf("%+v", position)
}

func calcPositionKey(address common.Address, tickLower, tickUpper int32) (common.Hash, error) {
	callData, err := encodePacked(address, tickLower, tickUpper)
	if err != nil {
		return common.Hash{}, err
	}

	return crypto.Keccak256Hash(callData), nil
}

// https://github.com/Uniswap/v3-core/blob/d8b1c635c275d2a9450bd6a78f3fa2484fef73eb/test/shared/utilities.ts#L75
// https://docs.soliditylang.org/en/develop/abi-spec.html#non-standard-packed-mode
// ethers.utils.solidityPack()
func encodePacked(args ...interface{}) ([]byte, error) {
	var buffer bytes.Buffer

	for _, arg := range args {
		switch v := arg.(type) {
		case common.Address:
			buffer.Write(v.Bytes())
		case *big.Int:
			buffer.Write(int24Bytes(v))
		case int32:
			bigInt := big.NewInt(int64(v))
			buffer.Write(int24Bytes(bigInt))
		case string:
			buffer.Write([]byte(v))
		case []byte:
			buffer.Write(v)
		default:
			return nil, fmt.Errorf("unsupported type: %T", v)
		}
	}

	return buffer.Bytes(), nil
}

func int24Bytes(n *big.Int) []byte {
	bytes := make([]byte, 3)

	//adding "f" before a number
	if n.Sign() == -1 {
		n = big.NewInt(0).Sub(big.NewInt(0), n)
		n = big.NewInt(0).Sub(big.NewInt(1<<24), n)
	}

	copy(bytes, n.Bytes()[max(0, len(n.Bytes())-3):])

	return bytes
}
