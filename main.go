package main

import (
	"fmt"
)

func main() {
	var name string
	var age int

	// Prompt for the user's name
	fmt.Print("Enter your name: ")
	fmt.Scanln(&name)

	// Prompt for the user's age
	fmt.Print("Enter your age: ")
	fmt.Scanln(&age)

	// Print a greeting message
	fmt.Printf("Hello, %s! You are %d years old.\n", name, age)
}
