package providers

import (
	"errors"
	"net/http"
	"fmt"

	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	// My packages:
	"safeharbor/apitypes"
	"safeharbor/rest"
)


/*******************************************************************************
 * For accessing the Clair scanning service.
 */
type ClairRestContext struct {
	rest.RestContext
	sessionId string
}

func CreateClairContext(hostname string, port int) *ClairRestContext {
	
	var portString string = fmt.Sprintf("%d", port)
	return &ClairRestContext{
		RestContext: *rest.CreateRestContext(hostname, portString, setClairSessionId),
		sessionId: "",
	}
}

func (clairContext *ClairRestContext) PingService() *apitypes.Result {
	var apiVersion string
	var engineVersion string
	var err error
	apiVersion, engineVersion, err = clairContext.getVersions()
	if err != nil { return apitypes.NewResult(500, err.Error()) }
	return apitypes.NewResult(200, fmt.Sprintf(
		"Service is up: api version %s, engine version %s", apiVersion, engineVersion))
}

/*******************************************************************************
 * See https://github.com/coreos/clair/blob/master/contrib/analyze-local-images/main.go
 */
func (clairContext *ClairRestContext) ScanImage(imageName string) *apitypes.Result {
	
	// Use the docker 'save' command to extract image to a tar of tar files.
	// Must be extracted to a temp directory that is shared with the clair container.
	
	// Save image
	fmt.Printf("Saving %s\n", imageName)
	path, err := save(imageName)
	defer os.RemoveAll(path)
	if err != nil {
		log.Fatalf("- Could not save image: %s\n", err)
	}

	// Retrieve history
	fmt.Println("Getting image's history")
	layerIDs, err := history(imageName)
	if err != nil || len(layerIDs) == 0 {
		log.Fatalf("- Could not get image's history: %s\n", err)
	}

	// Analyze layers
	fmt.Printf("Analyzing %d layers\n", len(layerIDs))
	for i := 0; i < len(layerIDs); i++ {
		fmt.Printf("- Analyzing %s\n", layerIDs[i])

		var err error
		if i > 0 {
			err = analyzeLayer(*endpoint, path+"/"+layerIDs[i]+"/layer.tar", layerIDs[i], layerIDs[i-1])
		} else {
			err = analyzeLayer(*endpoint, path+"/"+layerIDs[i]+"/layer.tar", layerIDs[i], "")
		}
		if err != nil {
			log.Fatalf("- Could not analyze layer: %s\n", err)
		}
	}

	// Get vulnerabilities
	fmt.Println("Getting image's vulnerabilities")
	vulnerabilities, err := getVulnerabilities(*endpoint, layerIDs[len(layerIDs)-1], *minimumPriority)
	if err != nil {
		log.Fatalf("- Could not get vulnerabilities: %s\n", err)
	}
	if len(vulnerabilities) == 0 {
		fmt.Println("Bravo, your image looks SAFE !")
	}
	for _, vulnerability := range vulnerabilities {
		fmt.Printf("- # %s\n", vulnerability.ID)
		fmt.Printf("  - Priority:    %s\n", vulnerability.Priority)
		fmt.Printf("  - Link:        %s\n", vulnerability.Link)
		fmt.Printf("  - Description: %s\n", vulnerability.Description)
	}

	/*
	defer os.RemoveAll(....temp dir)
	
	// Extract the individual layer tar files.
	
	// Scan each layer.
	for _, layerPath := range layerPaths {
		
		var id string = ....
		var err error = clairContext.processLayer(id, layerPath, "")
		if err != nil {
			....
		}
	}
	*/
	
	
	return apitypes.NewResult(500, "Not implemented yet")
}


/**************************** Implementation Methods ***************************
 ******************************************************************************/


/*******************************************************************************
 * 
 */
func (clairContext *ClairRestContext) getVersions() (apiVersion string, engineVersion string, err error) {

	var resp *http.Response
	resp, err = clairContext.SendGet(clairContext.sessionId,
		"v1/versions",
		[]string{},
		[]string{})
	
	if err != nil { return "", "", err }
	defer resp.Body.Close()
	
	clairContext.Verify200Response(resp)

	var responseMap map[string]interface{}
	responseMap, err = rest.ParseResponseBodyToMap(resp.Body)
	if err != nil { return "", "", err }
	var isType bool
	apiVersion, isType = responseMap["APIVersion"].(string)
	if ! isType { return "", "", errors.New("Value returned for APIVersion is not a string") }
	engineVersion, isType = responseMap["EngineVersion"].(string)
	if ! isType { return "", "", errors.New("Value returned for EngineVersion is not a string") }
	return apiVersion, engineVersion, nil
}

