/*******************************************************************************
 * This file contains all declarations related to Server.
 *
 * Copyright Scaled Markets, Inc.
 */

package server

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
	"runtime/debug"
	"time"
	//"errors"
	//"strconv"
	
	"goredis"
	
	// SafeHarbor packages:
	"safeharbor/apitypes"
	"docker"
	"scanners"
	"utilities"
)

/*******************************************************************************
 * A singleton Server is created by main to service all incoming HTTP requests.
 */
type Server struct {
	Config *Configuration
	PublicURL string
	httpServer *http.Server
	tcpListener net.Listener
	persistence *Persistence
	http.Handler
	certPool *x509.CertPool
	authService *AuthService
	DockerServices *docker.DockerServices
	ScanServices []scanners.ScanService
	EmailService *utilities.EmailService
	dispatcher *Dispatcher
	sessions map[string]*apitypes.Credentials  // map session key to Credentials.
	Authorize bool
	AllowToggleEmailVerification bool
	PerformEmailIdentityVerification bool
	MaxLoginAttemptsToRetain int
	InMemoryOnly bool
	Debug bool // for test only
	NoCache bool // for test only
	NoRegistry bool // for test only
}

/*******************************************************************************
 * Create a Server structure. This includes reading in the auth server cert.
 */
func NewServer(debug bool, nocache bool, stubScanners bool, useStubScannerFor map[string]*struct{},
	noauthor bool, allowToggleEmailVerification bool, publicHostname string, port int,
	adapter string, secretSalt string, inMemOnly bool, noRegistry bool) (*Server, error) {
	
	// Read configuration. (Defined in a JSON file.)
	fmt.Println("Reading configuration")
	var config *Configuration
	var err error
	config, err = getConfiguration()
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}
	
	// Override conf.json with any command line options.
	if port != 0 { config.port = port }
	if adapter != "" { config.netIntfName = adapter }
	if publicHostname != "" { config.PublicHostname = publicHostname }
	
	// Determine the IP address.
	config.ipaddr, err = utilities.DetermineIPAddress(config.netIntfName)
	if err != nil { return nil, err }
	if config.ipaddr == "" {
		fmt.Println("Did not find an IP4 address for network interface " + config.netIntfName)
		return nil, err
	}
	
	// Read all certificates.
	var certPool *x509.CertPool = getCerts(config)
	
	// Construct a Server with the configuration and cert pool.
	var server *Server = &Server{
		Debug: debug,
		NoCache: nocache,
		Authorize: (! noauthor),
		AllowToggleEmailVerification: allowToggleEmailVerification,
		PerformEmailIdentityVerification: (! allowToggleEmailVerification),
		Config:  config,
		certPool: certPool,
		dispatcher: NewDispatcher(),
		MaxLoginAttemptsToRetain: 5,
		InMemoryOnly: inMemOnly,
		NoRegistry: noRegistry,
	}
	
	// Identify public URL - needed so that the server can provide a URL/URI for
	// file resources to download.
	server.PublicURL = fmt.Sprintf("http://%s:%d", config.PublicHostname, config.port)
	
	// Ensure that the file repository exists.
	if ! fileExists(server.Config.FileRepoRootPath) {
		err = os.MkdirAll(server.Config.FileRepoRootPath, 0700)
		if err != nil { AbortStartup(
			"Unable to create directory '" + server.Config.FileRepoRootPath +
			"'; " + err.Error())
		}
	}
	
	// Tell dispatcher how to find server.
	server.dispatcher.server = server
	
	// Create authentication and authorization services.
	server.authService = NewAuthService(config.service,
		config.AuthServerName, config.AuthPort, certPool, secretSalt)
	
	// Connect to object database (redis).
	var redisClient *goredis.Redis = nil
	if ! server.InMemoryOnly {
		if config.RedisHost == "" { config.RedisHost = config.ipaddr }  // default to same host
		if config.RedisPort == 0 { config.RedisPort = 6379 }  // default for redis
		
		var network = "tcp"
		var db = 1
		var timeout = 5 * time.Second
		var maxidle = 1
		redisClient, err = goredis.Dial(&goredis.DialConfig{
			network,
			(config.RedisHost + ":" + fmt.Sprintf("%d", config.RedisPort)),
			db, config.RedisPswd, timeout, maxidle})
		
		if err != nil { AbortStartup("When connecting to redis: " + err.Error()) }
	}
	_, err = NewPersistence(server, redisClient)
	if err != nil { AbortStartup(err.Error()) }
	
	var engine docker.DockerEngine
	engine, err = docker.OpenDockerEngineConnection()
	if err != nil { AbortStartup("When connecting to docker engine: " + err.Error()) }
	
	var registry docker.DockerRegistry
	if ! server.NoRegistry {
		if config.RegistryHost == "" { AbortStartup("REGISTRY_HOST not set in configuration") }
		if config.RegistryPort == 0 { AbortStartup("REGISTRY_PORT not set in configuration") }
		if config.RegistryUserId == "" { AbortStartup("REGISTRY_USERID not set in configuration") }
		if config.RegistryPassword == "" { AbortStartup("REGISTRY_PASSWORD not set in configuration") }
		registry, err = docker.OpenDockerRegistryConnection(config.RegistryHost, config.RegistryPort,
			config.RegistryUserId, config.RegistryPassword)
		if err != nil { AbortStartup("When connecting to registry: " + err.Error()) }
	}
	server.DockerServices = docker.NewDockerServices(registry, engine)
	
	// To do: Make this a TLS listener.
	// Instantiate an HTTP server with the SafeHarbor server as the handler.
	// See https://golang.org/pkg/net/http/#Server
	server.httpServer = &http.Server{
		Handler: server.getHttpHandler(),
	}

	// Instantiate a TCP socker listener.
	fmt.Println("...Creating socket listener at", config.ipaddr, "port", config.port, "...")
	server.tcpListener, err = newTCPListener(config.ipaddr, config.port)
	if err != nil { AbortStartup("When creating socket listener: " + err.Error()) }
	
	// Verify that the docker service is running, and start it if not.
	// sudo service docker start
	// ....
	
	// Verify that system has python 2.
	// ....
	
	
	//....To do: Install a ^C signal handler, to gracefully shut down.
	//....To do: Ensure that request handlers are re-entrant (or guarded re-entrant).
	
	
	// Install the requested scanning services that are supported.
	//....To do: Dynamically link and load the scanning services that are requested,
	// so that we don't have to statically reference them below.
	
	var clairScanSvc scanners.ScanService
	var twistlockScanSvc scanners.ScanService
	var openscapScanSvc scanners.ScanService
	var obj interface{} = config.ScanServices["clair"]
	var isType bool
	if obj != nil {
		var clairConfig map[string]interface{}
		clairConfig, isType = obj.(map[string]interface{})
		if ! isType { AbortStartup("Configuration of clair services is ill-formed:") }
		clairConfig["LocalIPAddress"] = config.ipaddr
		if stubScanners || useStubScannerFor["clair"] != nil {
			clairScanSvc, err = scanners.CreateClairServiceStub(clairConfig) // for testing only
		} else {
			clairScanSvc, err = scanners.CreateClairService(clairConfig)
		}
		if err != nil { AbortStartup("When instantiating Clair scan service: " + err.Error()) }
	}
	
	obj = config.ScanServices["twistlock"]
	if obj != nil {
		var twistlockConfig map[string]interface{}
		twistlockConfig, isType = obj.(map[string]interface{})
		if ! isType { AbortStartup("Configuration of twistlock services is ill-formed:") }
		twistlockConfig["LocalIPAddress"] = config.ipaddr
		if stubScanners || useStubScannerFor["twistlock"] != nil {
			twistlockScanSvc, err = scanners.CreateTwistlockServiceStub(twistlockConfig) // for testing only
		} else {
			twistlockScanSvc, err = scanners.CreateTwistlockService(twistlockConfig)
		}
		if err != nil { AbortStartup("When instantiating Twistlock scan service: " + err.Error()) }
	}
	
	obj = config.ScanServices["openscap"]
	if obj != nil {
		var openscapConfig map[string]interface{}
		openscapConfig, isType = obj.(map[string]interface{})
		if ! isType { AbortStartup("Configuration of openscap services is ill-formed:") }
		openscapConfig["LocalIPAddress"] = config.ipaddr
		if stubScanners || useStubScannerFor["openscap"] != nil {
			openscapScanSvc, err = scanners.CreateOpenScapServiceStub(openscapConfig) // for testing only
		} else {
	//		openscapScanSvc, err = scanners.CreateOpenScapService(openscapConfig)
		}
		if err != nil { AbortStartup("When instantiating OpenScap scan service: " + err.Error()) }
	}
	
	server.ScanServices = []scanners.ScanService{
		clairScanSvc,
		twistlockScanSvc,
		openscapScanSvc,
	}
	
	// Install email service.
	obj = config.EmailService
	if obj == nil {
		if server.PerformEmailIdentityVerification {
			AbortStartup("Email service is not configured")
		}
	} else {
		var emailService *utilities.EmailService
		emailService, err = utilities.CreateEmailService(config.EmailService)
		if err != nil { AbortStartup("When instantiating email service: " + err.Error()) }
		server.EmailService = emailService
	}
	
	return server, nil
}

