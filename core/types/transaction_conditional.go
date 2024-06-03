package types

import (
	"encoding/json"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// KnownAccounts represents a set of knownAccounts
type KnownAccounts map[common.Address]KnownAccount

// KnownAccount allows for a user to express their preference of a known
// prestate at a particular account. Only one of the storage root or
// storage slots is allowed to be set. If the storage root is set, then
// the user prefers their transaction to only be included in a block if
// the account's storage root matches. If the storage slots are set,
// then the user prefers their transaction to only be included if the
// particular storage slot values from state match.
type KnownAccount struct {
	StorageRoot  *common.Hash
	StorageSlots map[common.Hash]common.Hash
}

// UnmarshalJSON will parse the JSON bytes into a KnownAccount struct.
func (ka *KnownAccount) UnmarshalJSON(data []byte) error {
	var hash common.Hash
	if err := json.Unmarshal(data, &hash); err == nil {
		ka.StorageRoot = &hash
		ka.StorageSlots = make(map[common.Hash]common.Hash)
		return nil
	}

	var mapping map[common.Hash]common.Hash
	if err := json.Unmarshal(data, &mapping); err != nil {
		return err
	}
	ka.StorageSlots = mapping

	return nil
}

// MarshalJSON will serialize the KnownAccount into JSON bytes.
func (ka *KnownAccount) MarshalJSON() ([]byte, error) {
	if ka.StorageRoot != nil {
		return json.Marshal(ka.StorageRoot)
	}
	return json.Marshal(ka.StorageSlots)
}

// Copy will copy the KnownAccount
func (ka *KnownAccount) Copy() KnownAccount {
	cpy := KnownAccount{
		StorageRoot:  nil,
		StorageSlots: make(map[common.Hash]common.Hash),
	}

	if ka.StorageRoot != nil {
		*cpy.StorageRoot = *ka.StorageRoot
	}
	for key, val := range ka.StorageSlots {
		cpy.StorageSlots[key] = val
	}
	return cpy
}

// Root will return the storage root and true when the user prefers
// execution against an account's storage root, otherwise it will
// return false.
func (ka *KnownAccount) Root() (common.Hash, bool) {
	if ka.StorageRoot == nil {
		return common.Hash{}, false
	}
	return *ka.StorageRoot, true
}

// Slots will return the storage slots and true when the user prefers
// execution against an account's particular storage slots, otherwise
// it will return false.
func (ka *KnownAccount) Slots() (map[common.Hash]common.Hash, bool) {
	if ka.StorageRoot != nil {
		return ka.StorageSlots, false
	}
	return ka.StorageSlots, true
}

//go:generate go run github.com/fjl/gencodec -type TransactionConditional -field-override transactionConditionalMarshalling -out gen_transaction_conditional_json.go

// TransactionConditional represents the preconditions that determine
// the inclusion of the transaction, enforced out-of-protocol by the
// sequencer.
type TransactionConditional struct {
	// KnownAccounts represents a user's preference of a known
	// prestate before their transaction is included.
	KnownAccounts KnownAccounts `json:"knownAccounts"`

	// Header state conditionals
	BlockNumberMin *big.Int `json:"blockNumberMin,omitempty"`
	BlockNumberMax *big.Int `json:"blockNumberMax,omitempty"`
	TimestampMin   *uint64  `json:"timestampMin,omitempty"`
	TimestampMax   *uint64  `json:"timestampMax,omitempty"`

	// Tracked internally for metrics purposes
	submissionTime time.Time `json:"-"`
}

// field type overrides for gencodec
type transactionConditionalMarshalling struct {
	BlockNumberMax *hexutil.Big
	BlockNumberMin *hexutil.Big
	TimestampMin   *hexutil.Uint64
	TimestampMax   *hexutil.Uint64
}

// Cost computes the cost of validating the TxOptions. It will return
// the number of storage lookups required by KnownAccounts.
func (opts *TransactionConditional) Cost() int {
	cost := 0
	for _, account := range opts.KnownAccounts {
		if _, isRoot := account.Root(); isRoot {
			cost += 1
		}
		if slots, isSlots := account.Slots(); isSlots {
			cost += len(slots)
		}
	}
	if opts.BlockNumberMin != nil || opts.BlockNumberMax != nil {
		cost += 1
	}
	if opts.TimestampMin != nil || opts.TimestampMax != nil {
		cost += 1
	}
	return cost
}

// Copy will copy the TransactionConditional
func (opts *TransactionConditional) Copy() TransactionConditional {
	cpy := TransactionConditional{
		KnownAccounts: make(map[common.Address]KnownAccount),
	}

	for key, val := range opts.KnownAccounts {
		cpy.KnownAccounts[key] = val.Copy()
	}
	if opts.BlockNumberMin != nil {
		*cpy.BlockNumberMin = *opts.BlockNumberMin
	}
	if opts.BlockNumberMax != nil {
		*cpy.BlockNumberMax = *opts.BlockNumberMax
	}
	if opts.TimestampMin != nil {
		*cpy.TimestampMin = *opts.TimestampMin
	}
	if opts.TimestampMax != nil {
		*cpy.TimestampMax = *opts.TimestampMax
	}
	return cpy
}

func (opts *TransactionConditional) SubmissionTime() time.Time {
	return opts.submissionTime
}

func (opts *TransactionConditional) SetSubmissionTime(t time.Time) {
	opts.submissionTime = t
}
