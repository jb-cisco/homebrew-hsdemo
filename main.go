package main

import (
	"fmt"
	"os/exec"
)

func main() {
	var name string
	var age int

	// Prompt for the user's name
	fmt.Print("Enter your name2: ")
	fmt.Scanln(&name)

	// Prompt for the user's age
	fmt.Print("Enter your age: ")
	fmt.Scanln(&age)

	// Print a greeting message
	fmt.Printf("Hello, %s! You are %d years old.\n", name, age)

	// Define the command to run
	cmd := exec.Command("eksctl", "delete", "cluster", "jimmy-aws-c1")

	// Run the command and capture the output
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If there is an error, print it and exit
		fmt.Printf("Error executing command: %s\n", err)
		return
	}

	// Print the output of the command
	fmt.Printf("Command output:\n%s\n", output)
}
