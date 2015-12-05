/*******************************************************************************
 * This file contains all declarations related to Server.
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
	//"errors"
	//"strconv"
	
	// My packages:
	//"rest"
	"safeharbor/apitypes"
)

/*******************************************************************************
 * A singleton Server is created by main to service all incoming HTTP requests.
 */
type Server struct {
	Config *Configuration
	httpServer *http.Server
	tcpListener net.Listener
	dbClient DBClient
	http.Handler
	certPool *x509.CertPool
	authService *AuthService
	dispatcher *Dispatcher
	sessions map[string]*apitypes.Credentials  // map session key to Credentials.
	Authorize bool
	Debug bool
}

/*******************************************************************************
 * Create a Server structure. This includes reading in the auth server cert.
 */
func NewServer(debug bool, noauthor bool, port int, adapter string, secretSalt string) *Server {
	
	// Read configuration. (Defined in a JSON file.)
	fmt.Println("Reading configuration")
	var config *Configuration
	var err error
	config, err = getConfiguration()
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}
	
	// Override conf.json with any command line options.
	if port != 0 { config.port = port }
	if adapter != "" { config.netIntfName = adapter }
	
	// Determine the IP address.
	var intfs []net.Interface
	intfs, err = net.Interfaces()
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}
	for _, intf := range intfs {
		fmt.Println("Examining interface " + intf.Name)
		if intf.Name == config.netIntfName {
			var addrs []net.Addr
			addrs, err = intf.Addrs()
			if err != nil {
				fmt.Println(err.Error())
				return nil
			}
			for _, addr := range addrs {
				fmt.Println("\tExamining address " + addr.String())
				config.ipaddr = strings.Split(addr.String(), "/")[0]
				var ip net.IP = net.ParseIP(config.ipaddr)
				if ip.To4() == nil {
					fmt.Println("\t\tskipping")
					continue // skip IP6 addresses
				}
				fmt.Println("Found " + addr.String() + " on network " + addr.Network());
				break
			}
			break
		}
	}
	if config.ipaddr == "" {
		fmt.Println("Did not find an IP4 address for network interface " + config.netIntfName)
		return nil
	}
	
	var certPool *x509.CertPool = getCerts(config)
	
	var dispatcher = NewDispatcher()
	
	// Construct a Server with the configuration and cert pool.
	var server *Server = &Server{
		Debug: debug,
		Authorize: (! noauthor),
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
		config.AuthServerName, config.AuthPort, certPool, secretSalt)
	
	// To do: Make this a TLS listener.
	// Instantiate an HTTP server with the SafeHarbor server as the handler.
	// See https://golang.org/pkg/net/http/#Server
	server.httpServer = &http.Server{
		Handler: server.getHttpHandler(),
	}

	// Instantiate a TCP socker listener.
	fmt.Println("...Creating socket listener at", config.ipaddr, "port", config.port, "...")
	server.tcpListener, err = newTCPListener(config.ipaddr, config.port)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1);
	}
	
	// Verify that the docker service is running, and start it if not.
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
func (server *Server) Start() {
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
	
	defer httpReq.Body.Close() // ensure that request body is always closed.

	if server.Debug { printHeaders(httpReq) }
	
	// Authenitcate session or user.
	var sessionToken *apitypes.SessionToken = nil
	sessionToken = server.authService.authenticateRequest(httpReq)
	if sessionToken == nil { fmt.Println("Server.ServeHTTP: Session token is nil") }
	
	//if sessionToken == nil { //return authent failure
	//	fmt.Println("Failed to authenticate - request being denied")
	//	apitypes.RespondWithClientError(writer, "Failed to authenticate - request being denied")
	//	return
	//}
	//fmt.Println("authenticated request")
	
	// Set a header with the API Version for all responses.
	// See https://developer.mozilla.org/en-US/docs/Web/HTTP/Access_control_CORS?redirectlocale=en-US&redirectslug=HTTP_access_control#Access-Control-Allow-Credentials
	writer.Header().Set("SafeHarbor-API-Version", "safeharbor/1.0")
	// http://www.html5rocks.com/en/tutorials/cors/#toc-adding-cors-support-to-the-server
	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	writer.Header().Set("Access-Control-Allow-Credentials", "false")
	//writer.Header().Set("Access-Control-Expose-Headers",
	
	server.dispatch(sessionToken, writer, httpReq)
	server.authService.addSessionIdToResponse(sessionToken, writer)
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
	
	if httpMethod == "GET" {
		
		if err = httpReq.ParseForm(); err != nil {
			apitypes.RespondWithClientError(writer, err.Error())
			return
		}
		values = httpReq.Form  // map[string]string
		
	} else if httpMethod == "POST" {  // dispatch to an error handler.
	
		// Authorization for a request should be performed using only the intersection
		// of the authority of the user and the requesting origin(s). 
		// Thus, if the request origin is the SafeHarbor Web App origin, we merely
		// need to authorize the user; otherwise, we deny. In the future we should
		// allow users to register trusted origins.
		
		if err = httpReq.ParseForm(); err != nil {
			apitypes.RespondWithClientError(writer, err.Error())
			return
		}
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
			if form == nil {
				apitypes.RespondWithClientError(writer, "No form found")
				return
			}
			fmt.Println("Read form data")
			if err != nil {
				apitypes.RespondWithClientError(writer, err.Error())
				return
			}
			
			values = form.Value
			files = form.File
			fmt.Println("Set file parameters")
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
 * Print the HTTP headers to stdout.
 */
func printHeaders(httpReq *http.Request) {
	fmt.Println("HTTP headers:")
	for key, val := range httpReq.Header {
		fmt.Println("\t" + key + ": " + val[0])
	}
}
