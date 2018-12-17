package util

// Abs64 returns absolute value of given int64 number: that is, x if x >= 0 and
// -x otherwise
func Abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
