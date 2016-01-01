/*******************************************************************************
 * Implementation of ScanProvider for the CoreOS Clair container scanner.
 * See https://github.com/coreos/clair
 */

package providers

import (
	"errors"
	"net/http"
	"fmt"

	//"bufio"
	//"bytes"
	//"encoding/json"
	//"flag"
	//"io/ioutil"
	//"log"
	"os"
	//"os/exec"
	//"strconv"
	//"strings"
	//"time"
	"strconv"

	// My packages:
	"safeharbor/apitypes"
	"safeharbor/rest"
)

type ClairServiceStub struct {
	Host string
	Port int
	Params map[string]string
}

func (clairSvc *ClairServiceStub) GetName() string { return "clair" }

func CreateClairServiceStub(params map[string]interface{}) (ScanService, error) {
	
	var host string
	var portStr string
	var isType bool
	
	host, isType = params["Host"].(string)
	portStr, isType = params["Port"].(string)
	if host == "" { return nil, errors.New("Parameter 'Host' not specified") }
	if portStr == "" { return nil, errors.New("Parameter 'Port' not specified") }
	if ! isType { return nil, errors.New("Parameter 'Host' is not a string") }
	if ! isType { return nil, errors.New("Parameter 'Port' is not a string") }
	
	var port int
	var err error
	port, err = strconv.Atoi(portStr)
	if err != nil { return nil, err }
	
	return &ClairServiceStub{
		Host: host,
		Port: port,
		Params: map[string]string{
			"MinimumPriority": "The minimum priority level of vulnerabilities to report",
		},
	}, nil
}

func (clairSvc *ClairServiceStub) GetEndpoint() string {
	return fmt.Sprintf("http://%s:%d", clairSvc.Host, clairSvc.Port)
}

func (clairSvc *ClairServiceStub) GetParameterDescriptions() map[string]string {
	return clairSvc.Params
}

func (clairSvc *ClairServiceStub) GetParameterDescription(name string) (string, error) {
	var desc string = clairSvc.Params[name]
	if desc == "" { return "", errors.New("No parameter named '" + name + "'") }
	return desc, nil
}

func (clairSvc *ClairServiceStub) CreateScanContext(params map[string]string) (ScanContext, error) {
	
	var minPriority string
	
	if params != nil {
		minPriority = params["MinimumPriority"]
		// this param is optional so do not require its presence.
	}
	
	return &ClairRestContextStub{
		RestContext: *rest.CreateRestContext(
			clairSvc.Host, fmt.Sprintf("%d", clairSvc.Port), setClairSessionStubId),
		MinimumVulnerabilityPriority: minPriority,
		ClairServiceStub: clairSvc,
		sessionId: "",
	}, nil
}

func (clairSvc *ClairServiceStub) AsScanProviderDesc() *apitypes.ScanProviderDesc {
	var params = []apitypes.ParameterInfo{}
	for name, desc := range clairSvc.Params {
		params = append(params, *apitypes.NewParameterInfo(name, desc))
	}
	return apitypes.NewScanProviderDesc(clairSvc.GetName(), params)
}

/*******************************************************************************
 * For accessing the Clair scanning service.
 */
type ClairRestContextStub struct {
	rest.RestContext
	MinimumVulnerabilityPriority string
	ClairServiceStub *ClairServiceStub
	sessionId string
}

func (clairContext *ClairRestContextStub) getEndpoint() string {
	return clairContext.ClairServiceStub.GetEndpoint()
}

func (clairContext *ClairRestContextStub) PingService() *apitypes.Result {
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
func (clairContext *ClairRestContextStub) ScanImage(imageName string) (*ScanResult, error) {
	
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

	// Analyze layers
	fmt.Printf("Analyzing layers")

	// Get vulnerabilities
	fmt.Println("Getting image's vulnerabilities")
	var vulnerabilities = []Vulnerability{
		Vulnerability{
			ID: "12345-XYZ-4",
			Link: "http://somewhere.cert.org",
			Priority: "High",
			Description: "A very bad vulnerability",
		},
	}
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
func (clairContext *ClairRestContextStub) GetVersions() (apiVersion string, engineVersion string, err error) {

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

func (clairContext *ClairRestContextStub) GetHealth() string {
	//resp = get("v1/health")
	return ""
}


/**************************** Internal Implementation Methods ***************************
 ******************************************************************************/



/*******************************************************************************
 * Set the session Id as a cookie.
 */
func setClairSessionStubId(req *http.Request, sessionId string) {
	
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
