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

type LoadBalancerStatus struct {
	Ingress []struct {
		IP       string `json:"ip"`
		Hostname string `json:"hostname"`
	} `json:"ingress"`
}

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

	//cluster name
	clusterName, _ := pterm.DefaultInteractiveTextInput.WithDefaultText("Cluster Name").WithDefaultValue("hsdemo-cluster").Show()

	cmd = exec.Command("eksdemo", "get", "cluster", clusterName)
	output, err = cmd.Output()
	createNewCluster := false
	if err != nil {
		fmt.Printf("No existing %s found in current active aws profile. If this is not expected cancel this program and run aws configure --profile PROFILENAME \n", clusterName)
		createNewCluster = true
	} else {
		fmt.Println(string(output))
	}

	//CNI
	cniOptions := []string{"Cilium", "EKS Default"}
	cniChoice, _ := pterm.DefaultInteractiveSelect.WithDefaultText("CNI").WithOptions(cniOptions).Show()

	// Print two new lines as spacer.
	if !createNewCluster {
		fmt.Println("Cluster already exist. To delete the cluster exit and run: eksdemo delete cluster", clusterName)
		fmt.Println(pterm.Red("WARNING:"), "Do you wish to re-run all setup commands on the existing cluster?")
		prompt := pterm.DefaultInteractiveConfirm
		result, _ := prompt.Show("Do you wish to continue?")
		if !result {
			return
		}
		execute("setting eksdemo and kubectl context", nil, "eksdemo", "use-context", clusterName)
	} else {
		fmt.Println(pterm.Red("WARNING:"), "Continuing will create a new cluster and related resources in the above AWS account. Please ensure this is a non-production demo system. This process will take approximately 20 minutes.")
		prompt := pterm.DefaultInteractiveConfirm
		result, _ := prompt.Show("Do you wish to continue?")
		if !result {
			return
		}
		errored := execute("creating cluster", nil, "eksdemo", "create", "cluster", clusterName)
		if errored != nil {
			fmt.Println("Aborting the remaining steps because the cluster creation failed! :( ")
		}
	}

	if cniChoice == "Cilium" {
		execute("installing cilium", nil, "eksdemo", "install", "cilium", "--cluster", clusterName)
	}

	execute("deploying registry secrets", nil, "kubectl", "create", "secret", "docker-registry", "hypershield-tsa-registry",
		"--namespace", "kube-system",
		"--docker-server", tsa_registry,
		"--docker-username", "AWS",
		"--docker-password", tsa_registry_cred,
		"--docker-email", tsa_registry_email)

	execute("deploying TSA", nil, "helm", "install", "hypershield-tsa", "oci://"+tsa_registry+"/charts/hypershield-tsa", "--namespace", "kube-system", "--set", "apiTokenSecret="+CDO_API_TOKEN, "--version", "1.5.0",
		"--set", "tetragon.imagePullPolicy=Always",
		"--set", "tetragon.imagePullSecrets[0].name=hypershield-tsa-registry")

	execute("installing storage driver", nil, "eksdemo", "install", "storage-ebs-csi", "-c", clusterName)
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
	execute("deploy splunk instance", &yamlContent, "kubectl", "apply", "--namespace=splunk-operator", "-f", "-")

	// Command to get the current AWS user
	cmd = exec.Command("kubectl", "get", "secrets", "--namespace=splunk-operator", "splunk-s1-standalone-secret-v1", "--output", "json")

	// Run the command and capture the output
	output, err = cmd.Output()
	if err != nil {
		fmt.Printf("Error executing command: %s\n", err)
	}
	// Decode the base64 output
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

	execute("create loadbalancer", nil, "kubectl", "expose", "pod", "splunk-s1-standalone-0",
		"--type=LoadBalancer", "--port=80", "--target-port=8000",
		"--name=splunk-lb",
		"--namespace=splunk-operator")

	/*
		// Define the kubectl command to get the service details in JSON format
		cmd = exec.Command("kubectl", "get", "svc", "splunk-lb", "-o", "json")

		// Run the command and capture the output
		output, err = cmd.Output()
		if err != nil {
			fmt.Printf("Error executing command: %v\n", err)
			return
		}

		// Parse the JSON output
		var status LoadBalancerStatus
		if err := json.Unmarshal(output, &status); err != nil {
			fmt.Printf("Error parsing JSON: %v\n", err)
			return
		}

		// Print the IP addresses
		for _, ingress := range status.Ingress {
			if ingress.IP != "" {
				fmt.Printf("LoadBalancer IP: %s\n", ingress.IP)
			} else if ingress.Hostname != "" {
				fmt.Printf("LoadBalancer Hostname: %s\n", ingress.Hostname)
			}
		}

	*/
	//@todo auto create splunk index
	/*
	   	yamlConten2 := `
	   splunkPlatform:
	     insecureSkipVerify: true
	     logsEnabled: true
	   logsCollection:
	     extraFileLogs:
	       filelog/tetragon-log:
	         include: [/var/run/cilium/tetragon/tetragon.log]
	         start_at: beginning
	         include_file_path: true
	         include_file_name: false
	         resource:
	           com.splunk.index: demo
	           com.splunk.source: tetragon
	           host.name: 'EXPR(env("K8S_NODE_NAME"))'
	           com.splunk.sourcetype: tetragon
	       filelog/cilium-log:
	         include: [/var/run/cilium/hubble/events.log]
	         start_at: beginning
	         include_file_path: true
	         include_file_name: false
	         resource:
	           com.splunk.index: demo
	           com.splunk.source: cilium
	           host.name: 'EXPR(env("K8S_NODE_NAME"))'
	           com.splunk.sourcetype: cilium
	   agent:
	     extraVolumeMounts:
	       - name: tetragon
	         mountPath: /var/run/cilium/tetragon
	       - name: cilium
	         mountPath: /var/run/cilium/hubble
	     extraVolumes:
	       - name: tetragon
	         hostPath:
	           path: /var/run/cilium/tetragon
	       - name: cilium
	         hostPath:
	           path: /var/run/cilium/hubble
	   	`
	*/
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