/*******************************************************************************
 * 
 */
func (server *Server) setEmailVerification(enabled bool) {
	server.PerformEmailIdentityVerification = enabled
}

/*******************************************************************************
 * 
 */
func AbortStartup(msg string) {
	fmt.Println("Aborting startup:", msg)
	debug.PrintStack()
	os.Exit(1);
}

/*******************************************************************************
 * 
 */
func (server *Server) Start() {
	// Start listening for incoming HTTP requests.
	// Creates a new service goroutine for each incoming connection on tcpListener.
	// Each service goroutine reads requests and then calls httpServer.Handler
	// to reply to them. See https://golang.org/pkg/net/http/#Server.Serve
	defer server.tcpListener.Close()
	fmt.Println("...Starting service...")
	if err := server.httpServer.Serve(server.tcpListener); err != nil { AbortStartup(err.Error()) }
}

/*******************************************************************************
 * Gracefully stop the server. No work in progress is aborted.
 */
func (server *Server) Stop() {
	fmt.Println("...Stopping service...")
	server.StopAcceptingNewRequests()
	server.WaitUntilNoRequestsInProgress(10)
	fmt.Println("...stop.")
	os.Exit(0)
}

/*******************************************************************************
 * 
 */
func (server *Server) StopAcceptingNewRequests() {
	// Set flag for dispatcher that no more requests should be accepted.
	// ....
}