func (clairContext *ClairRestContext) getHealth() string {
	//resp = get("v1/health")
	return ""
}

func (clairContext *ClairRestContext) processLayer(id, path, parentId string) error {
	var err error
	var resp *http.Response
	resp, err = clairContext.SendPost(clairContext.sessionId,
		"v1/layers",
		[]string{"ID", "Path", "ParentID"},
		[]string{id, path, parentId})
	
	if err != nil { return err }
	defer resp.Body.Close()
	
	clairContext.Verify200Response(resp)

	//var responseMap map[string]interface{}
	_, err  = rest.ParseResponseBodyToMap(resp.Body)
	if err != nil { return err }
	//var version string = responseMap["Version"]
	return nil
}

func (clairContext *ClairRestContext) getLayerOS() {
}

func (clairContext *ClairRestContext) getLayerParent() {
}

func (clairContext *ClairRestContext) getLayerPackageList() {
}

func (clairContext *ClairRestContext) getLayerPackageDiff() {
}

func (clairContext *ClairRestContext) getLayerVulnerabilities() {
}

func (clairContext *ClairRestContext) getLayerVulnerabilitiesDelta() {
}

func (clairContext *ClairRestContext) getLayerVulnerabilitiesBatch() {
}

func (clairContext *ClairRestContext) getVulnerabilityInfo() {
}

func (clairContext *ClairRestContext) getLayersIntroducingVulnerability() {
}

func (clairContext *ClairRestContext) getLayersAffectedByVulnerability() {
}

/*******************************************************************************
 * Set the session Id as a cookie.
 */
func setClairSessionId(req *http.Request, sessionId string) {
	
	// Set cookie containing the session Id.
	var cookie = &http.Cookie{
		Name: "SessionId",
		Value: sessionId,
		//Path: 
		//Domain: 
		//Expires: 
		//RawExpires: 
		MaxAge: 86400,
		Secure: false,  //....change to true later.
		HttpOnly: true,
		//Raw: 
		//Unparsed: 
	}
	
	req.AddCookie(cookie)
}

/*******************************************************************************
 * Retrieve image as a tar of tars, and extract each tar (layer).
 * Return the path to the directory containing the layer tar files.
 */
func save(imageName string) (string, error) {
	path, err := ioutil.TempDir("", "analyze-local-image-")
	if err != nil {
		return "", err
	}

	var stderr bytes.Buffer
	save := exec.Command("docker", "save", imageName)
	save.Stderr = &stderr
	extract := exec.Command("tar", "xf", "-", "-C"+path)
	extract.Stderr = &stderr
	pipe, err := extract.StdinPipe()
	if err != nil {
		return "", err
	}
	save.Stdout = pipe

	err = extract.Start()
	if err != nil {
		return "", errors.New(stderr.String())
	}
	err = save.Run()
	if err != nil {
		return "", errors.New(stderr.String())
	}
	err = pipe.Close()
	if err != nil {
		return "", err
	}
	err = extract.Wait()
	if err != nil {
		return "", errors.New(stderr.String())
	}

	return path, nil
}

/*******************************************************************************
 * Retrieve a list of the layer Ids.
 */
func history(imageName string) ([]string, error) {
	var stderr bytes.Buffer
	cmd := exec.Command("docker", "history", "-q", "--no-trunc", imageName)
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return []string{}, err
	}

	err = cmd.Start()
	if err != nil {
		return []string{}, errors.New(stderr.String())
	}

	var layers []string
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		layers = append(layers, scanner.Text())
	}

	for i := len(layers)/2 - 1; i >= 0; i-- {
		opp := len(layers) - 1 - i
		layers[i], layers[opp] = layers[opp], layers[i]
	}

	return layers, nil
}

/*******************************************************************************
 * 
 */
func getVulnerabilities(endpoint, layerID, minimumPriority string) ([]APIVulnerability, error) {
	response, err := http.Get(endpoint + fmt.Sprintf(getLayerVulnerabilitiesURI, layerID, minimumPriority))
	if err != nil {
		return []APIVulnerability{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		body, _ := ioutil.ReadAll(response.Body)
		return []APIVulnerability{}, fmt.Errorf("Got response %d with message %s", response.StatusCode, string(body))
	}

	var apiResponse APIVulnerabilitiesResponse
	err = json.NewDecoder(response.Body).Decode(&apiResponse)
	if err != nil {
		return []APIVulnerability{}, err
	}

	return apiResponse.Vulnerabilities, nil
}
