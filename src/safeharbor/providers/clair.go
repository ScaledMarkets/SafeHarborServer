/*******************************************************************************
 * Implementation of ScanProvider for the CoreOS Clair container scanner.
 * See https://github.com/coreos/clair
 */

package providers

import (
	"errors"
	"net/http"
	"fmt"

	"bufio"
	"bytes"
	"encoding/json"
	//"flag"
	"io/ioutil"
	//"log"
	"os"
	"os/exec"
	//"strconv"
	//"strings"
	//"time"

	// My packages:
	"safeharbor/apitypes"
	"safeharbor/rest"
)

type ClairService struct {
	Host string
	Port int
}

func CreateClairService(host string, port int) ScanService {
	return &ClairService{
		Host: host,
		Port: port,
	}
}

func (clairSvc *ClairService) GetEndpoint() string {
	return fmt.Sprintf("http://%s:%d", clairSvc.Host, clairSvc.Port)
}

func (clairSvc *ClairService) GetParameterDescriptions() map[string]string {
	return map[string]string{
		"MinimumPriority": "The minimum priority level of vulnerabilities to report",
	}
}

func (clairSvc *ClairService) CreateScanContext(params map[string]string) (ScanContext, error) {
	
	var minPriority string
	
	if params != nil {
		minPriority = params["MinimumPriority"]
		// this param is optional so do not require its presence.
	}
	
	return &ClairRestContext{
		RestContext: *rest.CreateRestContext(
			clairSvc.Host, fmt.Sprintf("%d", clairSvc.Port), setClairSessionId),
		MinimumVulnerabilityPriority: minPriority,
		ClairService: clairSvc,
		sessionId: "",
	}, nil
}

/*******************************************************************************
 * For accessing the Clair scanning service.
 */
type ClairRestContext struct {
	rest.RestContext
	MinimumVulnerabilityPriority string
	ClairService *ClairService
	sessionId string
}

func (clairContext *ClairRestContext) getEndpoint() string {
	return clairContext.ClairService.GetEndpoint()
}

func (clairContext *ClairRestContext) PingService() *apitypes.Result {
	var apiVersion string
	var engineVersion string
	var err error
	apiVersion, engineVersion, err = clairContext.GetVersions()
	if err != nil { return apitypes.NewResult(500, err.Error()) }
	return apitypes.NewResult(200, fmt.Sprintf(
		"Service is up: api version %s, engine version %s", apiVersion, engineVersion))
}

/*******************************************************************************
 * See https://github.com/coreos/clair/blob/master/contrib/analyze-local-images/main.go
 */
func (clairContext *ClairRestContext) ScanImage(imageName string) (*ScanResult, error) {
	
	// Use the docker 'save' command to extract image to a tar of tar files.
	// Must be extracted to a temp directory that is shared with the clair container.
	
	// Save image
	fmt.Printf("Saving %s\n", imageName)
	path, err := save(imageName)
	defer os.RemoveAll(path)
	if err != nil { return nil, err }

	// Retrieve history
	fmt.Println("Getting image's history")
	layerIDs, err := history(imageName)
	if err != nil { return nil, err }
	if len(layerIDs) == 0 { return nil, errors.New("Could not get image's history") }

	// Analyze layers
	fmt.Printf("Analyzing %d layers\n", len(layerIDs))
	for i := 0; i < len(layerIDs); i++ {
		fmt.Printf("- Analyzing %s\n", layerIDs[i])

		var err error
		if i > 0 {
			err = analyzeLayer(clairContext.getEndpoint(), path+"/"+layerIDs[i]+"/layer.tar", layerIDs[i], layerIDs[i-1])
		} else {
			err = analyzeLayer(clairContext.getEndpoint(), path+"/"+layerIDs[i]+"/layer.tar", layerIDs[i], "")
		}
		if err != nil { return nil, err }
	}

	// Get vulnerabilities
	fmt.Println("Getting image's vulnerabilities")
	var vulnerabilities []Vulnerability
	vulnerabilities, err = getVulnerabilities(
		clairContext.getEndpoint(), layerIDs[len(layerIDs)-1], clairContext.MinimumVulnerabilityPriority)
	if err != nil { return nil, err }
	if len(vulnerabilities) == 0 {
		fmt.Println("No vulnerabilities found for image")
	}
	for _, vulnerability := range vulnerabilities {
		fmt.Printf("- # %s\n", vulnerability.ID)
		fmt.Printf("  - Priority:    %s\n", vulnerability.Priority)
		fmt.Printf("  - Link:        %s\n", vulnerability.Link)
		fmt.Printf("  - Description: %s\n", vulnerability.Description)
	}
	
	return &ScanResult{
		Vulnerabilities: vulnerabilities,
	}, nil
}


