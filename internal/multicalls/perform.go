package multicalls

import (
	"math/big"

	"github.com/yearn/ydaemon/common/env"
	"github.com/yearn/ydaemon/common/ethereum"
)

func Perform(chainID uint64, calls []ethereum.Call, blockNumber *big.Int) map[string][]interface{} {
	caller := ethereum.MulticallClientForChainID[chainID]
	batchSize := env.MAX_BATCH_SIZE[chainID]
	return caller.ExecuteByBatch(calls, batchSize, blockNumber)
}
