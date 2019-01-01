package bitcoin

import (
	"encoding/json"
	"errors"

	"github.com/btcsuite/btcutil"
)

// MinimalFeeRate is minimal fee rate per kilobyte that can be set in satoshis.
// Internally bitcoin node counts fee in satoshis per byte and minimal amount
// of BTC is 1 satoshi, so for 1000 bytes minimal rate is 1000 satoshis
const MinimalFeeRate = 1000 // 1 satoshi per byte

// MinimalFeeRateBTC is mimimal fee rate per kilobyte in BTC
var MinimalFeeRateBTC = btcutil.Amount(MinimalFeeRate).ToBTC()

// FeeType is an enum for setting how fee should be calculated for outgoing
// transactions (withdrawals). There are two possible types: rate per kilobyte
// (which is traditional and used by Bitcoin Core by default, with it given
// fee value will be multiplied by tx size in kilobytes) and fixed (resulting
// fee value will be equal to given fee value)
type FeeType int

// Possible fee types.
// PerKBRateFee means fee paid will equal given fee value times tx size in KB
// FixedFee means fee paid will be exatly equal to given fee value
// InvalidFee means fee type is invalid and is used for unknown, uninitialized
// values, and conversions from other types when source value is invalid
const (
	InvalidFee FeeType = iota
	PerKBRateFee
	FixedFee
)

var feeTypeToStringMap = map[FeeType]string{
	FixedFee:     "fixed",
	PerKBRateFee: "per-kb-rate",
	InvalidFee:   "invalid",
}

var stringToFeeTypeMap = make(map[string]FeeType)

func init() {
	for feeType, feeTypeStr := range feeTypeToStringMap {
		stringToFeeTypeMap[feeTypeStr] = feeType
	}
}

func (ft FeeType) String() string {
	feeTypeStr, ok := feeTypeToStringMap[ft]
	if !ok {
		return "invalid"
	}
	return feeTypeStr
}

// FeeTypeFromString converts string to FeeType. "fixed" is converted to
// FixedFee, "per-kb-rate" is converted to PerKBRateFee
// Value "invalid" is converted to InvalidFee without producing an error
// because InvalidFee is used in normal conditions when we don't know real
// fee type (for incoming payments for example)
// all other values produce InvalidFee and an error
func FeeTypeFromString(feeTypeStr string) (FeeType, error) {
	ft, ok := stringToFeeTypeMap[feeTypeStr]
	if !ok {
		return InvalidFee, errors.New(
			"Failed to convert string '" + feeTypeStr + "' to fee type",
		)
	}
	return ft, nil
}

// MarshalJSON is use to serialize FeeType to JSON and simply returns string
// representation of given FeeType
func (ft FeeType) MarshalJSON() ([]byte, error) {
	return []byte("\"" + ft.String() + "\""), nil
}

// UnmarshalJSON deserializes FeeType from JSON. Resulting value is
// mapped from string representation of fee type
func (ft *FeeType) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	*ft, err = FeeTypeFromString(j)
	return err
}
