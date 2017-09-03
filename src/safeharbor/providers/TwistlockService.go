/*******************************************************************************
 * Implementation of ScanProvider for the Twistlock container scanner.
 * See:
 *	https://twistlock.desk.com/customer/en/portal/topics/876139-twistlock-api/articles
 *	https://twistlock.desk.com/customer/en/portal/articles/2831956-twistlock-api-2-1
 *		(See "Selective Repository Scan")
 *	Check ability to access the API:
 *		curl -k -u admin:admin https://localhost:8083/api/v1/defenders
 *	Configuring registry scanning:
 *		https://twistlock.desk.com/customer/portal/articles/2309759-configure-registry-scans
 *	Performing a scan:
 *		curl -k -u admin:admin -H "Content-Type: application/json" -d '{"tag":{"registry":"","repo":"scaledmarkets/taskruntime"}}' -X POST http://localhost:8081/api/v1/registry/scan
 *	Obtaining scan results:
 *		curl -k -u admin:admin https://localhost:8083/api/v1/registry?repository='scaledmarkets/taskruntime'
 *
 * Copyright Scaled Markets, Inc.
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
	"time"
	"strconv"

	// SafeHarbor packages:
	"safeharbor/apitypes"
	"safeharbor/utils"

	"utilities/rest"
)

type TwistlockService struct {
	UseSSL bool
	Host string
	Port int
	LocalIPAddress string  // of this machine, for Twistlock to call back
	Params map[string]string
}

func (twistlockSvc *TwistlockService) GetName() string { return "twistlock" }

func CreateTwistlockService(params map[string]interface{}) (ScanService, error) {
	
	var host string
	var portStr string
	var localIPAddress string
	var isType bool
	
	host, isType = params["Host"].(string)
	if host == "" { return nil, utils.ConstructUserError("Parameter 'Host' not specified") }
	if ! isType { return nil, utils.ConstructUserError("Parameter 'Host' is not a string") }

	portStr, isType = params["Port"].(string)
	if portStr == "" { return nil, utils.ConstructUserError("Parameter 'Port' not specified") }
	if ! isType { return nil, utils.ConstructUserError("Parameter 'Port' is not a string") }

	localIPAddress, isType = params["LocalIPAddress"].(string)
	if localIPAddress == "" { return nil, utils.ConstructUserError("Parameter 'localIPAddress' not specified") }
	if ! isType { return nil, utils.ConstructUserError("Parameter 'localIPAddress' is not a string") }
	
	var port int
	var err error
	port, err = strconv.Atoi(portStr)
	if err != nil { return nil, err }
	
	return &TwistlockService{
		UseSSL: false,
		Host: host,
		Port: port,
		LocalIPAddress: localIPAddress,
		Params: map[string]string{
			"MinimumPriority": "The minimum priority level of vulnerabilities to report",
		},
	}, nil
}

func (twistlockSvc *TwistlockService) GetEndpoint() string {
	return fmt.Sprintf("http://%s:%d", twistlockSvc.Host, twistlockSvc.Port)
}

func (twistlockSvc *TwistlockService) GetParameterDescriptions() map[string]string {
	return twistlockSvc.Params
}

func (twistlockSvc *TwistlockService) GetParameterDescription(name string) (string, error) {
	var desc string = twistlockSvc.Params[name]
	if desc == "" { return "", utils.ConstructUserError("No parameter named '" + name + "'") }
	return desc, nil
}

func (twistlockSvc *TwistlockService) CreateScanContext(params map[string]string) (ScanContext, error) {
	
	var minPriority string
	
	if params != nil {
		minPriority = params["MinimumPriority"]
		// this param is optional so do not require its presence.
	}
	
	var scheme string
	if twistlockSvc.UseSSL { scheme = "https" } else { scheme = "http" }
	
	return &TwistlockRestContext{
		RestContext: *rest.CreateTCPRestContext(scheme,
			twistlockSvc.Host, twistlockSvc.Port, "", "", setTwistlockSessionId),
		MinimumVulnerabilityPriority: minPriority,
		TwistlockService: twistlockSvc,
		sessionId: "",
	}, nil
}

func (twistlockSvc *TwistlockService) AsScanProviderDesc() *apitypes.ScanProviderDesc {
	var params = []apitypes.ParameterInfo{}
	for name, desc := range twistlockSvc.Params {
		params = append(params, *apitypes.NewParameterInfo(name, desc))
	}
	return apitypes.NewScanProviderDesc(twistlockSvc.GetName(), params)
}

/*******************************************************************************
 * For accessing the Twistlock scanning service.
 */
type TwistlockRestContext struct {
	rest.RestContext
	MinimumVulnerabilityPriority string
	TwistlockService *TwistlockService
	sessionId string
}

func (twistlockContext *TwistlockRestContext) getEndpoint() string {
	return twistlockContext.TwistlockService.GetEndpoint()
}

func (twistlockContext *TwistlockRestContext) PingService() *apitypes.Result {
	var apiVersion string
	var engineVersion string
	var err error
	apiVersion, engineVersion, err = twistlockContext.GetVersions()
	if err != nil { return apitypes.NewResult(500, err.Error()) }
	return apitypes.NewResult(200, fmt.Sprintf(
		"Service is up: api version %s, engine version %s", apiVersion, engineVersion))
}

/*******************************************************************************
 * 
 */