/*******************************************************************************
 * Blocks until the set of dispatched requests is empty. Waits no longer than
 * the specified number of seconds. Returns false if timeout, true if all
 * requests ended.
 */
func (server *Server) WaitUntilNoRequestsInProgress(maxSeconds int) bool {
	// ....
	return true
}

/*******************************************************************************
 * 
 */
func (server *Server) ResumeAcceptingNewRequests() {
	// ....
}

/*******************************************************************************
 * Warn the administrator that a user has attempted to log in more than
 * MaxLoginAttemptsToRetain times.
 */
func (server *Server) LoginAlert(userId string) {
	fmt.Println("*****Possible brute for attack for user Id " + userId)
}

/*******************************************************************************
 * 
 */
func (server *Server) GetScanServices() []scanners.ScanService {
	return server.ScanServices
}

/*******************************************************************************
 * 
 */
func (server *Server) GetScanService(name string) scanners.ScanService {
	for _, service := range server.ScanServices {
		if service.GetName() == name { return service }
	}
	return nil
}

/*******************************************************************************
 * Build a Certificate data structure by reading the file at the specified path.
 */
func getCert(certPath string) *x509.Certificate {
	
	file, err := os.Open(certPath)
	if err != nil {
		fmt.Println(fmt.Sprintf("Could not open certificate at %s", certPath))
		return nil
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Println(err.Error())
		}
	}()
	var fileInfo os.FileInfo
	fileInfo, err = file.Stat()
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}
	var fileLength = fileInfo.Size()
	var asn1DataBuf = make([]byte, fileLength)
	var n int
	n, err = file.Read(asn1DataBuf)
	if err != nil && err != io.EOF {
		fmt.Println(err.Error())
		return nil
	}
	if int64(n) != fileLength {
		fmt.Println("Number of bytes read for cert does not match file length")
		return nil
	}
	
	// Construct a certificate from the bytes that were read.
	var cert *x509.Certificate
	cert, err = x509.ParseCertificate(asn1DataBuf)
	if err != nil {
		fmt.Println(err.Error())
		return nil
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
	
	fmt.Println("\n\n\n------------incoming request------------")
	defer httpReq.Body.Close() // ensure that request body is always closed.

	if server.Debug { printHeaders(httpReq) }
	
	// Authenitcate session or user.
	var sessionToken *apitypes.SessionToken = nil
	sessionToken = server.authService.authenticateRequestCookie(httpReq)
	if sessionToken == nil { fmt.Println("Server.ServeHTTP: SessionId cookie is nil") }
	
	// Set a header with the API Version for all responses.
	// See https://developer.mozilla.org/en-US/docs/Web/HTTP/Access_control_CORS?redirectlocale=en-US&redirectslug=HTTP_access_control#Access-Control-Allow-Credentials
	writer.Header().Set("SafeHarbor-API-Version", "safeharbor/1.0")
	// http://www.html5rocks.com/en/tutorials/cors/#toc-adding-cors-support-to-the-server
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	writer.Header().Set("Access-Control-Allow-Credentials", "false")
	//writer.Header().Set("Access-Control-Expose-Headers",
	
	server.dispatch(sessionToken, writer, httpReq)
	server.authService.addSessionIdToResponse(sessionToken, writer)

	fmt.Println("---returning from request---\n\n\n")
}

