/*******************************************************************************
 * Implementation of ScanProvider for the CoreOS Clair container scanner.
 * See https://github.com/coreos/clair
 */

package providers

import (
	//"errors"
	"net/http"
	"fmt"

	//"bufio"
	//"bytes"
	//"encoding/json"
	//"flag"
	//"io/ioutil"
	//"log"
	//"os"
	//"os/exec"
	//"strconv"
	//"strings"
	//"time"
	"strconv"

	// SafeHarbor packages:
	"safeharbor/apitypes"
	"safeharbor/rest"
	"safeharbor/util"
)

type ClairServiceStub struct {
	UseSSL bool
	Host string
	Port int
	LocalIPAddress string  // of this machine, for clair to call back
	Params map[string]string
}

func (clairSvc *ClairServiceStub) GetName() string { return "clair" }

func CreateClairServiceStub(params map[string]interface{}) (ScanService, error) {
	
	var host string
	var portStr string
	var localIPAddress string
	var isType bool
	
	host, isType = params["Host"].(string)
	if host == "" { return nil, util.ConstructError("Parameter 'Host' not specified") }
	if ! isType { return nil, util.ConstructError("Parameter 'Host' is not a string") }

	portStr, isType = params["Port"].(string)
	if portStr == "" { return nil, util.ConstructError("Parameter 'Port' not specified") }
	if ! isType { return nil, util.ConstructError("Parameter 'Port' is not a string") }

	localIPAddress, isType = params["LocalIPAddress"].(string)
	if localIPAddress == "" { return nil, util.ConstructError("Parameter 'localIPAddress' not specified") }
	if ! isType { return nil, util.ConstructError("Parameter 'localIPAddress' is not a string") }
	
	var port int
	var err error
	port, err = strconv.Atoi(portStr)
	if err != nil { return nil, err }
	
	return &ClairServiceStub{
		UseSSL: false,
		Host: host,
		Port: port,
		LocalIPAddress: localIPAddress,
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
	if desc == "" { return "", util.ConstructError("No parameter named '" + name + "'") }
	return desc, nil
}

func (clairSvc *ClairServiceStub) CreateScanContext(params map[string]string) (ScanContext, error) {
	
	var minPriority string
	
	if params != nil {
		minPriority = params["MinimumPriority"]
		// this param is optional so do not require its presence.
	}
	
	return &ClairRestContextStub{
		RestContext: *rest.CreateRestContext(clairSvc.UseSSL,
			clairSvc.Host, clairSvc.Port, "", "", setClairSessionStubId),
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
	
	// Save image
	fmt.Printf("Saving %s\n", imageName)

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

	var vulnDescs = make([]*apitypes.VulnerabilityDesc, len(vulnerabilities))
	for i, vuln := range vulnerabilities {
		vulnDescs[i] = apitypes.NewVulnerabilityDesc(
			vuln.ID, vuln.Link, vuln.Priority, vuln.Description)
	}
	
	return &ScanResult{
		Vulnerabilities: vulnDescs,
	}, nil
}


/**************************** Clair Service Methods ***************************
 ******************************************************************************/


/*******************************************************************************
 * 
 */
func (clairContext *ClairRestContextStub) GetVersions() (apiVersion string, engineVersion string, err error) {

	var resp *http.Response
	resp, err = clairContext.SendSessionGet(clairContext.sessionId, "v1/versions", nil, nil)
	
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
