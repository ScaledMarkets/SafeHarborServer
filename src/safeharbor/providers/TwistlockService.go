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

ScanResultWaitIntervalMs int
MaxNumberOfTries int

type TwistlockService struct {
	UseSSL bool
	Host string
	Port int
	UserId string
	Password string
	//LocalIPAddress string  // of this machine, for Twistlock to call back
	Params map[string]string
}

func (twistlockSvc *TwistlockService) GetName() string { return "twistlock" }

func CreateTwistlockService(params map[string]interface{}) (ScanService, error) {
	
	var host string
	var portStr string
	var userId string
	var password string
	//var localIPAddress string
	var isType bool
	
	host, isType = params["Host"].(string)
	if host == "" { return nil, utils.ConstructUserError("Parameter 'Host' not specified") }
	if ! isType { return nil, utils.ConstructUserError("Parameter 'Host' is not a string") }

	portStr, isType = params["Port"].(string)
	if portStr == "" { return nil, utils.ConstructUserError("Parameter 'Port' not specified") }
	if ! isType { return nil, utils.ConstructUserError("Parameter 'Port' is not a string") }

	userId, isType = params["UserId"].(string)
	if userId == "" { return nil, utils.ConstructUserError("Parameter 'UserId' not specified") }
	if ! isType { return nil, utils.ConstructUserError("Parameter 'UserId' is not a string") }

	password, isType = params["Password"].(string)
	if password == "" { return nil, utils.ConstructUserError("Parameter 'Password' not specified") }
	if ! isType { return nil, utils.ConstructUserError("Parameter 'Password' is not a string") }
	
	//localIPAddress, isType = params["LocalIPAddress"].(string)
	//if localIPAddress == "" { return nil, utils.ConstructUserError("Parameter 'localIPAddress' not specified") }
	//if ! isType { return nil, utils.ConstructUserError("Parameter 'localIPAddress' is not a string") }
	
	var port int
	var err error
	port, err = strconv.Atoi(portStr)
	if err != nil { return nil, err }
	
	return &TwistlockService{
		UseSSL: true,
		Host: host,
		Port: port,
		UserId: userId,
		Password: password,
		//LocalIPAddress: localIPAddress,
		Params: map[string]string{
			"UserId": "User id for connecting to the Twistlock server",
			"Password": "Password for connecting to the Twistlock server",
		},
	}, nil
}