/**************************** Clair Service Methods ***************************
 ******************************************************************************/


/*******************************************************************************
 * 
 */
func (clairContext *ClairRestContext) GetVersions() (apiVersion string, engineVersion string, err error) {

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

func (clairContext *ClairRestContext) GetHealth() string {
	//resp = get("v1/health")
	return ""
}

func (clairContext *ClairRestContext) ProcessLayer(id, path, parentId string) error {
	var err error
	var resp *http.Response
	
	err = analyzeLayer(clairContext.getEndpoint(), path, id, parentId)
	
	if err != nil { return err }
	defer resp.Body.Close()
	
	clairContext.Verify200Response(resp)

	//var responseMap map[string]interface{}
	_, err  = rest.ParseResponseBodyToMap(resp.Body)
	if err != nil { return err }
	//var version string = responseMap["Version"]
	return nil
}

func (clairContext *ClairRestContext) GetLayerOS() {
}

func (clairContext *ClairRestContext) GetLayerParent() {
}

func (clairContext *ClairRestContext) GetLayerPackageList() {
}

func (clairContext *ClairRestContext) GetLayerPackageDiff() {
}

func (clairContext *ClairRestContext) GetLayerVulnerabilities() {
}

func (clairContext *ClairRestContext) GetLayerVulnerabilitiesDelta() {
}

func (clairContext *ClairRestContext) GetLayerVulnerabilitiesBatch() {
}

func (clairContext *ClairRestContext) GetVulnerabilityInfo() {
}

func (clairContext *ClairRestContext) GetLayersIntroducingVulnerability() {
}

func (clairContext *ClairRestContext) GetLayersAffectedByVulnerability() {
}


/**************************** Internal Implementation Methods ***************************
 ******************************************************************************/



const (
	postLayerURI               = "/v1/layers"
	getLayerVulnerabilitiesURI = "/v1/layers/%s/vulnerabilities?minimumPriority=%s"
)

type APIVulnerabilitiesResponse struct {
	Vulnerabilities []Vulnerability
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
func analyzeLayer(endpoint, path, layerID, parentLayerID string) error {
	payload := struct{ ID, Path, ParentID string }{ID: layerID, Path: path, ParentID: parentLayerID}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	request, err := http.NewRequest("POST", endpoint+postLayerURI, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != 201 {
		body, _ := ioutil.ReadAll(response.Body)
		return fmt.Errorf("Got response %d with message %s", response.StatusCode, string(body))
	}

	return nil
}

/*******************************************************************************
 * 
 */
func getVulnerabilities(endpoint, layerID, minimumPriority string) ([]Vulnerability, error) {
	
	response, err := http.Get(endpoint + fmt.Sprintf(getLayerVulnerabilitiesURI, layerID, minimumPriority))
	if err != nil {
		return []Vulnerability{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		body, _ := ioutil.ReadAll(response.Body)
		return []Vulnerability{}, fmt.Errorf("Got response %d with message %s", response.StatusCode, string(body))
	}

	var apiResponse APIVulnerabilitiesResponse
	err = json.NewDecoder(response.Body).Decode(&apiResponse)
	if err != nil {
		return []Vulnerability{}, err
	}

	return apiResponse.Vulnerabilities, nil
}
