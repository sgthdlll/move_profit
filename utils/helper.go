package utils

func InArrayString(val string, arr []string) bool {
	if len(arr) <= 0 {
		return false
	}

	for _, v := range arr {
		if v == val {
			return true
		}
	}

	return false
}

func InArray(val int, arr []int) bool {
	if len(arr) <= 0 {
		return false
	}

	for _, v := range arr {
		if v == val {
			return true
		}
	}

	return false
}
