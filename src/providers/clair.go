package providers

import (
	"errors"
	"net/http"
	"fmt"
	
	// My packages:
	"apitypes"
	"rest"
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
	var health string = clairContext.getHealth()
	if health == "" { return apitypes.NewResult(500, health) } else {
		return apitypes.NewResult(200, health) }
}

/*******************************************************************************
 * See https://github.com/coreos/clair/blob/master/contrib/analyze-local-images/main.go
 */
func (clairContext *ClairRestContext) ScanImage(imageName string) *apitypes.Result {
	
	// Use the docker 'save' command to extract image to a tar of tar files.
	// Must be extracted to a temp directory that is shared with the clair container.
	
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
