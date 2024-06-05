// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package legacypool

import (
	"math/big"
	"math/rand"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
)

// Tests that transactions can be added to strict lists and list contents and
// nonce boundaries are correctly maintained.
func TestStrictListAdd(t *testing.T) {
	// Generate a list of transactions to insert
	key, _ := crypto.GenerateKey()

	txs := make(types.Transactions, 1024)
	for i := 0; i < len(txs); i++ {
		txs[i] = transaction(uint64(i), 0, key)
	}
	// Insert the transactions in a random order
	list := newList(true)
	for _, v := range rand.Perm(len(txs)) {
		list.Add(txs[v], DefaultConfig.PriceBump, nil)
	}
	// Verify internal state
	if len(list.txs.items) != len(txs) {
		t.Errorf("transaction count mismatch: have %d, want %d", len(list.txs.items), len(txs))
	}
	for i, tx := range txs {
		if list.txs.items[tx.Nonce()] != tx {
			t.Errorf("item %d: transaction mismatch: have %v, want %v", i, list.txs.items[tx.Nonce()], tx)
		}
	}
}

// TestListAddVeryExpensive tests adding txs which exceed 256 bits in cost. It is
// expected that the list does not panic.
func TestListAddVeryExpensive(t *testing.T) {
	key, _ := crypto.GenerateKey()
	list := newList(true)
	for i := 0; i < 3; i++ {
		value := big.NewInt(100)
		gasprice, _ := new(big.Int).SetString("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", 0)
		gaslimit := uint64(i)
		tx, _ := types.SignTx(types.NewTransaction(uint64(i), common.Address{}, value, gaslimit, gasprice, nil), types.HomesteadSigner{}, key)
		t.Logf("cost: %x bitlen: %d\n", tx.Cost(), tx.Cost().BitLen())
		list.Add(tx, DefaultConfig.PriceBump, nil)
	}
}

// TestFilterTransactionConditionals tests filtering by invalid TransactionConditionals.
func TestFilterTransactionConditional(t *testing.T) {
	// Create an in memory state db to test against.
	state, _ := state.New(types.EmptyRootHash, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	state.IntermediateRoot(false)

	// Create a private key to sign transactions.
	key, _ := crypto.GenerateKey()

	// Create a list.
	list := newList(true)

	// Create a transaction with no defined conditional and add to the list.
	tx1 := transaction(0, 1000, key)
	list.Add(tx1, DefaultConfig.PriceBump, nil)

	// There should be no drops at this point, no conditional txs
	drops, err := list.FilterTransactionConditionals(state)
	if err != nil {
		t.Fatalf("error filtering by TransactionConditionals: %s", err)
	}
	if count := len(drops); count != 0 {
		t.Fatalf("got %d filtered by TransactionConditionals when there should not be any", count)
	}

	// Create another transaction with a conditional
	tx2 := transaction(1, 1000, key)
	tx2.SetConditional(&types.TransactionConditional{
		KnownAccounts: map[common.Address]types.KnownAccount{{19: 1}: {StorageRoot: &types.EmptyRootHash}},
	})
	list.Add(tx2, DefaultConfig.PriceBump, nil)

	// There should still be no drops as no state has been modified.
	drops, err = list.FilterTransactionConditionals(state)
	if err != nil {
		t.Fatalf("error filtering by TransactionConditionals: %s", err)
	}
	if count := len(drops); count != 0 {
		t.Fatalf("got %d filtered by TransactionConditionals when there should not be any", count)
	}

	// Set state that conflicts with tx2's conditional
	state.SetState(common.Address{19: 1}, common.Hash{}, common.Hash{31: 1})
	state.IntermediateRoot(false)

	// tx2 should be the single transaction filtered out
	drops, err = list.FilterTransactionConditionals(state)
	if err == nil {
		t.Fatalf("expected tx filtered by TransactionConditionals")
	}
	if count := len(drops); count != 1 {
		t.Fatalf("got %d filtered by TransactionConditionals when there should be a single one", count)
	}
	if drops[0] != tx2 {
		t.Fatalf("Got %x, expected %x", drops[0].Hash(), tx2.Hash())
	}
	if list.Len() != 1 {
		t.Fatal("expected only 1 transaction remaining in the list")
	}
}

func BenchmarkListAdd(b *testing.B) {
	// Generate a list of transactions to insert
	key, _ := crypto.GenerateKey()

	txs := make(types.Transactions, 100000)
	for i := 0; i < len(txs); i++ {
		txs[i] = transaction(uint64(i), 0, key)
	}
	// Insert the transactions in a random order
	priceLimit := uint256.NewInt(DefaultConfig.PriceLimit)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list := newList(true)
		for _, v := range rand.Perm(len(txs)) {
			list.Add(txs[v], DefaultConfig.PriceBump, nil)
			list.Filter(priceLimit, DefaultConfig.PriceBump)
		}
	}
}

func BenchmarkListCapOneTx(b *testing.B) {
	// Generate a list of transactions to insert
	key, _ := crypto.GenerateKey()

	txs := make(types.Transactions, 32)
	for i := 0; i < len(txs); i++ {
		txs[i] = transaction(uint64(i), 0, key)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list := newList(true)
		// Insert the transactions in a random order
		for _, v := range rand.Perm(len(txs)) {
			list.Add(txs[v], DefaultConfig.PriceBump, nil)
		}
		b.StartTimer()
		list.Cap(list.Len() - 1)
		b.StopTimer()
	}
}
