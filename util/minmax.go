package util

// Min returns minimum of two int numbers
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Max returns maximum of two int numbers
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Min64 returns minimum of two int64 numbers
func Min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// Max64 returns maximum of two int64 numbers
func Max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
