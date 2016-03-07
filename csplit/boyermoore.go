package main

var BuffSize = 7

func main() {
	pattern := "eric"
	test := "america"
}

func preBad(x string, m int, badChar []int) {
	for i := 0; i < BuffSize; i++ {
		badChar[i] = m
	}

	for i := 0; i < m-1; i++ {
		badChar[x[i]] = m - i - 1
	}
}

func suffix(x string, m int, suff []int) {
	var f int

	suff[m-1] = m
	g := m - 1

	for i := m - 2; i >= 0; i-- {
		if i > g && suff[i+m-1-f] < i-g {
			suff[i] = suff[i+m-1-f]
		} else {
			if i > g {
				g = i
			}
			f = i
			for g >= 0 && x[g] == x[g+m-1-f] {
				g--
			}
			suff[i] = f - g
		}
	}
}

func preGood(x string, m int, goodChar []int) {
	suff := make([]int, BuffSize)
	suffix(x, m, suff)
}
