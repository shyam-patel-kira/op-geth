package types

import (
	"encoding/json"
	"math/big"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestTransactionConditionalCost(t *testing.T) {
	uint64Ptr := func(num uint64) *uint64 {
		return &num
	}

	tests := []struct {
		name string
		cond TransactionConditional
		cost int
	}{
		{
			name: "empty conditional",
			cond: TransactionConditional{},
			cost: 0,
		},
		{
			name: "block number lookup counts once",
			cond: TransactionConditional{BlockNumberMin: big.NewInt(1), BlockNumberMax: big.NewInt(2)},
			cost: 1,
		},
		{
			name: "timestamp lookup counts once",
			cond: TransactionConditional{TimestampMin: uint64Ptr(0), TimestampMax: uint64Ptr(5)},
			cost: 1,
		},
		{
			name: "storage root lookup",
			cond: TransactionConditional{KnownAccounts: map[common.Address]KnownAccount{
				common.Address{19: 1}: KnownAccount{
					StorageRoot: &EmptyRootHash,
				}}},
			cost: 1,
		},
		{
			name: "cost per storage slot lookup",
			cond: TransactionConditional{KnownAccounts: map[common.Address]KnownAccount{
				common.Address{19: 1}: KnownAccount{
					StorageSlots: map[common.Hash]common.Hash{
						common.Hash{}:      common.Hash{31: 1},
						common.Hash{31: 1}: common.Hash{31: 1},
					},
				}}},
			cost: 2,
		},
		{
			name: "cost summed together",
			cond: TransactionConditional{
				BlockNumberMin: big.NewInt(1),
				TimestampMin:   uint64Ptr(1),
				KnownAccounts: map[common.Address]KnownAccount{
					common.Address{19: 1}: KnownAccount{StorageRoot: &EmptyRootHash},
					common.Address{19: 2}: KnownAccount{
						StorageSlots: map[common.Hash]common.Hash{
							common.Hash{}:      common.Hash{31: 1},
							common.Hash{31: 1}: common.Hash{31: 1},
						},
					}}},
			cost: 5,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cost := test.cond.Cost()
			if cost != test.cost {
				t.Errorf("Test %s mismatch in TransactionConditional cost. Got %d, Expected: %d", test.name, cost, test.cost)
			}
		})
	}
}

func TestTransactionConditionalUnmarshalJSON(t *testing.T) {
	uint64Ptr := func(num uint64) *uint64 {
		return &num
	}
	hashPtr := func(hash common.Hash) *common.Hash {
		return &hash
	}

	tests := []struct {
		name     string
		input    string
		mustFail bool
		expected TransactionConditional
	}{
		{
			name:     "StateRoot",
			input:    `{"knownAccounts":{"0x6b3A8798E5Fb9fC5603F3aB5eA2e8136694e55d0":"0x290decd9548b62a8d60345a988386fc84ba6bc95484008f6362f93160ef3e563"}}`,
			mustFail: false,
			expected: TransactionConditional{
				KnownAccounts: map[common.Address]KnownAccount{
					common.HexToAddress("0x6b3A8798E5Fb9fC5603F3aB5eA2e8136694e55d0"): KnownAccount{
						StorageRoot:  hashPtr(common.HexToHash("0x290decd9548b62a8d60345a988386fc84ba6bc95484008f6362f93160ef3e563")),
						StorageSlots: make(map[common.Hash]common.Hash),
					},
				},
			},
		},
		{
			name:     "StorageSlots",
			input:    `{"knownAccounts":{"0x6b3A8798E5Fb9fC5603F3aB5eA2e8136694e55d0":{"0xc65a7bb8d6351c1cf70c95a316cc6a92839c986682d98bc35f958f4883f9d2a8":"0x0000000000000000000000000000000000000000000000000000000000000000"}}}`,
			mustFail: false,
			expected: TransactionConditional{
				KnownAccounts: map[common.Address]KnownAccount{
					common.HexToAddress("0x6b3A8798E5Fb9fC5603F3aB5eA2e8136694e55d0"): KnownAccount{
						StorageRoot: nil,
						StorageSlots: map[common.Hash]common.Hash{
							common.HexToHash("0xc65a7bb8d6351c1cf70c95a316cc6a92839c986682d98bc35f958f4883f9d2a8"): common.HexToHash("0x"),
						},
					},
				},
			},
		},
		{
			name:     "EmptyObject",
			input:    `{"knownAccounts":{}}`,
			mustFail: false,
			expected: TransactionConditional{
				KnownAccounts: make(map[common.Address]KnownAccount),
			},
		},
		{
			name:     "EmptyStrings",
			input:    `{"knownAccounts":{"":""}}`,
			mustFail: true,
			expected: TransactionConditional{
				KnownAccounts: nil,
			},
		},
		{
			name:     "BlockNumberMin",
			input:    `{"blockNumberMin":"0x1"}`,
			mustFail: false,
			expected: TransactionConditional{
				BlockNumberMin: big.NewInt(1),
			},
		},
		{
			name:     "BlockNumberMax",
			input:    `{"blockNumberMin":"0x1", "blockNumberMax":"0x2"}`,
			mustFail: false,
			expected: TransactionConditional{
				BlockNumberMin: big.NewInt(1),
				BlockNumberMax: big.NewInt(2),
			},
		},
		{
			name:     "TimestampMin",
			input:    `{"timestampMin":"0xffff"}`,
			mustFail: false,
			expected: TransactionConditional{
				TimestampMin: uint64Ptr(uint64(0xffff)),
			},
		},
		{
			name:     "TimestampMax",
			input:    `{"timestampMax":"0xffffff"}`,
			mustFail: false,
			expected: TransactionConditional{
				TimestampMax: uint64Ptr(uint64(0xffffff)),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var opts TransactionConditional
			err := json.Unmarshal([]byte(test.input), &opts)
			if test.mustFail && err == nil {
				t.Errorf("Test %s should fail", test.name)
			}
			if !test.mustFail && err != nil {
				t.Errorf("Test %s should pass but got err: %v", test.name, err)
			}

			if !reflect.DeepEqual(opts, test.expected) {
				t.Errorf("Test %s got unexpected value, want %#v, got %#v", test.name, test.expected, opts)
			}
		})
	}
}
