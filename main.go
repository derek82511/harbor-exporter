package main

import (
	"bufio"
	"crypto/tls"
	"derek82511/harbor-exporter/client"
	"derek82511/harbor-exporter/conf"
	"derek82511/harbor-exporter/utils"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/panjf2000/ants/v2"
	"golang.org/x/crypto/ssh/terminal"
)

// ProcPullImageArg Definition
type ProcPullImageArg struct {
	Image string
}

// ProcSaveImageArg Definition
type ProcSaveImageArg struct {
	SubImages  []string
	ExportFile string
}

var (
	configFile  string
	username    string
	password    string
	insecureTLS bool
	tool        string
)

func init() {
	flag.StringVar(&configFile, "f", "", "Input Configuration File")
	flag.StringVar(&username, "u", "", "Harbor Username")
	flag.BoolVar(&insecureTLS, "k", false, "Use Insecure TLS")
	flag.Usage = usage
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage:\n")
	flag.PrintDefaults()
}

func main() {
	flag.Parse()

	if configFile == "" {
		fmt.Fprintf(os.Stderr, "Error: Unknown args\n")
		os.Exit(1)
	}

	now := time.Now()
	datetime := now.Format("20060102150405")

	configuration := conf.GetConfiguration(configFile)

	tool = (*configuration).Tool

	fmt.Fprintf(os.Stdout, "Start Harbor Image Exporter\n")
	fmt.Fprintf(os.Stdout, "Configuration file: %s\n", configFile)
	fmt.Fprintf(os.Stdout, "Source Registry Host: %s\n", (*configuration).Registry)
	fmt.Fprintf(os.Stdout, "Https use insecure TLS: %t\n", insecureTLS)
	fmt.Fprintf(os.Stdout, "Use Tool: %s\n", tool)
	fmt.Fprintf(os.Stdout, "\n")

	var pool int
	if (*configuration).Runtime.Pool == 0 {
		pool = 1
	} else if (*configuration).Runtime.Pool < 0 {
		pool = ants.DefaultAntsPoolSize
	} else {
		pool = (*configuration).Runtime.Pool
	}

	var maxImageCount int
	if (*configuration).Runtime.ExportFile.MaxImageCount == 0 {
		maxImageCount = 15
	} else {
		maxImageCount = (*configuration).Runtime.ExportFile.MaxImageCount
	}

	fmt.Fprintf(os.Stdout, "Process pool size: %d\n", pool)
	fmt.Fprintf(os.Stdout, "Maximum image count per export tar file: %d\n", maxImageCount)
	fmt.Fprintf(os.Stdout, "\n")

	if username != "" {
		fmt.Fprintf(os.Stdout, "Username: %s\n", username)
		fmt.Fprintf(os.Stdout, "Password: ")
		passwordBytes, err := terminal.ReadPassword(0)
		fmt.Fprintf(os.Stdout, "\n")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		password = string(passwordBytes)
		if password == "" {
			fmt.Fprintf(os.Stderr, "Please enter password\n")
			os.Exit(1)
		}
	}

	fmt.Fprintf(os.Stdout, "Start exporting ...\n")

	images := &[]string{}

	for _, project := range (*configuration).Projects {
		fmt.Fprintf(os.Stdout, "--Prepare Project: %s ...\n", project.Name)

		var repositories []string
		if project.Repository.FetchAll == true {
			repositories = *(getRepositoryList((*configuration).Registry, project.Name))
		} else {
			repositories = project.Repository.Items
		}

		for _, repository := range repositories {
			tmpImages := fetchRepository((*configuration).Registry, project.Name, repository)
			*images = append(*images, (*tmpImages)...)
		}
	}

	defer ants.Release()

	// Pull Image
	fmt.Fprintf(os.Stdout, "--Pulling images ...\n")

	var wgPullImage sync.WaitGroup

	doPullImage := func(i interface{}) {
		procArg := i.(ProcPullImageArg)
		pullImage(procArg.Image)
		wgPullImage.Done()
	}

	poolPullImage, _ := ants.NewPoolWithFunc(pool, doPullImage)

	defer poolPullImage.Release()

	for _, image := range *images {
		wgPullImage.Add(1)
		_ = poolPullImage.Invoke(ProcPullImageArg{
			Image: image,
		})
	}

	fmt.Printf("Pull Image used goroutines: %d\n", poolPullImage.Running())
	wgPullImage.Wait()
	fmt.Printf("Finish Pull Image\n")

	// Save Image
	fmt.Fprintf(os.Stdout, "--Saving images ...\n")

	var wgSaveImage sync.WaitGroup

	doSaveImage := func(i interface{}) {
		procArg := i.(ProcSaveImageArg)

		cmdArgs := []string{"save", "-o", procArg.ExportFile}

		for _, image := range procArg.SubImages {
			cmdArgs = append(cmdArgs, image)
		}

		fmt.Fprintf(os.Stdout, fmt.Sprintf("Save to %s ...\n", procArg.ExportFile))

		ensureDir(procArg.ExportFile)
		cmd := exec.Command(tool, cmdArgs...)
		run(cmd)

		wgSaveImage.Done()
	}

	poolSaveImage, _ := ants.NewPoolWithFunc(pool, doSaveImage)

	defer poolSaveImage.Release()

	i := 0
	for idxRange := range utils.Partition(len(*images), maxImageCount) {
		exportFile := fmt.Sprintf("output/%s/export-%d.tar", datetime, i)
		subImages := (*images)[idxRange.Low:idxRange.High]
		wgSaveImage.Add(1)
		_ = poolSaveImage.Invoke(ProcSaveImageArg{
			SubImages:  subImages,
			ExportFile: exportFile,
		})

		i++
	}

	fmt.Printf("Save Image used goroutines: %d\n", poolSaveImage.Running())
	wgSaveImage.Wait()
	fmt.Printf("Finish Save Image\n")

	// Save Import Shell Script
	fmt.Fprintf(os.Stdout, "--Generate import.sh ...\n")

	scriptText := getScriptTemplate()
	tagScriptText := ""
	pushScriptText := ""

	for _, image := range *images {
		targetImage := strings.Replace(image, (*configuration).Registry, "${target_registry}", 1)
		tagScriptText += fmt.Sprintf("%s tag %s %s\n", tool, image, targetImage)
		pushScriptText += fmt.Sprintf("%s push %s\n", tool, targetImage)
	}

	scriptText = strings.Replace(scriptText, "{{tag_script}}", tagScriptText, 1)
	scriptText = strings.Replace(scriptText, "{{push_script}}", pushScriptText, 1)

	writeFile(scriptText, fmt.Sprintf("output/%s/import.sh", datetime))

	fmt.Fprintf(os.Stdout, "End export\n")
	fmt.Fprintf(os.Stdout, "Output:\n")
	fmt.Fprintf(os.Stdout, "- %s\n", fmt.Sprintf("output/%s/export-*.tar", datetime))
	fmt.Fprintf(os.Stdout, "- %s\n", fmt.Sprintf("output/%s/import.sh", datetime))
}

