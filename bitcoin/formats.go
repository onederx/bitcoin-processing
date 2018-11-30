package bitcoin

import (
	"github.com/shopspring/decimal"
	"log"
)

type BitcoinAmount uint64

var satoshiInBTCDecimal = decimal.New(1, 8)

func (amount BitcoinAmount) ToStringedFloat() string {
	return decimal.New(int64(amount), -8).String()
}

func (amount BitcoinAmount) Float64() float64 {
	amountFloat, exact := decimal.New(int64(amount), -8).Float64()
	if !exact {
		log.Printf(
			"WARNING: non-exact conversion from BitcoinAmount to float64."+
				"BitcoinAmount is %s, float amount is %f",
			amount,
			amountFloat,
		)
	}
	return amountFloat
}

func (amount BitcoinAmount) ToBTC() float64 {
	return amount.Float64()
}

func (amount BitcoinAmount) String() string {
	return amount.ToStringedFloat()
}

func (amount BitcoinAmount) MarshalJSON() ([]byte, error) {
	return []byte("\"" + amount.ToStringedFloat() + "\""), nil
}

func BitcoinAmountFromStringedFloat(amountSF string) (BitcoinAmount, error) {
	amountDecimal, err := decimal.NewFromString(amountSF)
	if err != nil {
		return 0, err
	}
	return BitcoinAmount(amountDecimal.Mul(satoshiInBTCDecimal).IntPart()), nil
}
