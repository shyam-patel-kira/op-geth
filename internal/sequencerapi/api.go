package sequencerapi

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/rpc"
)

const (
	txConditionalMaxCost = 1000
)

type ethApi struct {
	b     ethapi.Backend
	txApi ethapi.TransactionAPI
}

func GetAPIs(b ethapi.Backend, txApi ethapi.TransactionAPI) []rpc.API {
	return []rpc.API{
		{
			Namespace: "eth",
			Service:   &ethApi{b, txApi},
		},
	}
}

func (s *ethApi) SendRawTransactionConditional(ctx context.Context, txBytes hexutil.Bytes, cond types.TransactionConditional) (common.Hash, error) {
	cost := cond.Cost()
	if cost > txConditionalMaxCost {
		return common.Hash{}, fmt.Errorf("conditional cost, %d, exceeded 1000", cost)
	}

	state, header, err := s.b.StateAndHeaderByNumber(context.Background(), rpc.LatestBlockNumber)
	if err != nil {
		return common.Hash{}, err
	}
	if header.CheckTransactionConditional(&cond); err != nil {
		return common.Hash{}, fmt.Errorf("failed header check: %w", err)
	}
	if state.CheckTransactionConditional(&cond); err != nil {
		return common.Hash{}, fmt.Errorf("failed state check: %w", err)
	}

	// We also check against the prior block to eliminate the incentive for MEV in comparison with sendRawTransaction
	prevBlock := rpc.BlockNumberOrHash{BlockHash: &header.ParentHash}
	prevState, _, err := s.b.StateAndHeaderByNumberOrHash(context.Background(), prevBlock)
	if err != nil {
		return common.Hash{}, err
	}
	if prevState.CheckTransactionConditional(&cond); err != nil {
		return common.Hash{}, fmt.Errorf("failed state check for prior block: %w", err)
	}

	tx := new(types.Transaction)
	if err := tx.UnmarshalBinary(txBytes); err != nil {
		return common.Hash{}, err
	}

	tx.SetConditional(&cond)
	return ethapi.SubmitTransaction(ctx, s.b, tx)
}
