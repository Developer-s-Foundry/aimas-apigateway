package main

import "fmt"

func main() {
	sc, err := NewServiceConfig("aimas.yml", "")

	if err != nil {
		fmt.Println(err.Error())
	}

	fmt.Println(sc)
}
