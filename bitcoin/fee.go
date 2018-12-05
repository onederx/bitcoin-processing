package bitcoin

import (
	"errors"

	"github.com/btcsuite/btcutil"
)

const MinimalFeeRate = 1000 // 1 satoshi per byte
var MinimalFeeRateBTC = btcutil.Amount(MinimalFeeRate).ToBTC()

type FeeType int

const (
	InvalidFee FeeType = iota
	PerKBRateFee
	FixedFee
)

var feeTypeToStringMap map[FeeType]string = map[FeeType]string{
	FixedFee:     "fixed",
	PerKBRateFee: "per-kb-rate",
}

var stringToFeeTypeMap map[string]FeeType = make(map[string]FeeType)

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

func FeeTypeFromString(feeTypeStr string) (FeeType, error) {
	ft, ok := stringToFeeTypeMap[feeTypeStr]
	if !ok {
		return InvalidFee, errors.New(
			"Failed to convert string '" + feeTypeStr + "' to fee type",
		)
	}
	return ft, nil
}

func (ft FeeType) MarshalJSON() ([]byte, error) {
	return []byte("\"" + ft.String() + "\""), nil
}
