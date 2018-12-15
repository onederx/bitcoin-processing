package bitcoin

import (
	"log"

	"github.com/shopspring/decimal"
)

// BTCAmount is a bitcoin processing app's own type for representing an amount
// of bitcoins and is internally an uint64 holding amount of satoshi so that
// precision is not lost due to used floating point number representation.
// It supports conversion to float64 (to interact with APIs that accept BTC
// amounts as floats), and to string. It is also JSON-able and resulting
// JSON value is a stringed float because by API convention all amounts of
// bitcoins are transfered to clients as stringed floats
type BTCAmount uint64

var satoshiInBTCDecimal = decimal.New(1, 8)

// ToStringedFloat converts BTCAmount to a string with float amount of BTC
// written in it. It is used to create API responses to client
// Library "github.com/shopspring/decimal" is used for convertion
func (amount BTCAmount) ToStringedFloat() string {
	return decimal.New(int64(amount), -8).String()
}

// Float64 converts BTCAmount to float64. It is used to pass amount to an API
// that accepts float64.
// Library "github.com/shopspring/decimal" is used for convertion
func (amount BTCAmount) Float64() float64 {
	amountFloat, exact := decimal.New(int64(amount), -8).Float64()
	if !exact {
		log.Printf(
			"WARNING: non-exact conversion from BTCAmount to float64."+
				"BTCAmount is %s, float amount is %f",
			amount,
			amountFloat,
		)
	}
	return amountFloat
}

// ToBTC returns amount as a floating-point number of BTC. It the same as
// .Float64()
func (amount BTCAmount) ToBTC() float64 {
	return amount.Float64()
}

func (amount BTCAmount) String() string {
	return amount.ToStringedFloat()
}

// MarshalJSON is used to serialize BTCAmount to JSON. Resulting JSON value
// is a string obtained by .ToStringedFloat()
func (amount BTCAmount) MarshalJSON() ([]byte, error) {
	return []byte("\"" + amount.ToStringedFloat() + "\""), nil
}

// BTCAmountFromFloat creates BTCAmount from floating-poing amount of BTC
func BTCAmountFromFloat(amountF64 float64) BTCAmount {
	return BTCAmount(
		decimal.NewFromFloat(amountF64).Mul(satoshiInBTCDecimal).IntPart(),
	)
}

// BTCAmountFromStringedFloat creates BTCAmount from stringed float and is used
// to read values from API requests
func BTCAmountFromStringedFloat(amountSF string) (BTCAmount, error) {
	amountDecimal, err := decimal.NewFromString(amountSF)
	if err != nil {
		return 0, err
	}
	return BTCAmount(amountDecimal.Mul(satoshiInBTCDecimal).IntPart()), nil
}
