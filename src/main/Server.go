package main

import (
	"fmt"
	"net/http"
	"io"
	"os"
	"io/ioutil"
	"crypto/tls"
	"crypto/x509"
	"errors"
)

/*
This file contains all declarations related to Server.
*/


/*******************************************************************************
 * 
 */
type Server struct {
	Config Configuration
	http.Handler
	//AuthSvc AuthorizationService
	certPool *x509.CertPool
}

/*******************************************************************************
 * Create a Server structure. This includes reading in the auth server cert.
 */
func NewServer(config Configuration) *Server {
	
	// Read certificate files and add certs to cert pool.
	var certPool *x509.CertPool = x509.NewCertPool()
	var rootCert *x509.Certificate = nil
	if config.LocalRootCertPath != "" {
		rootCert = getCert(config.LocalRootCertPath)
		certPool.AddCert(rootCert)
	}
	var authCert *x509.Certificate = getCert(config.LocalAuthCertPath)
	certPool.AddCert(authCert)
	
	// Construct a Server with the configuration and cert pool.
	var server *Server = &Server{
		Config:  config,
		certPool: certPool,
	}
	return server
}

/*******************************************************************************
 * 
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
 * 
 */
func (server *Server) getHandler() http.Handler {
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
 * 
 */
func (server *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close() // ensure that request body is always closed.

	// Set a header with the Docker Distribution API Version for all responses.
	w.Header().Add("Docker-Distribution-API-Version", "registry/2.0")
	
	
	// Connect to auth server.
	if ! server.authorized("registry.docker.com", "repository:samalba/my-app:push", "jlhawn") {
		fmt.Println("Unauthorized: %s, %s, %s")
	}
	
	server.dispatch(w, r)
}

/*******************************************************************************
 * Interpret the request string to determine which method is being requested,
 * and invoke the requested method.
 */
func (server *Server) dispatch(w http.ResponseWriter, r *http.Request) {
	//....
}

/*******************************************************************************
 * https://stackoverflow.com/questions/24496344/golang-send-http-request-with-certificate
 */
func (server *Server) authorized(service string, scope string, account string) bool {
	
	var tr *http.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: server.certPool},
		DisableCompression: true,
	}
	var client *http.Client = &http.Client{Transport: tr}
	
	// Access auth server and get response.
	//.....Not sure this is right. See here:
	// https://www.socketloop.com/references/golang-crypto-tls-server-and-connectionstate-functions-example
	var err error;
	var resp *http.Response
	var url string = fmt.Sprintf("https://%s:%s/v2/token/?service=%s&scope=%s&account=%s",
			server.Config.AuthServerName, server.Config.AuthPort,
			service, scope, account)
	resp, err = client.Get(url)
	if err != nil {
		fmt.Println("Error")
		panic(err)
		return false
	}
	
	defer resp.Body.Close()
	
	// Parse the response body.
	fmt.Println("Parsing response body")
	var body []byte
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err.Error())
		return false
	}
	fmt.Println(body)
	
	return true
}
