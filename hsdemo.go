package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"

	"github.com/pterm/pterm"
	"github.com/pterm/pterm/putils"
)

func main() {
	pterm.DefaultBigText.WithLetters(putils.LettersFromString("hsdemo")).Srender()
	pterm.DefaultHeader.Println("HS Demo Tool - jamboyki@cisco.com - Non-production demo use only")
	pterm.DefaultSection.Println("Tools")

	prereqs := true

	requiredApps := []string{"aws", "kubectl", "eksdemo"}
	for _, value := range requiredApps {
		if isAppInstalled(value) {
			fmt.Println(pterm.Green("\u2713 "), value)
		} else {
			fmt.Println(pterm.Red("\u274C "), value)
			fmt.Println("Try running brew reinstall hsdemo or find a way to install the avove required missing application")
		}
	}

	pterm.DefaultSection.Println("Local Environment")
	//registry
	tsa_registry := "654654525765.dkr.ecr.us-east-2.amazonaws.com"
	value, exists := os.LookupEnv("HYPERSHIELD_TSA_REGISTRY")
	if exists && value != "" {
		tsa_registry = value
	}

	fmt.Println(pterm.Green("\u2713 "), "TSA Registry:", tsa_registry)

	//registry credential
	tsa_registry_cred := ""
	value, exists = os.LookupEnv("HYPERSHIELD_TSA_REGISTRY_CREDENTIAL")
	if exists && value != "" {
		tsa_registry_cred = value
	} else {
		cmd := exec.Command("aws", "--region", "us-east-2", "ecr", "get-login-password")
		output, err := cmd.Output()
		if err != nil {
			fmt.Println(pterm.Red("\u274C "), "TSA Registry Credential: Not Set")
			fmt.Println("Unable to get tsa registry credential: ", output)
			fmt.Println("Unable to get tsa registry credential: ", err)
			fmt.Println("Make sure you are logged in using AWS CLI or make sure the environmental variable HYPERSHIELD_TSA_REGISTRY_CREDENTIAL is set.")
			prereqs = false
		}
		tsa_registry_cred = string(output)
	}
	fmt.Println(pterm.Green("\u2713 "), "TSA Registry Credential:", tsa_registry_cred)

	//email
	tsa_registry_email := ""
	value, exists = os.LookupEnv("HYPERSHIELD_TSA_REGISTRY_EMAIL")
	if exists && value != "" {
		tsa_registry_email = value
	} else {
		tsa_registry_email, _ = pterm.DefaultInteractiveTextInput.Show("Enter email address with tsa registry access: ")
	}
	fmt.Println(pterm.Green("\u2713 "), "TSA Registry Email:", tsa_registry_email)

	if !prereqs {
		pterm.Println("Unable to proceed until the above issues are fixed.")
		return
	}

	pterm.DefaultSection.Println("AWS Environment")
	// Command to get the current AWS user
	cmd := exec.Command("aws", "sts", "get-caller-identity")

	// Run the command and capture the output
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error executing command: %s\n", err)
		return
	}
	fmt.Println(string(output))

	cmd = exec.Command("eksdemo", "get", "cluster", "hsdemo-cluster")
	output, err = cmd.Output()
	createNewCluster := false
	if err != nil {
		fmt.Println("No existing hsdemo-cluster found in current active aws profile. If this is not expected cancel this program and run aws configure --profile PROFILENAME")
		createNewCluster = true
	} else {
		fmt.Println(string(output))
	}

	// Print two new lines as spacer.
	if createNewCluster {
		fmt.Println(pterm.Red("WARNING:"), "Contininuing will create a new cluster and related resources in the above AWS account. Please ensure this is a non-production demo system. This process will take approximately 20 minutes.")
		prompt := pterm.DefaultInteractiveConfirm
		result, _ := prompt.Show("Do you wish to continue?")
		if !result {
			return
		}
		execute("creating cluster", "eksdemo", "create", "cluster", "hsdemo-cluster")
	}

	execute("installing storage driver", "eksdemo", "install", "storage-ebs-csi", "-c", "hsdemo-cluster")
	execute("annotating storage", "kubectl", "annotate", "storageclass", "gp2", `storageclass.kubernetes.io/is-default-class=“true”`)

}

func execute(description string, command string, args ...string) error {
	spinner, _ := pterm.DefaultSpinner.Start(description)
	area, _ := pterm.DefaultArea.Start()
	defer area.Stop()

	cmd := exec.Command(command, args...)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		log.Fatal(err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		spinner.Fail(description + " " + err.Error())
		return err
	}

	combinedReader := io.MultiReader(stdoutPipe, stderrPipe)

	// Create a scanner to read the output line by line
	scanner := bufio.NewScanner(combinedReader)
	output := ""
	for scanner.Scan() {
		line := scanner.Text()
		area.Update("                      ", line)
		output = output + line + "\n"
	}

	// Check for errors during scanning
	if err := scanner.Err(); err != nil {
		fmt.Println(output)
		spinner.Fail(description + " " + err.Error())
		return err
	}

	// Wait for the command to finish
	if err := cmd.Wait(); err != nil {
		fmt.Println(output)
		spinner.Fail(description + " " + err.Error())
		return err
	}

	spinner.Success(description)
	return nil
}

func isAppInstalled(appName string) bool {
	_, err := exec.LookPath(appName)
	return err == nil
}
