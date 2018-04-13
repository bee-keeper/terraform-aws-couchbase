package test

import (
	"testing"
	"github.com/gruntwork-io/terratest"
	"fmt"
	"github.com/gruntwork-io/terratest/packer"
	"log"
	"time"
	"net/http"
	"net/url"
	"io/ioutil"
	"strings"
	"os"
)

// The username and password we use in all the examples, mocks, and tests
const usernameForTest = "admin"
const passwordForTest = "password"

func createBaseRandomResourceCollection(t * testing.T) *terratest.RandomResourceCollection {
	resourceCollectionOptions := terratest.NewRandomResourceCollectionOptions()

	// Exclude regions where we don't have ACM certs for testing
	resourceCollectionOptions.ForbiddenRegions = []string{
		"ap-northeast-2",
		"ap-southeast-1",
		"eu-central-1",
	}

	randomResourceCollection, err := terratest.CreateRandomResourceCollection(resourceCollectionOptions)
	if err != nil {
		t.Fatalf("Failed to create Random Resource Collection: %s", err.Error())
	}

	return randomResourceCollection
}

func createBaseTerratestOptions(t *testing.T, testName string, folder string, resourceCollection *terratest.RandomResourceCollection) *terratest.TerratestOptions {
	terratestOptions := terratest.NewTerratestOptions()

	terratestOptions.UniqueId = resourceCollection.UniqueId
	terratestOptions.TemplatePath = folder
	terratestOptions.TestName = testName

	return terratestOptions
}

func buildCouchbaseWithPacker(logger *log.Logger, builderName string, baseAmiName string, awsRegion string, folderPath string, edition string) (string, error) {
	templatePath := fmt.Sprintf("%s/couchbase.json", folderPath)

	options := packer.PackerOptions{
		Template: templatePath,
		Only: builderName,
		Vars: map[string]string{
			"aws_region": awsRegion,
			"base_ami_name": baseAmiName,
			"edition": edition,
		},
	}

	// The Packer file provisioner we use tries to copy this entire Couchbase module using a relative path like
	// ../../../terraform-aws-couchbase. This works fine in a normal checkout, but with CircleCi, (a) the code is
	// checked out into a folder called "project" and not "terraform-aws-couchbase" and (b) to support GOPATH, we
	// create a symlink to the original project and run the tests from that symlinked folder. One or both of these
	// issues leads to very strange issues that sometimes cause the Packer build to fail:
	// https://github.com/hashicorp/packer/issues/6103
	if os.Getenv("CIRCLECI") != "" {
		logger.Printf("Overriding root folder path for Packer build to /home/circleci/project/")
		options.Vars["root_folder_path"] = "/home/circleci/project/"
	}

	return packer.BuildAmi(options, logger)
}

func deploy(t *testing.T, terratestOptions *terratest.TerratestOptions) {
	_, err := terratest.Apply(terratestOptions)
	if err != nil {
		t.Fatalf("Failed to apply templates: %s", err.Error())
	}
}

func HttpPostForm(t *testing.T, postUrl string, postParams url.Values, logger *log.Logger) (int, string, error) {
	logger.Printf("Making an HTTP POST call to URL %s with body %v", postUrl, postParams)

	client := http.Client{
		// By default, Go does not impose a timeout, so an HTTP connection attempt can hang for a LONG time.
		Timeout: 10 * time.Second,
	}

	resp, err := client.PostForm(postUrl, postParams)
	if err != nil {
		return -1, "", err
	}

	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return -1, "", err
	}

	return resp.StatusCode, strings.TrimSpace(string(respBody)), nil
}