/*******************************************************************************
 * Abstract functions that we need from docker and docker registry.
 */

package server

import (
	"fmt"
	"os"
	"io"
	"io/ioutil"
	"strings"
	"os/exec"
	"errors"
)


type DockerService struct {
}

func NewDockerService() *DockerService {
	return &DockerService{}
}

/*******************************************************************************
 * 
 */
func (docker *DockerService) BuildDockerfile(dockerfile Dockerfile, realm Realm, repo Repo,
	imageName string) (string, error) {
	
	if ! localDockerImageNameIsValid(imageName) {
		return "", errors.New(fmt.Sprintf("Image name '%s' is not valid - must be " +
			"of format <name>[:<tag>]", imageName))
	}
	fmt.Println("Image name =", imageName)
	
	// Check if am image with that name already exists.
	var cmd *exec.Cmd = exec.Command("/usr/bin/docker", "inspect", imageName)
	var output []byte
	var err error
	output, err = cmd.CombinedOutput()
	var outputStr string = string(output)
	if ! strings.HasPrefix(outputStr, "Error") {
		return "", errors.New("An image with name " + imageName + " already exists.")
	}
	
	// Verify that the image name conforms to Docker's requirements.
	err = nameConformsToSafeHarborImageNameRules(imageName)
	if err != nil { return "", err }
	
	// Create a temporary directory to serve as the build context.
	var tempDirPath string
	tempDirPath, err = ioutil.TempDir("", "")
	//....TO DO: Is the above a security problem? Do we need to use a private
	// directory? I think so.
	defer os.RemoveAll(tempDirPath)
	fmt.Println("Temp directory = ", tempDirPath)

	// Copy dockerfile to that directory.
	var in, out *os.File
	in, err = os.Open(dockerfile.getExternalFilePath())
	if err != nil { return "", err }
	var dockerfileCopyPath string = tempDirPath + "/" + dockerfile.getName()
	out, err = os.Create(dockerfileCopyPath)
	if err != nil { return "", err }
	_, err = io.Copy(out, in)
	if err != nil { return "", err }
	err = out.Close()
	if err != nil { return "", err }
	fmt.Println("Copied Dockerfile to " + dockerfileCopyPath)
	
//	fmt.Println("Changing directory to '" + tempDirPath + "'")
//	err = os.Chdir(tempDirPath)
//	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	// Create a the docker build command.
	// https://docs.docker.com/reference/commandline/build/
	// REPOSITORY                      TAG                 IMAGE ID            CREATED             VIRTUAL SIZE
	// docker.io/cesanta/docker_auth   latest              3d31749deac5        3 months ago        528 MB
	// Image id format: <hash>[:TAG]
	
	var imageFullName string = realm.getName() + "/" + repo.getName() + ":" + imageName
	cmd = exec.Command("docker", "build", 
		"--file", tempDirPath + "/" + dockerfile.getName(),
		"--tag", imageFullName, tempDirPath)
	
	// Execute the command in the temporary directory.
	// This initiates processing of the dockerfile.
	output, err = cmd.CombinedOutput()
	outputStr = string(output)
	fmt.Println("...finished processing dockerfile.")
	fmt.Println("Output from docker build command:")
	fmt.Println(outputStr)
	fmt.Println()
	fmt.Println("End of output from docker build command.")
	
	fmt.Println("Files in " + tempDirPath + ":")
	dirfiles, _ := ioutil.ReadDir(tempDirPath)
	for _, f := range dirfiles {
		fmt.Println("\t" + f.Name())
	}
	
	if err != nil {
		fmt.Println()
		fmt.Println("Returning from buildDockerfile, with error")
		return "", errors.New(err.Error() + ", " + outputStr)
	}
	fmt.Println("Performed docker build command successfully.")
	return outputStr, nil
}

/*******************************************************************************
 * 
 */
func (docker *DockerService) SaveImage(dockerImage DockerImage) (string, error) {
	
	fmt.Println("Creating temp file to save the image to...")
	var tempFile *os.File
	var err error
	tempFile, err = ioutil.TempFile("", "")
	// TO DO: Is the above a security issue?
	if err != nil { return "", err }
	var tempFilePath = tempFile.Name()
	
	var imageFullName string
	imageFullName, err = dockerImage.getFullName()
	if err != nil { return "", err }
	var cmd *exec.Cmd = exec.Command("docker", "save", "-o", tempFilePath, imageFullName)
	fmt.Println(fmt.Sprintf("Running docker save -o%s %s", tempFilePath, imageFullName))
	err = cmd.Run()
	if err != nil { return "", err }
	return tempFilePath, nil
}

/*******************************************************************************
 * 
 */
func (docker *DockerService) RemoveDockerImage(dockerImage DockerImage) error {
	var imageFullName string
	var err error
	imageFullName, err = dockerImage.getFullName()
	if err != nil { return err }
	var cmd *exec.Cmd = exec.Command("docker", "rmi", imageFullName)
	fmt.Println(fmt.Sprintf("Running docker rmi %s", imageFullName))
	return cmd.Run()
}
