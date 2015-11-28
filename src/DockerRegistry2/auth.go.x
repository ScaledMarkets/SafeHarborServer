package DockerRegistry2

import (
	"fmt"
	"net/http"
	"net/url"
	"io"
	"strings"
	"crypto/tls"

/*******************************************************************************
 * Verify that the credentials match a registered user. If so, return a session
 * token that can be used to validate subsequent requests. This, in effect,
 * creates a new user session.
 */
func (authSvc *AuthService) authenticateCredentials(creds *Credentials) *SessionToken {
	
	/***************
	// Access the auth server to authenticate the credentials.
	//if ! authSvc.sendQueryToAuthServer(creds, authSvc.Service,
	//	creds.userid, "", "", []string{}) { return nil }
	***************/
	
	var sessionId string = authSvc.createUniqueSessionId()
	var token *SessionToken = NewSessionToken(sessionId, creds.UserId)
	
	// Cache the new session token, so that this Server can recognize it in future
	// exchanges during this session.
	authSvc.Sessions[sessionId] = creds
	
	return token
}

/*******************************************************************************
 * Check if the specified account is allowed to have access to the specified
 * resource. This function does not authenticate the
 * account - that is done by authenticateCredentials().
 * https://stackoverflow.com/questions/24496344/golang-send-http-request-with-certificate
 */
func (authSvc *AuthService) authorized(creds *Credentials, account string, 
	scope_type string, scope_name string, scope_actions []string) bool {

	return true  //....Remove!!!!!!!!!!!!!!
	

//	return authSvc.sendQueryToAuthServer(creds, authSvc.Service,
//		creds.userid, scope_type, scope_name, scope_actions)
}

/*******************************************************************************
 * Return the userid and password from the HTTP header, or nil if not present.
 */
func getSessionBasicAuthCreds(httpReq *http.Request) *Credentials {
	userId, password, ok := httpReq.BasicAuth()
	if !ok { return nil }
	return NewCredentials(userId, password)
}

/*******************************************************************************
 * Establish a TLS connection with the authentication/authorization server.
 * This connection is maintained.
 */
func connectToAuthServer(certPool *x509.CertPool) *http.Client {
	
	var tr *http.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: certPool},
		DisableCompression: true,
	}
	return &http.Client{Transport: tr}
}

/*******************************************************************************
 * Send an authentication or authorization request to the auth server. If successful,
 * return true, otherwise return false. This function encapsulates the HTTP messsage
 * format required by the auth server.
 */
func (authSvc *AuthService) sendQueryToAuthServer(creds *Credentials, 
	service string, account string,
	scope_type string, scope_name string, scope_actions []string) bool {
	
	// https://github.com/cesanta/docker_auth/blob/master/auth_server/server/server.go
	var urlstr string = fmt.Sprintf(
		"https://%s:%s/auth",
		authSvc.AuthServerName, authSvc.AuthPort)
	
	var request *http.Request
	var err error
	var actions string = strings.Join(scope_actions, ",")
	var scope string = fmt.Sprintf("%s%s%s", scope_type, scope_name, actions)
	var data url.Values = url.Values {
		"service": []string{service},
		"scope": []string{scope},
		"account": []string{creds.UserId},
	}
	var reader io.Reader = strings.NewReader(data.Encode())
	
	request, err = http.NewRequest("POST", urlstr, reader)
		if err != nil { panic(err) }
	request.SetBasicAuth(creds.UserId, creds.Password)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var resp *http.Response
	resp, err = authSvc.AuthClient.Do(request)
	if err != nil {
		fmt.Println(err.Error())
		return false
	}
	if resp.StatusCode != 200 {
		fmt.Println(fmt.Sprintf("Response code %s", resp.StatusCode))
		return false
	}
	
	defer resp.Body.Close()
	
	return true
}

