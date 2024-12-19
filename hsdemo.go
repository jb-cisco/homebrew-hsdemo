package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
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
	//CDO Token
	CDO_API_TOKEN := ""
	value, exists := os.LookupEnv("CDO_API_TOKEN")
	if exists && value != "" {
		CDO_API_TOKEN = value
	} else {
		result, _ := pterm.DefaultInteractiveTextInput.Show("Paste your SCC API TOKEN (or set env CDO_API_TOKEN)")
		CDO_API_TOKEN = result
	}

	if CDO_API_TOKEN == "" {
		fmt.Println(pterm.Red("\u274C "), "CDO API Token: Not Set")
		println("Try going to https://us.manage.security.cisco.com/settings?selectedTab=user_management to create an API token")
	} else {
		fmt.Println(pterm.Green("\u2713 "), "CDO API Token: ", CDO_API_TOKEN)
	}

	//registry
	tsa_registry := "654654525765.dkr.ecr.us-east-2.amazonaws.com"
	value, exists = os.LookupEnv("HYPERSHIELD_TSA_REGISTRY")
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

	pterm.DefaultSection.Println("TSA Access")
	result := execute("Logging in TSA regsitry", nil, "helm", "registry", "login", "--username", "AWS", "--password", tsa_registry_cred, tsa_registry)
	if result != nil {
		pterm.Println("Unable to proceed until the above issue is fixed.")
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
	if !createNewCluster {
		fmt.Println("Cluster already exist. More demo features coming soon. To delete the cluster run: eksdemo delete cluster hsdemo-cluster")
		fmt.Println(pterm.Red("WARNING:"), "Do you wish to re-run all setup commands on the existing hsdemo-cluster?")
		prompt := pterm.DefaultInteractiveConfirm
		result, _ := prompt.Show("Do you wish to continue?")
		if !result {
			return
		}
		execute("setting eksdemo and kubectl context", nil, "eksdemo", "use-context", "hsdemo-cluster")
	} else {
		fmt.Println(pterm.Red("WARNING:"), "Contininuing will create a new cluster and related resources in the above AWS account. Please ensure this is a non-production demo system. This process will take approximately 20 minutes.")
		prompt := pterm.DefaultInteractiveConfirm
		result, _ := prompt.Show("Do you wish to continue?")
		if !result {
			return
		}
		execute("creating cluster", nil, "eksdemo", "create", "cluster", "hsdemo-cluster")
	}

	execute("installing cilium", nil, "eksdemo", "install", "cilium", "--cluster", "hsdemo-cluster")
	execute("deploying registry secrets", nil, "kubectl", "create", "secret", "docker-registry", "hypershield-tsa-registry",
		"--namespace", "kube-system",
		"--docker-server", tsa_registry,
		"--docker-username", "AWS",
		"--docker-password", tsa_registry_cred,
		"--docker-email", tsa_registry_email)

	execute("deploying TSA", nil, "helm", "install", "hypershield-tsa", "oci://"+tsa_registry+"/charts/hypershield-tsa", "--namespace", "kube-system", "--set", "apiTokenSecret="+CDO_API_TOKEN, "--version", "1.3.4",
		"--set", "tetragon.imagePullPolicy=Always",
		"--set", "tetragon.imagePullSecrets[0].name=hypershield-tsa-registry")

	execute("installing storage driver", nil, "eksdemo", "install", "storage-ebs-csi", "-c", "hsdemo-cluster")
	execute("annotating storage", nil, "kubectl", "annotate", "storageclass", "gp2", "storageclass.kubernetes.io/is-default-class=true")
	execute("deploying splunk operator", nil, "kubectl", "apply", "--server-side", "--force-conflicts", "-f", "https://github.com/splunk/splunk-operator/releases/download/2.7.0/splunk-operator-namespace.yaml")

	// Define the YAML content
	yamlContent := `
  apiVersion: enterprise.splunk.com/v4
  kind: Standalone
  metadata:
    name: s1
    finalizers:
    - enterprise.splunk.com/delete-pvc`
	execute("deploy splunk instance", &yamlContent, "kubectl", "apply", "--namespace", "splunk-operator", "-f", "-")

	// Command to get the current AWS user
	cmd = exec.Command("kubectl", "get", "secrets", "-n", "splunk-operator", "splunk-s1-standalone-secret-v1", "--template={{index .data \"default.yml\"}}")

	// Run the command and capture the output
	output, err = cmd.Output()
	if err != nil {
		fmt.Printf("Error executing command: %s\n", err)
		return
	}
	// Decode the base64 output
	decodedOutput, err := base64.StdEncoding.DecodeString(string(output))
	if err != nil {
		fmt.Printf("Error decoding base64: %s\n", err)
		return
	}
	// Print the decoded output
	fmt.Println(string(decodedOutput))

	execute("create loadbalancer", nil, "kubectl", "expose", "deployment", "splunk-s1-standalone",
		"--type=LoadBalancer", "--port=80", "--target-port=8000",
		"--name=splunk-lb",
		"--selector=app.kubernetes.io/component=standalone,app.kubernetes.io/instance=splunk-s1-standalone,app.kubernetes.io/managed-by=splunk-operator,app.kubernetes.io/name=standalone,app.kubernetes.io/part-of=splunk-s1-standalone")

}

func execute(description string, input *string, command string, args ...string) error {
	spinner, _ := pterm.DefaultSpinner.Start(description)
	area, _ := pterm.DefaultArea.Start()
	defer area.Stop()

	cmd := exec.Command(command, args...)
	if input != nil {
		cmd.Stdin = bytes.NewBufferString(*input)
	}

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
