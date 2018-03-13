package configmap

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	corev1 "github.com/ericchiang/k8s/apis/core/v1"
	"github.com/inovex/trovilo/config"
)

func writeFile(file string, contents []byte) error {
	err := os.MkdirAll(filepath.Dir(file), 0755)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(file, contents, 0644)
}

func writeOSFile(file *os.File, contents []byte) error {
	err := os.MkdirAll(filepath.Dir(file.Name()), 0755)
	if err != nil {
		return err
	}

	_, err = file.Write(contents)
	return err
}

func deleteFile(file string) error {
	return os.Remove(file)
}

func genereateTargetPath(targetDir string, namespace string, configMap string, configMapDataFile string) string {
	return filepath.Join(targetDir, namespace, configMap, configMapDataFile)
}

func runCmdAgainstCMFile(file string, cmd []string) (string, error) {
	var args []string

	for cmdPos := range cmd {
		arg := cmd[cmdPos]
		if strings.Contains(arg, "%s") {
			args = append(args, fmt.Sprintf(arg, file))
		} else {
			args = append(args, arg)
		}
	}
	c := exec.Command(args[0], args[1:]...)
	output, err := c.CombinedOutput()

	return strings.TrimSpace(string(output)), err
}

// VerifyCM runs user-defined tests against ConfigMap's files to decide whether to accept them or not
func VerifyCM(configMap *corev1.ConfigMap, verifySteps []config.VerifyStep) (map[string]string, string, error) {
	verifiedFiles := map[string]string{}

	for file, fileContents := range configMap.Data {
		for step := range verifySteps {
			if len(verifySteps[step].Cmd) != 0 {

				// Prepare (temporary) file to verify
				tempFile, err := ioutil.TempFile("", fmt.Sprintf("trovilo-%s-", file))
				if err != nil {
					return verifiedFiles, "", err
				}
				err = writeOSFile(tempFile, []byte(fileContents))
				if err != nil {
					return verifiedFiles, "", err
				}

				// In the end just remove the temporary file, regardless of the verification result
				defer deleteFile(tempFile.Name())

				output, err := runCmdAgainstCMFile(tempFile.Name(), verifySteps[step].Cmd)

				if err != nil {
					// Immediately abort if there's just one piece of the configmap that is invalid
					return verifiedFiles, output, err
				}

				verifiedFiles[file] = output
			}
		}

	}

	return verifiedFiles, "", nil
}

// CompareCMLabels tests a ConfigMap against expected labels (selector)
func CompareCMLabels(expected map[string]string, actual map[string]string) bool {
	if len(actual) == 0 {
		// immediately abort if there are no labels at all
		return false
	}

	for key, expectedValue := range expected {
		actualValue, found := (actual)[key]
		if !found || expectedValue != actualValue {
			return false
		}
	}
	return true
}

// RegisterCM writes a ConfigMap to filesystem
func RegisterCM(configMap *corev1.ConfigMap, targetDir string) ([]string, error) {
	var registeredFiles []string

	for file, fileContents := range configMap.Data {
		targetFile := genereateTargetPath(targetDir, *configMap.Metadata.Namespace, *configMap.Metadata.Name, file)
		registeredFiles = append(registeredFiles, targetFile)

		err := writeFile(targetFile, []byte(fileContents))
		if err != nil {
			return registeredFiles, err
		}
	}

	return registeredFiles, nil
}

// IsCMAlreadyRegistered is a helper function that checks whether we already know this ConfigMap
func IsCMAlreadyRegistered(configMap *corev1.ConfigMap, targetDir string) bool {
	for file := range configMap.Data {
		targetFile := genereateTargetPath(targetDir, *configMap.Metadata.Namespace, *configMap.Metadata.Name, file)

		_, err := os.Stat(targetFile)
		if err == nil {
			return true
		}
	}
	return false
}

// RemoveCMfromTargetDir removes a ConfigMap's files from filesystem
func RemoveCMfromTargetDir(configMap *corev1.ConfigMap, targetDir string) ([]string, error) {
	var removedFiles []string

	for file := range configMap.Data {
		targetFile := genereateTargetPath(targetDir, *configMap.Metadata.Namespace, *configMap.Metadata.Name, file)
		removedFiles = append(removedFiles, *configMap.Metadata.Namespace, *configMap.Metadata.Name, targetFile)

		err := deleteFile(targetFile)
		if err != nil {
			return removedFiles, err
		}
	}

	return removedFiles, nil
}
