package main

import (
	"fmt"
)

func main() {
	data := getPrivData()
	var validStr = "valid"
	if valid := checkValidData(data); !valid {
		validStr = "not valid"
	}
	fmt.Printf("privateData is: %x (%s)\n", data, validStr)
}
