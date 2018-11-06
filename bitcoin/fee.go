package bitcoin

import (
	"errors"
	"github.com/btcsuite/btcutil"
)

const MinimalFeeRate = 1000 // 1 satoshi per byte
var MinimalFeeRateBTC = btcutil.Amount(MinimalFeeRate).ToBTC()

type FeeType int

const (
	FixedFee FeeType = iota
	PerKBRateFee
	InvalidFee
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
	if ok {
		return feeTypeStr
	}
	return "invalid"
}

func FeeTypeFromString(feeTypeStr string) (FeeType, error) {
	ft, ok := stringToFeeTypeMap[feeTypeStr]

	if ok {
		return ft, nil
	}
	return InvalidFee, errors.New(
		"Failed to convert string '" + feeTypeStr + "' to fee type",
	)
}
