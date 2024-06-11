package types

import (
	"encoding/json"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
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

func TestTransactionConditionalValidation(t *testing.T) {
	uint64Ptr := func(num uint64) *uint64 {
		return &num
	}

	tests := []struct {
		name     string
		cond     TransactionConditional
		mustFail bool
	}{
		{
			name:     "empty conditional",
			cond:     TransactionConditional{},
			mustFail: false,
		},
		{
			name:     "equal block constraint",
			cond:     TransactionConditional{BlockNumberMin: big.NewInt(1), BlockNumberMax: big.NewInt(1)},
			mustFail: false,
		},
		{
			name:     "block min greater than max",
			cond:     TransactionConditional{BlockNumberMin: big.NewInt(2), BlockNumberMax: big.NewInt(1)},
			mustFail: true,
		},
		{
			name:     "equal timestamp constraint",
			cond:     TransactionConditional{TimestampMin: uint64Ptr(1), TimestampMax: uint64Ptr(1)},
			mustFail: false,
		},
		{
			name:     "timestamp min greater than max",
			cond:     TransactionConditional{TimestampMin: uint64Ptr(2), TimestampMax: uint64Ptr(1)},
			mustFail: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.cond.Validate()
			if test.mustFail && err == nil {
				t.Errorf("Test %s should fail", test.name)
			}
			if !test.mustFail && err != nil {
				t.Errorf("Test %s should pass but got err: %v", test.name, err)
			}
		})
	}
}

func TestTransactionConditionalSerDeser(t *testing.T) {
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
		{
			name:     "SubmissionTime",
			input:    `{"submissionTime": 1234}`,
			mustFail: true,
			expected: TransactionConditional{
				KnownAccounts: nil,
			},
		},
		{
			name:     "UnknownField",
			input:    `{"foobarbaz": 1234}`,
			mustFail: true,
			expected: TransactionConditional{
				KnownAccounts: nil,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var cond TransactionConditional
			err := json.Unmarshal([]byte(test.input), &cond)
			if test.mustFail && err == nil {
				t.Errorf("Test %s should fail", test.name)
			}
			if !test.mustFail && err != nil {
				t.Errorf("Test %s should pass but got err: %v", test.name, err)
			}
			if !reflect.DeepEqual(cond, test.expected) {
				t.Errorf("Test %s got unexpected value, want %#v, got %#v", test.name, test.expected, cond)
			}

			// Also rlp encode & decode.
			if !test.mustFail {
				rlpBytes, err := rlp.EncodeToBytes(cond)
				if err != nil {
					t.Errorf("Test %s failed to rlp encode: %s", test.name, err)
				}

				var condCpy TransactionConditional
				if err := rlp.DecodeBytes(rlpBytes, &condCpy); err != nil {
					t.Errorf("Test %s failed to decode rlp bytes: %s", test.name, err)
				}
				if !reflect.DeepEqual(cond, test.expected) {
					t.Errorf("Test %s got unexpected value, want %#v, got %#v", test.name, test.expected, cond)
				}
			}
		})
	}

	t.Run("SubmissionTime RLP ser/deser", func(t *testing.T) {
		cond := TransactionConditional{SubmissionTime: time.Now()}
		rlpBytes, err := rlp.EncodeToBytes(cond)
		if err != nil {
			t.Errorf("Test with Submissiontime failed to rlp encode: %s", err)
		}

		var condCpy TransactionConditional
		if err := rlp.DecodeBytes(rlpBytes, &condCpy); err != nil {
			t.Errorf("Test with SubmissionTime failed to decode rlp bytes: %s", err)
		}

		if condCpy.SubmissionTime.Unix() != cond.SubmissionTime.Unix() {
			t.Errorf("SubmissionTime mismatch. Got %d, Expected: %d", condCpy.SubmissionTime.Unix(), cond.SubmissionTime.Unix())
		}
	})
}
