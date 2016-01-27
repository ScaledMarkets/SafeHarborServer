/*******************************************************************************
 * Implementation of ScanProvider for the CoreOS Clair container scanner.
 * See https://github.com/coreos/clair
 *
	// Clair scan:
	// https://github.com/coreos/clair
	// https://github.com/coreos/clair/tree/master/contrib/analyze-local-images
	
	From Clair maintainer (Quentin Machu):
	You don’t actually need to run Clair on each host, a single Clair instance/database
	is able to analyze all your container images. That is why it is an API-driven service.
	All Clair needs is being able to access your container images. When you insert
	a container layer via the API (https://github.com/coreos/clair/blob/master/docs/API.md#insert-a-new-layer),
	you have to specify a path to the layer tarball that Clair can access;
	it can either be a filesystem path or an URL. So you can analyze local images
	or images stored on S3, OpenStack Swift, Ceph pretty easily!
	
	You may want to take a look at https://github.com/coreos/clair/tree/master/contrib/analyze-local-images,
	a small tool I hacked to ease analyzing local images. But in fact, I added
	a very minimal “remote” support, allowing Clair to run somewhere else:
	the local images are served by a web server.
	
	docker pull quay.io/coreos/clair
	sudo docker run -i -t -m 500M -v /tmp:/tmp -p 6060:6060 quay.io/coreos/clair:latest --db-type=bolt --db-path=/db/database
	sudo GOPATH=/home/vagrant go get -u github.com/coreos/clair/contrib/analyze-local-images
	/home/vagrant/bin/analyze-local-images <Docker Image ID>
 */

package providers

import (
	//"errors"
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
	"strconv"

	// SafeHarbor packages:
	"safeharbor/apitypes"
	"safeharbor/rest"
	"safeharbor/util"
)

type ClairService struct {
	Host string
	Port int
	Params map[string]string
}

func CreateClairService(params map[string]interface{}) (ScanService, error) {
	
	var host string
	var portStr string
	var isType bool
	
	host, isType = params["Host"].(string)
	portStr, isType = params["Port"].(string)
	if host == "" { return nil, util.ConstructError("Parameter 'Host' not specified") }
	if portStr == "" { return nil, util.ConstructError("Parameter 'Port' not specified") }
	if ! isType { return nil, util.ConstructError("Parameter 'Host' is not a string") }
	if ! isType { return nil, util.ConstructError("Parameter 'Port' is not a string") }
	
	var port int
	var err error
	port, err = strconv.Atoi(portStr)
	if err != nil { return nil, err }
	
	return &ClairService{
		Host: host,
		Port: port,
		Params: map[string]string{
			"MinimumPriority": "The minimum priority level of vulnerabilities to report",
		},
	}, nil
}

func (clairSvc *ClairService) GetName() string { return "clair" }

func (clairSvc *ClairService) GetEndpoint() string {
	return fmt.Sprintf("http://%s:%d", clairSvc.Host, clairSvc.Port)
}

func (clairSvc *ClairService) GetParameterDescriptions() map[string]string {
	return clairSvc.Params
}

func (clairSvc *ClairService) GetParameterDescription(name string) (string, error) {
	var desc string = clairSvc.Params[name]
	if desc == "" { return "", util.ConstructError("No parameter named '" + name + "'") }
	return desc, nil
}

func (clairSvc *ClairService) CreateScanContext(params map[string]string) (ScanContext, error) {
	
	var minPriority string
	
	if params != nil {
		minPriority = params["MinimumPriority"]
		// this param is optional so do not require its presence.
	}
	
	return &ClairRestContext{
		RestContext: *rest.CreateRestContext(
			clairSvc.Host, clairSvc.Port, setClairSessionId),
		MinimumVulnerabilityPriority: minPriority,
		ClairService: clairSvc,
		sessionId: "",
	}, nil
}

func (clairSvc *ClairService) AsScanProviderDesc() *apitypes.ScanProviderDesc {
	var params = []apitypes.ParameterInfo{}
	for name, desc := range clairSvc.Params {
		params = append(params, *apitypes.NewParameterInfo(name, desc))
	}
	return apitypes.NewScanProviderDesc(clairSvc.GetName(), params)
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
	defer func() {
		fmt.Println("Removing all files at " + path)
		os.RemoveAll(path)
	}()
	if err != nil { return nil, err }

	// Retrieve history
	fmt.Println("Getting image's history")
	layerIDs, err := history(imageName)
	if err != nil { return nil, err }
	if len(layerIDs) == 0 { return nil, util.ConstructError("Could not get image's history") }

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
	if ! isType { return "", "", util.ConstructError("Value returned for APIVersion is not a string") }
	engineVersion, isType = responseMap["EngineVersion"].(string)
	if ! isType { return "", "", util.ConstructError("Value returned for EngineVersion is not a string") }
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
		return "", util.ConstructError(stderr.String())
	}
	err = save.Run()
	if err != nil {
		return "", util.ConstructError(stderr.String())
	}
	err = pipe.Close()
	if err != nil {
		return "", err
	}
	err = extract.Wait()
	if err != nil {
		return "", util.ConstructError(stderr.String())
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
		return []string{}, util.ConstructError(stderr.String())
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
