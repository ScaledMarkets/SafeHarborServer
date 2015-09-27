/*******************************************************************************
 * This file contains all declarations related to Server.
 */

package main

import (
	"fmt"
	"net"
	"net/http"
	"mime/multipart"
	"net/url"
	"io"
	//"io/ioutil"
	//"path/filepath"
	"os"
	"strings"
	"crypto/x509"
	"errors"
	//"strconv"
)

/*******************************************************************************
 * A singleton Server is created by main to service all incoming HTTP requests.
 */
type Server struct {
	Config *Configuration
	httpServer *http.Server
	tcpListener net.Listener
	dbClient *InMemClient
	http.Handler
	certPool *x509.CertPool
	authService *AuthService
	dispatcher *Dispatcher
	sessions map[string]*Credentials  // map session key to Credentials.
	Debug bool
}

/*******************************************************************************
 * Create a Server structure. This includes reading in the auth server cert.
 */
func NewServer(debug bool) *Server {
	
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
		Debug: debug,
		Config:  config,
		certPool: certPool,
		dispatcher: dispatcher,
	}
	
	server.dbClient = NewInMemClient(server)
	
	// Ensure that the file repository exists.
	if ! fileExists(server.Config.FileRepoRootPath) {
		fmt.Println("Repository does not exist,", server.Config.FileRepoRootPath)
		return nil
	}
	
	dispatcher.server = server
	
	server.authService = NewAuthService(config.service,
		config.AuthServerName, config.AuthPort, certPool)
	
	// To do: Make this a TLS listener.
	// Instantiate an HTTP server with the SafeHarbor server as the handler.
	// See https://golang.org/pkg/net/http/#Server
	server.httpServer = &http.Server{
		Handler: server.getHttpHandler(),
	}

	// Instantiate a TCP socker listener.
	fmt.Println("...Creating socket listener on port", config.port, "...")
	server.tcpListener, err = newTCPListener(config.ipaddr, config.port)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1);
	}
	
	// Verify that the docker daemon is running, and start it if not.
	// sudo service docker start
	// ....
	
	// Verify that system has python 2.
	// ....
	
	
	//....To do: Install a ^C signal handler, to gracefully shut down.
	//....To do: Ensure that request handlers are re-entrant (or guarded re-entrant).
	
	return server
}

/*******************************************************************************
 * 
 */
func (server *Server) start() {
	// Start listening for incoming HTTP requests.
	// Creates a new service goroutine for each incoming connection on tcpListener.
	// Each service goroutine reads requests and then calls httpServer.Handler
	// to reply to them. See https://golang.org/pkg/net/http/#Server.Serve
	defer server.tcpListener.Close()
	fmt.Println("...Starting service...")
	if err := server.httpServer.Serve(server.tcpListener); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

/*******************************************************************************
 * Gracefully stop the server. No work in progress is aborted.
 */
func (server *Server) stop() {
	fmt.Println("...Stopping service...")
	server.stopAcceptingRequests()
	server.waitUntilNoRequestsInProgress()
	fmt.Println("...stop.")
	os.Exit(0)
}

/*******************************************************************************
 * 
 */
func (server *Server) stopAcceptingRequests() {
	// Set flag for dispatcher that no more requests should be accepted.
	// ....
}

/*******************************************************************************
 * Blocks until the set of dispatched requests is empty.
 */
func (server *Server) waitUntilNoRequestsInProgress() {
	// ....
}

/*******************************************************************************
 * 
 */
func (server *Server) resumeAcceptingRequests() {
	// ....
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
	
	sessionToken = server.authService.authenticateRequest(httpReq)
	if sessionToken == nil { //return authent failure
		writer.WriteHeader(401)
		return
	}
	
	server.dispatch(sessionToken, writer, httpReq)
}

/*******************************************************************************
 * Interpret the request string to determine which method is being requested,
 * and invoke the requested method.
 */
func (server *Server) dispatch(sessionToken *SessionToken,
	writer http.ResponseWriter, httpReq *http.Request) {

	fmt.Println("Dispatching request")
	
	var err error
	var httpMethod string = strings.ToUpper(httpReq.Method)
	var reqName string = strings.Trim(httpReq.URL.Path, "/ ")
	var headers http.Header = httpReq.Header  // map[string][]string
	var values url.Values
	var files map[string][]*multipart.FileHeader = nil
	
	if httpMethod == "GET" {
		
		if err = httpReq.ParseForm(); err != nil { panic(err) }
		values = httpReq.Form  // map[string]string
		
	} else if httpMethod == "POST" {  // dispatch to an error handler.
	
		// Authorization for a request should be performed using only the intersection
		// of the authority of the user and the requesting origin(s). 
		// Thus, if the request origin is the SafeHarbor Web App origin, we merely
		// need to authorize the user; otherwise, we deny. In the future we should
		// allow users to register trusted origins.
		
		if err = httpReq.ParseForm(); err != nil { panic(err) }
		values = httpReq.PostForm  // map[string]string
		
		// Check if the POST is multipart/form-data.
		// https://golang.org/pkg/net/http/#Request.MultipartReader
		// http://www.w3.org/TR/html401/interact/forms.html#h-17.13.4
		var mpReader *multipart.Reader
		mpReader, err = httpReq.MultipartReader()
		if mpReader != nil { // has multipart data
			// We require all multipart requests to include one (and only one) file part.
			fmt.Println("has multipart data...")
			
			// https://golang.org/pkg/mime/multipart/#Reader.ReadForm
			var form *multipart.Form
			form, err = mpReader.ReadForm(10000)
			if err != nil { panic(err) }
			
			values = form.Value
			files = form.File
		}

	} else {
		respondMethodNotSupported(writer, httpReq.Method)
		return
	}

	fmt.Println("Calling handleRequest")
	server.dispatcher.handleRequest(sessionToken, headers, writer, reqName, values, files)
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
 Read the configuration file and return a new Configuration struct.
 */
func getConfiguration() (*Configuration, error) {
	
	configurationPath := os.Getenv("SAFEHARBOR_CONFIGURATION_PATH")

	if configurationPath == "" {
		configurationPath = "conf.json"  // try the default location
	}

	var file *os.File
	var err error

	file, err = os.Open(configurationPath)
	if err != nil {
		return nil, fmt.Errorf("Could not open configuration file (usually conf.json)")
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
func newTCPListener(ipaddr string, port int) (net.Listener, error) {
	tcpListener, err := net.Listen("tcp", fmt.Sprintf("%[1]s:%[2]d", ipaddr, port))
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


/*******************************************************************************
 * 
 */
func respondMethodNotSupported(writer http.ResponseWriter, methodName string) {
	writer.WriteHeader(405)
	io.WriteString(writer, "HTTP method not supported:" + methodName)
}
