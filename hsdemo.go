package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"

	"github.com/pterm/pterm"
	"github.com/pterm/pterm/putils"
)

type LoadBalancerStatus struct {
	Ingress []struct {
		IP       string `json:"ip"`
		Hostname string `json:"hostname"`
	} `json:"ingress"`
}

func main() {
	/* Logo */
	pterm.DefaultBigText.WithLetters(putils.LettersFromString("hsdemo")).Srender()
	pterm.DefaultHeader.Println("HS Demo Tool 1.6 - jamboyki@cisco.com - Non-production demo use only")
	pterm.DefaultSection.Println("Tools")

	/* Params */
	ciliumFlag := flag.Bool("cilium", true, "use cilium else default eks cni will be used")
	agentFlag := flag.Bool("tesseract", true, "use tesseract else tetragon will be used")
	ccFlag := flag.Bool("createcluster", true, "will create eks cluster if it does not exist")
	clusterNamePtr := flag.String("clustername", "hsdemo-cluster", "eks cluster name")
	interactiveFlag := flag.Bool("interactive", true, "will prompt user for options otherwise will use defaults where possible or fail")
	tsaRegPtr := flag.String("tsareg", "654654525765.dkr.ecr.us-east-2.amazonaws.com", "TSA container registry url. Can also be overridden by env var HYPERSHIELD_TSA_REGISTRY")
	tsaReEmailPtr := flag.String("tsaregemail", "", "TSA registry email. Can also be set by env var HYPERSHIELD_TSA_REGISTRY_EMAIL.")
	flag.Parse()

	/* Prereqs */
	//dependencies
	requiredApps := []string{"aws", "kubectl", "eksdemo"}
	for _, value := range requiredApps {
		if isAppInstalled(value) {
			fmt.Println(pterm.Green("\u2713 "), value)
		} else {
			fmt.Println(pterm.Red("\u274C "), value)
			fmt.Println("Try running brew reinstall hsdemo or find a way to install the avove required missing application")
			os.Exit(1)
		}
	}

	//env and tokens
	pterm.DefaultSection.Println("Local Environment")

	//If using TSA and not Tetragon
	tsa_reg := ""
	tsa_registry_cred := ""
	tsa_registry_email := ""
	SCC_API_TOKEN := ""

	if *agentFlag {
		//scc token
		value, exists := os.LookupEnv("SCC_API_TOKEN")
		if exists && value != "" {
			SCC_API_TOKEN = value
		} else if !*interactiveFlag {
			fmt.Println(pterm.Red("\u274C "), "You must set environmental variable SCC_API_TOKEN or use interactive mode")
			os.Exit(1)
		} else {
			println("Try going to https://us.manage.security.cisco.com/settings?selectedTab=user_management to create an API user and copy the token")
			result, _ := pterm.DefaultInteractiveTextInput.WithMultiLine().Show("Paste your SCC API TOKEN (or set env SCC_API_TOKEN)")
			SCC_API_TOKEN = result
		}
		fmt.Println(pterm.Green("\u2713 "), "SCC API Token: ", SCC_API_TOKEN)

		//tsa registry
		value, exists = os.LookupEnv("HYPERSHIELD_TSA_REGISTRY")
		if exists && value != "" {
			tsa_reg = value
		} else {
			tsa_reg = *tsaRegPtr
		}
		fmt.Println(pterm.Green("\u2713 "), "TSA Registry:", tsa_reg)

		//registry credential
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
				os.Exit(1)
			}
			tsa_registry_cred = string(output)
		}
		fmt.Println(pterm.Green("\u2713 "), "TSA Registry Credential:", tsa_registry_cred)

		//email
		value, exists = os.LookupEnv("HYPERSHIELD_TSA_REGISTRY_EMAIL")
		if *tsaReEmailPtr != "" {
			tsa_registry_email = *tsaRegPtr
		} else if exists && value != "" {
			tsa_registry_email = value
		} else if !*interactiveFlag {
			fmt.Println(pterm.Red("\u274C "), "You must set environmental variable HYPERSHIELD_TSA_REGISTRY_EMAIL or use interactive mode")
			os.Exit(1)
		} else {
			tsa_registry_email, _ = pterm.DefaultInteractiveTextInput.Show("Enter email address with tsa registry access: ")
		}
		fmt.Println(pterm.Green("\u2713 "), "TSA Registry Email:", tsa_registry_email)

		pterm.DefaultSection.Println("TSA Access")
		result := execute("Logging in TSA regsitry", nil, "helm", "registry", "login", "--username", "AWS", "--password", tsa_registry_cred, tsa_reg)
		if result != nil {
			fmt.Println(pterm.Red("TSA Registry Access Failure"))
			os.Exit(1)
		}

	} //end tsa only

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

	//cluster name
	clusterName := ""
	if *clusterNamePtr != "" {
		clusterName = *clusterNamePtr
	} else if *interactiveFlag {
		clusterName, _ = pterm.DefaultInteractiveTextInput.WithDefaultText("Cluster Name").WithDefaultValue("hsdemo-cluster").Show()
	} else {
		clusterName = "hsdemo-cluster"
		fmt.Printf("Non-interactive mode using default clustername hsdemo-cluster")
	}

	cmd = exec.Command("eksdemo", "get", "cluster", clusterName)
	output, err = cmd.Output()

	/* Deploy */
	//create cluster
	createNewCluster := false
	if err != nil && *ccFlag {
		fmt.Printf("No existing %s found in current active aws profile. Cluster will be created. If this is not expected cancel this program and run aws configure --profile PROFILENAME \n", clusterName)
		createNewCluster = true
	} else if err != nil && !*ccFlag {
		fmt.Println(string(output))
		fmt.Printf("No existing %s found in current active aws profile. Cluster will be created. Maybe run aws configure --profile PROFILENAME \n", clusterName)
		fmt.Println(pterm.Red("No cluster to work with and you specified not to create a new cluster"))
		os.Exit(1)
	} else {
		fmt.Printf("Using existing cluster %s found in current active aws profile.\n", clusterName)
	}

	if createNewCluster && *interactiveFlag {
		fmt.Println(pterm.Red("WARNING:"), "Continuing will create a new cluster and related resources in the above AWS account. Please ensure this is a non-production demo system. This process will take approximately 20 minutes.")
		prompt := pterm.DefaultInteractiveConfirm
		result, _ := prompt.Show("Do you wish to continue?")
		if !result {
			return
		}
		errored := execute("creating cluster", nil, "eksdemo", "create", "cluster", clusterName, "--os", "Ubuntu2004", "--version", "1.29")
		if errored != nil {
			fmt.Println("Aborting the remaining steps because the cluster creation failed! :( ")
			os.Exit(1)
		}
	}

	//cilium
	if *ciliumFlag {
		execute("installing cilium", nil, "eksdemo", "install", "cilium", "--cluster", clusterName)
	}

	//tsa or tetragon
	if *agentFlag {
		//tsa
		execute("deploying registry secrets", nil, "kubectl", "create", "secret", "docker-registry", "hypershield-tsa-registry",
			"--namespace", "kube-system",
			"--docker-server", tsa_reg,
			"--docker-username", "AWS",
			"--docker-password", tsa_registry_cred,
			"--docker-email", tsa_registry_email)

		execute("deploying TSA", nil, "helm", "install", "hypershield-tsa", "oci://"+tsa_reg+"/charts/hypershield-tsa", "--namespace", "kube-system", "--set", "apiTokenSecret="+SCC_API_TOKEN, "--version", "1.6.0",
			"--set", "tetragon.imagePullPolicy=Always",
			"--set", "tetragon.imagePullSecrets[0].name=hypershield-tsa-registry")

	} else {
		//tetragon
		execute("deploying Tetragon", nil, "helm", "install", "tetragon", "cilium/tetragon", "--namespace", "kube-system")
	}

	//storage
	execute("installing storage driver", nil, "eksdemo", "install", "storage-ebs-csi", "-c", clusterName)
	execute("annotating storage", nil, "kubectl", "annotate", "storageclass", "gp2", "storageclass.kubernetes.io/is-default-class=true")

	//splunk
	execute("deploying splunk operator", nil, "kubectl", "apply", "--server-side", "--force-conflicts", "-f", "https://github.com/splunk/splunk-operator/releases/download/2.7.0/splunk-operator-namespace.yaml")

	yamlContent := `
  apiVersion: enterprise.splunk.com/v4
  kind: Standalone
  metadata:
    name: s1
    finalizers:
    - enterprise.splunk.com/delete-pvc`
	execute("deploy splunk instance", &yamlContent, "kubectl", "apply", "--namespace=splunk-operator", "-f", "-")

	cmd = exec.Command("kubectl", "get", "secrets", "--namespace=splunk-operator", "splunk-s1-standalone-secret-v1", "--output", "json")
	output, err = cmd.Output()
	if err != nil {
		fmt.Printf("Error executing command: %s\n", err)
	}
	decodedOutput, err := base64.StdEncoding.DecodeString(string(output))
	if err != nil {
		fmt.Printf("Error decoding base64: %s\n", err)
	} else {
		// Print the decoded output
		fmt.Println(string(decodedOutput))
	}

	execute("Wait for splunk rollout", nil, "kubectl", "rollout", "status", "-w", "--namespace=splunk-operator",
		"--timeout=180s", "deployment/splunk-operator-controller-manager")

	execute("Wait for splunk pod", nil, "kubectl", "wait", "--for=condition=ready", "pod/splunk-s1-standalone-0", "--namespace=splunk-operator",
		"--timeout=180s")

	//loadbalancer
	execute("create loadbalancer", nil, "kubectl", "expose", "pod", "splunk-s1-standalone-0",
		"--type=LoadBalancer", "--port=80", "--target-port=8000",
		"--name=splunk-lb",
		"--namespace=splunk-operator")
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
		area.Update("                           ", line)
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