func (twistlockSvc *TwistlockService) GetEndpoint() string {
	var scheme string
	if twistlockSvc.UseSSL {
		scheme = "https"
	} else {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s:%d/api/v1", scheme, twistlockSvc.Host, twistlockSvc.Port)
}

func (twistlockSvc *TwistlockService) GetParameterDescriptions() map[string]string {
	return twistlockSvc.Params
}

func (twistlockSvc *TwistlockService) GetParameterDescription(name string) (string, error) {
	var desc string = twistlockSvc.Params[name]
	if desc == "" { return "", utils.ConstructUserError("No parameter named '" + name + "'") }
	return desc, nil
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
	//MinimumVulnerabilityPriority string
	TwistlockService *TwistlockService
	sessionId string
}

var _ TwistlockRestContext = &ScanContext{}

func (twistlockSvc *TwistlockService) CreateScanContext(params map[string]string) (ScanContext, error) {
	
	var minPriority string
	
	var scheme string
	if twistlockSvc.UseSSL { scheme = "https" } else { scheme = "http" }
	
	var sessionToken string
	sessionToken, err = authenticate(twistlockSvc.UserId, twistlockSvc.Password)
	if err != nil {
		return nil, err
	}
	
	var TwistlockRestContext context = &TwistlockRestContext{
		RestContext: *rest.CreateTCPRestContext(scheme,
			twistlockSvc.Host, twistlockSvc.Port, "", "", setTwistlockSessionId),
		//MinimumVulnerabilityPriority: minPriority,
		TwistlockService: twistlockSvc,
		sessionId: sessionToken,
	}
	
	err = context.authenticate(twistlockSvc.UserId, twistlockSvc.Password)
	if err != nil {
		return nil, err
	}
	
	return context, nil
}

/*
 * See https://twistlock.desk.com/customer/en/portal/articles/2831956-twistlock-api-2-1#authenticate
 */
func (twistlockContext *TwistlockRestContext) authenticate(string userId, password) error {
	
	....Authenticate to Twistlock server, to obtain session token.
	twistlockContext.sessionId = ....
	nil
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
	var err error = twistlockContext.initiateScan(....registryName, ....repoName)
	if err != nil {
		return nil, err
	}
	
	/* Obtain scan results.
		Unfortunately, the scan call is non-blocking and there is no way to tell
		when it completes, so we have to poll.
		The call will return an array, in which the first object contains these elements:
			scanTime - time scan was performed, e.g., "2017-09-02T17:01:43.265Z".
			info.cveVulnerabilities - either null, or an array of objects containing
			these string-valued attributes:
				id
				link
				severity
				description
	*/
	var vulnerabilities []interface{}
	var numberOfTries = 0
	for _, _ { // until we obtain an up to date scan result, or reach max # of tries
		numberOfTries++
		if numberOfTries > MaxNumberOfTries {
			return nil, utils.ConstructUserError("Timed out waiting for scan result")
		}
		vulnerabilities, scanCompletionTime, err = twistlockContext.getVulnerabilities(imageName);
		if err != nil {
			return nil, err
		}
		
		if scanCompletionTime.Before(time.Now()) {  // scan is the one that we initiated, or later
			break  // because we found a recent enough scan result
		}
		
		// Sleep for ScanResultWaitIntervalMs milliseconds.
		time.Sleep(ScanResultWaitIntervalMs * time.Millisecond)
	}
	
	// Parse the scan result.
	
	var info? interface{} = responseMap["info"]  // should be an map[string]
	var info map[string]interface{}
	var isType bool
	info, isType = info?.(map[string]interface{})
	if ! isType {
		return nil, utils.ConstructUserError("Unexpected json object type for info field")
	}
	
	var vulnerabilities []interface{}
	var vulnerabilities? interface{} = info["cveVulnerabilities"] // should be an array of objects
	if vulnerabilities? == nil {
		// No vulnerabilities found.
		vulnerabilities = make([]interface{}, 0)
	} else {
		vulnerabilities, isType = vulnerabilities?.([]interface{})
		if ! isType {
			return nil, utils.ConstructUserError("Unexpected json object type for cveVulnerabilities field")
		}
	}
	
	var vulnDescs = make([]*apitypes.VulnerabilityDesc, len(vulnerabilities))
	for i, vuln? := range vulnerabilities {
		var vuln map[string]interface{}
		vuln, isType = vuln?.(map[string]interface{})
		if ! isType {
			return nil, utils.ConstructUserError("Unexpected json object type for a cveVulnerability")
		}
		
		var id, link, severity, description string
		id, isType = vuln["id"}.(string)
		if ! isType {
			return nil, utils.ConstructUserError("Unexpected json object type for vulnerability id")
		}
		link, isType = vuln["link"].(string)
		if ! isType {
			return nil, utils.ConstructUserError("Unexpected json object type for vulnerability link")
		}
		severity, isType = vuln["severity"].(string)
		if ! isType {
			return nil, utils.ConstructUserError("Unexpected json object type for vulnerability severity")
		}
		description, isType = vuln["description"].(string)
		if ! isType {
			return nil, utils.ConstructUserError("Unexpected json object type for vulnerability description")
		}
	
		vulnDescs[i] = apitypes.NewVulnerabilityDesc(id, link, severity, description)
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
func (twistlockContext *TwistlockRestContext) initiateScan(registryName, repoName string) error {
	
	/* Perform scan.
		The call to initiate a scan is of the form,
			curl -k -u admin:admin -H "Content-Type: application/json" \
				-d '{"tag":{"registry":"","repo":"scaledmarkets/taskruntime"}}' \
				-X POST https://localhost:8081/api/v1/registry/scan
	*/
	layerURL, layerId, priorLayerId
	var jsonPayload string = fmt.Sprintf(
		"{\"tag\": {\"registry\": \"%s\", \"repo\": \"%s\"}}", registryName, repoName)
	
	var url = twistlockContext.getEndpoint() + "/registry/scan"
	fmt.Println("Sending request to twistlock:")
	fmt.Println("POST " + url + " " + string(jsonPayload))

	request, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(jsonPayload)))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	
	// Set session token.
	// See https://twistlock.desk.com/customer/en/portal/articles/2607258-accessing-the-api
	request.Header.Set("Authorization", "Bearer " + twistlockContext.sessionId)
	
	//....Set to ignore cert chain
	//....Need to use https

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		response.Body.Close()
		// Re-authenticate and try one more time.
		err = twistlockContext.authenticate(twistlockContext.TwistlockService.UserId,
			twistlockContext.TwistlockService.Password)
		if err != nil {
			return err
		}
		response, err := client.Do(request)
		if err != nil {
			response.Body.Close()
			return err
		}
	}
	defer response.Body.Close()

	if response.StatusCode >= 300 {
		body, _ := ioutil.ReadAll(response.Body)
		return fmt.Errorf("Got response %d with message %s", response.StatusCode, string(body))
	}
	
	return nil
}

/*******************************************************************************
 * 
 */
func (twistlockContext *TwistlockRestContext) getVulnerabilities(imageName string) ([]interface{}, time.Time, error) () {
	
	....How come we don''t have to specify the registry?
	
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