var _client *resty.Client

func getClient() *resty.Client {
	if _client == nil {
		_client = resty.New()
		_client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: insecureTLS})
		if username != "" {
			_client.SetBasicAuth(username, password)
		}
	}

	return _client
}

func getRepositoryList(registry string, project string) *[]string {
	repositories := client.GetRepositories(getClient(), registry, project)

	repos := &[]string{}

	for _, repository := range *repositories {
		*repos = append(*repos, strings.TrimPrefix(repository.Name, fmt.Sprintf("%s/", project)))
	}

	return repos
}

func fetchRepository(registry string, project string, repository string) *[]string {
	artifacts := client.GetArtifacts(getClient(), registry, project, repository)

	repoName := fmt.Sprintf("%s/%s/%s", registry, project, repository)

	images := &[]string{}

	for _, artifact := range *artifacts {
		for _, tag := range artifact.Tags {
			fullImageName := fmt.Sprintf("%s:%s", repoName, tag.Name)
			*images = append(*images, fullImageName)
		}
	}

	return images
}

func pullImage(image string) {
	cmdArgs := []string{"pull", image}
	cmd := exec.Command(tool, cmdArgs...)
	run(cmd)
}

func run(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "StderrPipe error: %v\n", err)
		os.Exit(1)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "StderrPipe error: %v\n", err)
		os.Exit(1)
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Start error: %v\n", err)
		os.Exit(1)
	}

	scannerOut := bufio.NewScanner(stdout)
	scannerOut.Split(bufio.ScanLines)
	for scannerOut.Scan() {
		text := scannerOut.Text()
		fmt.Fprintf(os.Stdout, "%s\n", text)
	}

	scannerErr := bufio.NewScanner(stderr)
	scannerErr.Split(bufio.ScanLines)
	for scannerErr.Scan() {
		text := scannerErr.Text()
		fmt.Fprintf(os.Stderr, "%s\n", text)
	}
}

func getScriptTemplate() string {
	text := ""

	text += "#!/bin/sh\n"
	text += "\n"
	text += "target_registry=$1\n"
	text += "\n"
	text += "for f in export-*.tar; do\n"
	text += "    cat $f | docker load\n"
	text += "done\n"
	text += "\n"
	text += "{{tag_script}}"
	text += "\n"
	text += "{{push_script}}"
	text += "\n"

	return text
}

func writeFile(text string, path string) {
	ensureDir(path)

	f, err := os.Create(path)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Create File Error: %v\n", err)
		os.Exit(1)
	}

	defer f.Close()

	_, err = f.WriteString(text)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Write File Error: %v\n", err)
		os.Exit(1)
	}
}

func ensureDir(path string) {
	dirName := filepath.Dir(path)
	if _, serr := os.Stat(dirName); serr != nil {
		merr := os.MkdirAll(dirName, os.ModePerm)
		if merr != nil {
			fmt.Fprintf(os.Stderr, "Mkdir Error: %v\n", merr)
			os.Exit(1)
		}
	}
}
