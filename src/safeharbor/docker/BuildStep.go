package docker

import (
	"fmt"
	
	// SafeHarbor packages:
	"safeharbor/rest"
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

func NewDockerBuildStep(number int, cmd string) *DockerBuildStep {
	return &DockerBuildStep{
		StepNumber: number,
		Command: cmd,
	}
}

func (step *DockerBuildStep) setUsedCache() {
	step.UsedCache = true
}

func (step *DockerBuildStep) setProducedImageId(id string) {
	step.ProducedDockerImageId = id
}

func (step *DockerBuildStep) AsJSON() string {
	
	var usedCache string
	if step.UsedCache { usedCache = "true" } else { usedCache = "false" }
	return fmt.Sprintf("{\"StepNumber\": %d, \"Command\": \"%s\", \"UsedCache\": %s, " +
		"\"ProducedDockerImageId\": \"%s\"}", step.StepNumber,
		rest.EncodeStringForJSON(step.Command), usedCache, step.ProducedDockerImageId)
}