/*******************************************************************************
 * Interpret the request string to determine which method is being requested,
 * and invoke the requested method.
 */
func (server *Server) dispatch(sessionToken *apitypes.SessionToken,
	writer http.ResponseWriter, httpReq *http.Request) {

	fmt.Println("Dispatching request")
	if sessionToken == nil { fmt.Println("Server.dispatch: Session token is nil") }
	
	var err error
	var httpMethod string = strings.ToUpper(httpReq.Method)
	var reqName string = strings.Trim(httpReq.URL.Path, "/ ")
	var headers http.Header = httpReq.Header  // map[string][]string
	var values url.Values
	var files map[string][]*multipart.FileHeader = nil
	
	fmt.Println("URL=" + httpReq.URL.String())  // debug
	fmt.Println("RequestURI=" + httpReq.RequestURI) // debug

	if httpMethod == "GET" {
		
		if err = httpReq.ParseForm(); err != nil { // Query parameters are automatically unencoded.
			apitypes.RespondWithClientError(writer, err.Error())
			return
		}
		values = httpReq.Form  // map[string][]string
		
	} else if httpMethod == "POST" {  // dispatch to an error handler.
	
		// Authorization for a request should be performed using only the intersection
		// of the authority of the user and the requesting origin(s). 
		// Thus, if the request origin is the SafeHarbor Web App origin, we merely
		// need to authorize the user; otherwise, we deny. In the future we should
		// allow users to register trusted origins.
		
		if err = httpReq.ParseForm(); err != nil {  // Query parameters are automatically unencoded.
			apitypes.RespondWithClientError(writer, err.Error())
			return
		}
		values = httpReq.PostForm  // map[string][]string
		
		// Check if the POST is multipart/form-data.
		// https://golang.org/pkg/net/http/#Request.MultipartReader
		// http://www.w3.org/TR/html401/interact/forms.html#h-17.13.4
		var mpReader *multipart.Reader
		mpReader, err = httpReq.MultipartReader()
		if mpReader == nil {
			fmt.Println("Request is not multipart")
		} else { // has multipart data
			// We require all multipart requests to include one (and only one) file part.
			fmt.Println("Request is multipart...")
			
			// https://golang.org/pkg/mime/multipart/#Reader.ReadForm
			var form *multipart.Form
			form, err = mpReader.ReadForm(10000)
			if err != nil {
				apitypes.RespondWithClientError(writer, err.Error())
				return
			}
			if form == nil {
				apitypes.RespondWithClientError(writer, "No form found")
				return
			}
			fmt.Println("Read all multipart form-data parts without error.")
			
			values = form.Value
			files = form.File
			fmt.Println(fmt.Sprintf(
				"Retrieved POST parameters: %d values and %d files.", len(values), len(files)))
		}

	} else if httpMethod == "OPTIONS" {
		// Handle pre-flight request.
		// See https://remysharp.com/2011/04/21/getting-cors-working
		// http://www.w3.org/TR/cors/#preflight-request
		
		//httpReq.Header["Access-Control-Request-Method"]
		var reqHeaders []string = httpReq.Header["Access-Control-Request-Headers"]
		if (reqHeaders == nil) || (len(reqHeaders) != 1) { return }
		
		writer.Header().Set("Access-Control-Allow-Headers", reqHeaders[0])
		return
		
	} else {
		apitypes.RespondMethodNotSupported(writer, httpReq.Method)
		return
	}

	// Enable client to "log" an annotation in the server's stdout, to make it
	// easier to find portions of server output that pertain to a given test.
	if server.Debug && (values != nil) {
		var stringToLog string
		stringToLog, err = apitypes.GetHTTPParameterValue(false, values, "Log")
		if stringToLog != "" {
			fmt.Println("Log:", stringToLog)
		}
	}
	
	fmt.Println("AccountVerificationToken=" + httpReq.FormValue("AccountVerificationToken"))  // debug
	
	fmt.Println("Calling handleRequest")
	server.dispatcher.handleRequest(sessionToken, headers, writer, reqName, values, files)
}

/*******************************************************************************
 * Return the URL of this server.
 */
func (server *Server) GetBasePublicURL() string {
	return server.PublicURL
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
 * Print the HTTP headers to stdout.
 */
func printHeaders(httpReq *http.Request) {
	fmt.Println("HTTP headers:")
	for key, val := range httpReq.Header {
		fmt.Println("\t" + key + ": " + val[0])
	}
}
