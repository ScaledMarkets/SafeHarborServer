package docker

import (
	"fmt"
	"os"
	"io"
	"io/ioutil"
	"strings"
	"os/exec"
	"errors"
	
	// SafeHarbor packages:
	"rest"
)

/*******************************************************************************
 * A structured representation of the output of a docker build. Produced by parsing
 * the output from the docker build command.
 */
type DockerBuildOutput struct {
	ErrorMessage string
	FinalDockerImageId string
	Steps []*DockerBuildStep
}

func NewDockerBuildOutput() *DockerBuildOutput {
	return &DockerBuildOutput{
	}
}

func (buildOutput *DockerBuildOutput) addStep(number int, cmd string, usedCache bool,
	producedImageId string) *DockerBuildStep {

	var step = NewDockerBuildStep(number, cmd)
	buildOutput.Steps = append(step, buildOutput.Steps)
	return step
}

func (buildOutput *DockerBuildOutput) setFinalImageId(id string) {
	buildOutput.FinalDockerImageId = id
}

func (buildOutput *DockerBuildOutput) GetFinalDockerImageId() string {
	return buildOutput.FinalDockerImageId
}

func (buildOutput *DockerBuildOutput) AsJSON() string {
	
	var s = fmt.Sprintf("{\"ErrorMessage\": \"%s\", \"FinalDockerImageId\": \"%s\", \"Steps\": [",
		buildOutput.ErrorMessage, buildOutput.FinalDockerImageId)
	
	for i, step := range buildOutput.Steps {
		if i > 0 { s = s + ", " }
		s = s + step.AsJSON()
	}
	
	s = s + "]}"
	return s
}

