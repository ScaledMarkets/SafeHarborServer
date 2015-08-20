/*******************************************************************************
 * This file contains all declarations related to Server.
 */
package main

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"io"
	"os"
	"strings"
	"crypto/tls"
	"crypto/x509"
	"errors"
)

/*******************************************************************************
 * A singleton Server is created by main to service all incoming HTTP requests.
 */
type Server struct {
	Config *Configuration
	client *http.Client
	dbClient *InMemClient
	http.Handler
	//AuthSvc AuthorizationService
	certPool *x509.CertPool
	dispatcher *Dispatcher
	sessions map[string]*Credentials  // map session key to Credentials.
}

/*******************************************************************************
 * Create a Server structure. This includes reading in the auth server cert.
 */
func NewServer() *Server {
	
	// Read configuration. (Defined in a JSON file.)
	fmt.Println("Reading configuration")
	var config *Configuration
	var err error
	config, err = getConfiguration()
	if err != nil {
		panic(err)
	}
	
	var certPool *x509.CertPool = getCerts(config)
	
	var dispatcher = NewDispatcher()
	
	// Construct a Server with the configuration and cert pool.
	var server *Server = &Server{
		Config:  config,
		certPool: certPool,
		dispatcher: dispatcher,
	}
	
	server.dbClient = NewInMemClient()
	
	dispatcher.server = server
	
	server.connectToAuthServer()
	
	// To do: Make this a TLS listener.
	// Instantiate an HTTP server with the SafeHarbor server as the handler.
	// See https://golang.org/pkg/net/http/#Server
	var httpServer *http.Server = &http.Server{
		Handler: server.getHttpHandler(),
	}

	// Instantiate a TCP socker listener.
	fmt.Println("...Creating socket listener on port ", config.port, "...")
	var tcpListener net.Listener
	tcpListener, err = newTCPListener(config.ipaddr, config.port)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1);
	}
	defer tcpListener.Close()
	
	//....To do: Install a ^C signal handler, to gracefully shut down.
	//....To do: Ensure that request handlers are re-entrant (or guarded re-entrant).
	
	// Start listening for incoming HTTP requests.
	// Creates a new service goroutine for each incoming connection on tcpListener.
	// Each service goroutine reads requests and then calls httpServer.Handler
	// to reply to them. See https://golang.org/pkg/net/http/#Server.Serve
	fmt.Println("...Starting service...")
	if err := httpServer.Serve(tcpListener); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	
	return server
}

/*******************************************************************************
 * Build a Certificate data structure by reading the file at the specified path.
 */
func getCert(certPath string) *x509.Certificate {
	
	file, err := os.Open(certPath)
	if err != nil {
		fmt.Println(fmt.Sprintf("Could not open certificate at %s", certPath))
		panic(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			panic(err)
		}
	}()
	var fileInfo os.FileInfo
	fileInfo, err = file.Stat()
	if err != nil { panic(err) }
	var fileLength = fileInfo.Size()
	var asn1DataBuf = make([]byte, fileLength)
	var n int
	n, err = file.Read(asn1DataBuf)
	if err != nil && err != io.EOF {
		panic(err)
	}
	if int64(n) != fileLength {
		panic(errors.New("Number of bytes read for cert does not match file length"))
	}
	
	// Construct a certificate from the bytes that were read.
	var cert *x509.Certificate
	cert, err = x509.ParseCertificate(asn1DataBuf)
	if err != nil {
		panic(err)
	}
	// to do:....check signature and CRL
	
	return cert
}

/*******************************************************************************
 * The HTTP level handler - not to be confused with the application level
 * handlers that the Dispatcher dispatches to.
 */
func (server *Server) getHttpHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				fmt.Sprintf("%v", err)
			}
		}()
		server.ServeHTTP(w, r)
	})
}

/*******************************************************************************
 * Entry point for each incoming HTTP request.
 */
func (server *Server) ServeHTTP(writer http.ResponseWriter, httpReq *http.Request) {
	
	defer httpReq.Body.Close() // ensure that request body is always closed.

	// Set a header with the Docker Distribution API Version for all responses.
	writer.Header().Add("SafeHarbor-API-Version", "safeharbor/2.0")
	
	// Authenitcate session or user.
	var sessionToken *SessionToken = nil
	
	/***********************
	 Commented out until I complete the authentication mechanism with Cesanta.
	var sessionId = getSessionId(httpReq)
	if sessionId != "" {
		sessionToken = server.validateSessionId(sessionId)
	}
	if sessionToken == nil { // authenticate basic credentials
		var creds *Credentials = getSessionBasicAuthCreds(httpReq)
		if creds != nil {
			sessionToken = server.authenticated(creds)
		}
	}
	if sessionToken == nil { //return authent failure
		writer.WriteHeader(401)
		return
	}
	***********************/
	
	server.dispatch(sessionToken, writer, httpReq)
}

/*******************************************************************************
 * Returns the session id header value, or "" if there is none.
 */
func getSessionId(httpReq *http.Request) string {
	var sessionId string = httpReq.Header["SessionId"][0]
	if len(sessionId) == 0 { return "" }
	if len(sessionId) > 1 { panic(errors.New("Ill-formed session id")) }
	return sessionId
}

/*******************************************************************************
 * Return the userid and password from the HTTP header, or nil if not present.
 */