func (twistlockContext *TwistlockRestContext) ScanImage(imageName string) (*ScanResult, error) {
	
	fmt.Println("Initiating image scan")
	var err error = twistlockContext.initiateScan(imageName)
	if err != nil {
		return nil, err
	}
	
	/* Obtain scan results.
		Unfortunately, the scan call is non-blocking and there is no way to tell
		when it completes, so we have to poll.
		The call will return an array, in which the first object contains these elements:
			scanTime - time scan was performed, e.g., "2017-09-02T17:01:43.265Z".
			info.cveVulnerabilities - an array of objects containing these string-valued attributes:
				id
				link
				severity
				description
	*/
	var vulnerabilities []interface{}
	var numberOfTries = 0
	for _, _ {
		numberOfTries++
		if numberOfTries > MaxNumberOfTries {
			return nil, utils.ConstructUserError("Timed out waiting for scan result")
		}
		vulnerabilities, scanCompletionTime, err = twistlockContext.getVulnerabilities(imageName);
		if err != nil {
			
		}
		
		if scanCompletionTime.Before(time.Now()) {  // scan is the one that we initiated, or later
			break
		}
	}
	
	var info? interface{} = responseMap["info"]  // should be an map[string]
	var info map[string]interface{}
	var isType bool
	info, isType = info?.(map[string]interface{})
	if ! isType {
		return nil, utils.ConstructUserError("Unexpected json object type for info field")
	}
	
	var vulnerabilities? interface{} = info["cveVulnerabilities"] // should be an array of objects
	var vulnerabilities, isType = vulnerabilities?.([]interface{})
	if ! isType {
		return nil, utils.ConstructUserError("Unexpected json object type for cveVulnerabilities field")
	}
	
	if len(vulnerabilities) == 0 {
		fmt.Println("No vulnerabilities found for image")
	}

	var vulnDescs = make([]*apitypes.VulnerabilityDesc, len(vulnerabilities))
	for i, vuln? := range vulnerabilities {
		var vuln map[string]interface{}
		vuln, isType = vuln?.(map[string]interface{})
		if ! isType {
			return nil, utils.ConstructUserError("Unexpected json object type for a cveVulnerability")
		}
		
		....add checking to this
		vulnDescs[i] = apitypes.NewVulnerabilityDesc(
			vuln["id"], vuln["link"], vuln["severity"], vuln["description"])
	}
	
	return &ScanResult{
		Vulnerabilities: vulnDescs,
	}, nil
}


/**************************** Twistlock Service Methods ***************************
 ******************************************************************************/


/*******************************************************************************
 * 
 */
func (twistlockContext *TwistlockRestContext) GetVersions() (apiVersion string, engineVersion string, err error) {

	var resp *http.Response
	resp, err = twistlockContext.SendSessionGet(twistlockContext.sessionId, "v1/versions", nil, nil)
	
	if err != nil { return "", "", err }
	defer resp.Body.Close()
	
	twistlockContext.Verify200Response(resp)

	var responseMap map[string]interface{}
	responseMap, err = rest.ParseResponseBodyToMap(resp.Body)
	if err != nil { return "", "", err }
	var isType bool
	apiVersion, isType = responseMap["APIVersion"].(string)
	if ! isType { return "", "", utils.ConstructServerError("Value returned for APIVersion is not a string") }
	engineVersion, isType = responseMap["EngineVersion"].(string)
	if ! isType { return "", "", utils.ConstructServerError("Value returned for EngineVersion is not a string") }
	return apiVersion, engineVersion, nil
}

func (twistlockContext *TwistlockRestContext) GetHealth() string {
	//resp = get("v1/health")
	return ""
}


/**************************** Internal Implementation Methods ***************************
 ******************************************************************************/

type APIVulnerabilitiesResponse struct {
	Vulnerabilities []Vulnerability
}

/*******************************************************************************
 * 
 */
func initiateScan(imageName string) error {
	
	/* Perform scan.
		The call to initiate a scan is of the form,
			curl -k -u admin:admin -H "Content-Type: application/json" -d '{"tag":{"registry":"","repo":"scaledmarkets/taskruntime"}}' -X POST http://localhost:8081/api/v1/registry/scan
	*/
	layerURL, layerId, priorLayerId
	var jsonPayload string = fmt.Sprintf("{\"tag\": {" +
		"\"registry\": \"%s\", " +
		"\"repo\": \"%s\"}}",
		registryName, imagePath)
	
	var url = twistlockContext.getEndpoint() + "registry/scan" + postLayerURI
	fmt.Println("Sending request to twistlock:")
	fmt.Println("POST " + url + " " + string(jsonPayload))

	request, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(jsonPayload)))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	....Set user/password
	....Set to ignore cert chain

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
	
}

/*******************************************************************************
 * 
 */
func getVulnerabilities(imageName string) ([]interface{}, time.Time, error) () {
	
	/*
		The call to obtain a scan result is of the form,
			curl -k -u admin:admin https://localhost:8083/api/v1/registry?repository='scaledmarkets/taskruntime'
	*/
	
	....var url = endpoint + fmt.Sprintf(getLayerVulnerabilitiesURI, layerID)
	fmt.Println(url)
	
	response, err := http.Get(url)
	if err != nil {
		return []Vulnerability{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		body, _ := ioutil.ReadAll(response.Body)
		return []Vulnerability{}, fmt.Errorf("Got response %d with message %s", response.StatusCode, string(body))
	}

	....no
	var apiResponse APIVulnerabilitiesResponse
	err = json.NewDecoder(response.Body).Decode(&apiResponse)
	if err != nil {
		return []Vulnerability{}, err
	}

	return apiResponse.Vulnerabilities, nil
}

/*******************************************************************************
 * Set the session Id as a cookie.
 */
func setTwistlockSessionId(req *http.Request, sessionId string) {
	
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
