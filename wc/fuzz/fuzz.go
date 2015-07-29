package wc

func Fuzz(data []byte) int {
	*printLines = true
	*printWords = true
	*printChars = true
	*printBytes = true
	*printLineLength = true

	if err := DO(); err != nil {
		panic(err)
	}
	return 1
}

func main() {}
