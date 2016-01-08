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
 * A build step, in a build output (see the DockerBuildOutput type).
 */
type DockerBuildStep struct {
	StepNumber int
	Command string
	UsedCache bool
	ProducedDockerImageId string
}

func NewDockerBuildStep(number int, cmd string, usedCache bool,
	producedImageId string) *DockerBuildStep {
	return &DockerBuildStep{
		StepNumber: number,
		Command: cmd,
		UsedCache: usedCache,
		ProducedDockerImageId: producedImageId,
	}
}

func (step *DockerBuildStep) setUsedCache() {
	step.UsedCache = true
}

func (step *DockerBuildStep) setProducedImageId(id string) {
	step.ProducedDockerImageId = id
}

func (step *DockerBuildStep) AsJSON() string {
	
	return fmt.Sprintf("{\"StepNumber\": %d, \"Command\": \"%s\", \"UsedCache\": %d, " +
		"\"ProducedDockerImageId\": \"%s\"}", step.StepNumber,
		rest.EncodeStringForJSON(step.Command), step.UsedCache)
}