func getSessionBasicAuthCreds(httpReq *http.Request) *Credentials {
	userid, password, ok := httpReq.BasicAuth()
	if !ok { return nil }
	return NewCredentials(userid, password)
}

/*******************************************************************************
 * Validate the specified session id. If valid, return a SessionToken with
 * the identity of the session's owner.
 */
func (server *Server) validateSessionId(sessionId string) *SessionToken {
	var credentials *Credentials = server.sessions[sessionId]
	if credentials == nil { return nil }
	
	return NewSessionToken(sessionId, credentials.userid)
}

/*******************************************************************************
 * Interpret the request string to determine which method is being requested,
 * and invoke the requested method.
 */
func (server *Server) dispatch(sessionToken *SessionToken,
	writer http.ResponseWriter, httpReq *http.Request) {

	fmt.Println("Dispatching request")
	
	// Retrieve the request name and arguments from the HTTP request.
	var reqName string = strings.Trim(httpReq.URL.Path, "/ ")
	if err := httpReq.ParseForm(); err != nil { panic(err) }
	var values url.Values = httpReq.PostForm  // map[string]string
	
	server.dispatcher.handleRequest(sessionToken, writer, reqName, values)
}

/*******************************************************************************
 * Read certificate files and add certs to cert pool.
 */
func getCerts(config *Configuration) *x509.CertPool {

	var certPool *x509.CertPool = x509.NewCertPool()
	var rootCert *x509.Certificate = nil
	if config.LocalRootCertPath != "" {
		rootCert = getCert(config.LocalRootCertPath)
		certPool.AddCert(rootCert)
	}
	var authCert *x509.Certificate = getCert(config.LocalAuthCertPath)
	certPool.AddCert(authCert)
	return certPool
}

/*******************************************************************************
 * Establish a TLS connection with the authentication/authorization server.
 * This connection is maintained.
 */
func (server *Server) connectToAuthServer() {
	
	var tr *http.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: server.certPool},
		DisableCompression: true,
	}
	server.client = &http.Client{Transport: tr}
}

/*******************************************************************************
 * Verify that the credentials match a registered user. If so, return a session
 * token that can be used to validate subsequent requests.
 */
func (server *Server) authenticated(creds *Credentials) *SessionToken {
	
	// Access the auth server to authenticate the credentials.
	if ! server.sendQueryToAuthServer(creds, server.Config.service,
		creds.userid, "", "", []string{}) { return nil }
	
	var sessionId string = server.createUniqueSessionId()
	var token *SessionToken = NewSessionToken(sessionId, creds.userid)
	
	// Cache the new session token, so that this Server can recognize it in future
	// exchanges during this session.
	server.sessions[sessionId] = creds
	
	return token
}

/*******************************************************************************
 * Check if the specified account is allowed to have access to the specified
 * resource. This function does not authenticate the
 * account - that is done by authenticated().
 * https://stackoverflow.com/questions/24496344/golang-send-http-request-with-certificate
 */
func (server *Server) authorized(creds *Credentials, account string, 
	scope_type string, scope_name string, scope_actions []string) bool {

	return server.sendQueryToAuthServer(creds, server.Config.service,
		creds.userid, scope_type, scope_name, scope_actions)
}

/*******************************************************************************
 * Send an authentication or authorization request to the auth server. If successful,
 * return true, otherwise return false. This function encapsulates the HTTP messsage
 * format required by the auth server.
 */
func (server *Server) sendQueryToAuthServer(creds *Credentials, 
	service string, account string,
	scope_type string, scope_name string, scope_actions []string) bool {
	
	// https://github.com/cesanta/docker_auth/blob/master/auth_server/server/server.go
	var urlstr string = fmt.Sprintf(
		"https://%s:%s/auth",
		server.Config.AuthServerName, server.Config.AuthPort)
	
	var request *http.Request
	var err error
	var actions string = strings.Join(scope_actions, ",")
	var scope string = fmt.Sprintf("%s%s%s", scope_type, scope_name, actions)
	var data url.Values = url.Values {
		"service": []string{service},
		"scope": []string{scope},
		"account": []string{creds.userid},
	}
	var reader io.Reader = strings.NewReader(data.Encode())
	
	request, err = http.NewRequest("POST", urlstr, reader)
		if err != nil { panic(err) }
	request.SetBasicAuth(creds.userid, creds.pswd)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var resp *http.Response
	resp, err = server.client.Do(request)
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

/*******************************************************************************
 Read the configuration file and return a new Configuration struct.
 */
func getConfiguration() (*Configuration, error) {
	configurationPath := os.Getenv("SAFEHARBOR_CONFIGURATION_PATH")

	if configurationPath == "" {
		return nil, fmt.Errorf("configuration path unspecified")
	}

	var file *os.File
	var err error
	file, err = os.Open(configurationPath)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	config, err := NewConfiguration(file)
	if err != nil {
		return nil, err
	}
	
	return config, nil
}

/*******************************************************************************
 * Return a new keep-alive TCP socker listener for the specified IP:port address.
 */
func newTCPListener(ipaddr, port string) (net.Listener, error) {
	tcpListener, err := net.Listen("tcp", fmt.Sprintf("%[1]s:%[2]s", ipaddr, port))
	if err != nil {
		return nil, err
	}

	return tcpKeepAliveListener{tcpListener.(*net.TCPListener)}, nil
}

/*******************************************************************************
 * 
 */
type tcpKeepAliveListener struct {
	*net.TCPListener
}
