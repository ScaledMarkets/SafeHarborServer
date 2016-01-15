package rest

import (
	"net/http"
	"mime/multipart"
	"fmt"
	"net/url"
	"io"
	"os"
	"strings"
	"bytes"
	"encoding/json"
)

type RestContext struct {
	httpClient *http.Client
	hostname string
	port string
	setSessionId func(request *http.Request, id string)
}

/*******************************************************************************
 * 
 */
func CreateRestContext(hostname string, port string, sessionIdSetter func(*http.Request, string)) *RestContext {
	return &RestContext{
		httpClient: &http.Client{
			Transport: &http.Transport{
				DisableCompression: true,
			},
		},
		hostname: hostname,
		port: port,
		setSessionId: sessionIdSetter,
	}
}

/*******************************************************************************
 * Send a GET request to the SafeHarborServer, at the specified REST endpoint method
 * (reqName), with the specified query parameters.
 */
func (restContext *RestContext) SendGet(sessionId string, reqName string, names []string,
	values []string) (*http.Response, error) {

	return restContext.sendReq(sessionId, "GET", reqName, names, values)
}

/*******************************************************************************
 * Send an HTTP POST formatted according to what is required by the SafeHarborServer
 * REST API, as defined in the slides "SafeHarbor REST API" of the design,
 * https://drive.google.com/open?id=1r6Xnfg-XwKvmF4YppEZBcxzLbuqXGAA2YCIiPb_9Wfo
 */
func (restContext *RestContext) SendPost(sessionId string, reqName string, names []string,
	values []string) (*http.Response, error) {

	return restContext.sendReq(sessionId, "POST", reqName, names, values)
}

/*******************************************************************************
 * Send an HTTP POST formatted according to what is required by the SafeHarborServer
 * REST API, as defined in the slides "SafeHarbor REST API" of the design,
 * https://drive.google.com/open?id=1r6Xnfg-XwKvmF4YppEZBcxzLbuqXGAA2YCIiPb_9Wfo
 */
func (restContext *RestContext) sendReq(sessionId string, reqMethod string,
	reqName string, names []string, values []string) (*http.Response, error) {

	// Send REST POST request to server.
	var urlstr string = fmt.Sprintf(
		"http://%s:%s/%s",
		restContext.hostname, restContext.port, reqName)
	
	var data url.Values = url.Values{}
	for index, each := range names {
		data[each] = []string{values[index]}
	}
	var reader io.Reader = strings.NewReader(data.Encode())
	var request *http.Request
	var err error
	request, err = http.NewRequest(reqMethod, urlstr, reader)
	if err != nil { return nil, err }
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if sessionId != "" { restContext.setSessionId(request, sessionId) }
	//if sessionId != "" { request.Header.Set("Session-Id", sessionId) }
	
	var resp *http.Response
	resp, err = restContext.httpClient.Do(request)
	if err != nil { return nil, err }
	return resp, nil
}


/*******************************************************************************
 * Send request as a multi-part so that a file can be attached.
 */
func (restContext *RestContext) SendFilePost(sessionId string, reqName string, names []string,
	values []string, path string) (*http.Response, error) {

	var urlstr string = fmt.Sprintf(
		"http://%s:%s/%s",
		restContext.hostname, restContext.port, reqName)

	// Prepare a form that you will submit to that URL.
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	
	// Add file
	f, err := os.Open(path)
	if err != nil { return nil, err }
	var fileInfo os.FileInfo
	fileInfo, err = f.Stat()
	if err != nil { return nil, err }
	fw, err := w.CreateFormFile("filename", fileInfo.Name())
	if err != nil { return nil, err }
	_, err = io.Copy(fw, f)
	if err != nil { return nil, err }
	
	// Add the other fields
	if names != nil {
		for index, each := range names {
			fw, err = w.CreateFormField(each)
			if err != nil { return nil, err }
			_, err = fw.Write([]byte(values[index]))
			if err != nil { return nil, err }
		}
	}
	
	// Don't forget to close the multipart writer.
	// If you don't close it, your request will be missing the terminating boundary.
	w.Close()

	// Now that you have a form, you can submit it to your handler.
	req, err := http.NewRequest("POST", urlstr, &b)
	if err != nil { return nil, err }
	
	// Don't forget to set the content type, this will contain the boundary.
	req.Header.Set("Content-Type", w.FormDataContentType())
	if sessionId != "" { restContext.setSessionId(req, sessionId) }
	//if sessionId != "" { req.Header.Set("Session-Id", sessionId) }

	// Submit the request
	res, err := restContext.httpClient.Do(req)
	if err != nil { return nil, err }

	return res, nil
}

/*******************************************************************************
 * Parse an HTTP JSON response that can be converted to a map.
 */
func ParseResponseBodyToMap(body io.ReadCloser) (map[string]interface{}, error) {
	var value []byte = ReadResponseBody(body)
	var obj map[string]interface{}
	err := json.Unmarshal(value, &obj)
	//var dec *json.Decoder = json.NewDecoder(body)
	//err := dec.Decode(&obj)
	if err != nil { return nil, err }
	//AssertErrIsNil(err, "When unmarshalling obj")
	
	
	//var result map[string]interface{}
	//var isType bool
	
	//result, _ = obj.(map[string]interface{})
	//assertThat(isType, "Wrong type: obj is not a map[string]interface{}")
	return obj, nil
}

/*******************************************************************************
 * Parse an HTTP JSON response that can be converted to an array of maps.
 */
func ParseResponseBodyToMaps(body io.ReadCloser) ([]map[string]interface{}, error) {
	var value []byte = ReadResponseBody(body)
	var obj []map[string]interface{}
	err := json.Unmarshal(value, &obj)
	if err != nil { return nil, err }
	//var isType bool
	//result, isType = obj.([]map[string]interface{})
	//if ! isType {
	//	fmt.Println("This is what was returned:")
	//	fmt.Println(result)
	//}
	//assertThat(isType, "Wrong type: obj is not a []map[string]interface{} - it is a " + 
	//	fmt.Sprintf("%s", reflect.TypeOf(obj)))
	return obj, nil
}

/*******************************************************************************
 * Parse an arbitrary HTTP JSON response.
 */
func ReadResponseBody(body io.ReadCloser) []byte {
	
	var value []byte = make([]byte, 0)
	//var s string = ""
	for {
		var buf []byte = make([]byte, 100)
		n, err := body.Read(buf)
		if n > 0 { value = append(value, buf[0:n]...) }
		//fmt.Println("Read", string(buf))
		if err != nil { break }
		if n < len(buf) { break }
	}
	fmt.Println("rest.ReadResponseBody this:")
	fmt.Println(string(value))
	fmt.Println("--")
	fmt.Println()
	
	return value
}

/*******************************************************************************
 * Write the specified map to stdout.
 */
func PrintMap(m map[string]interface{}) {
	fmt.Println("Map:")
	for k, v := range m {
		fmt.Println(k, v)
	}
}

/*******************************************************************************
 * If the response is not 200, then throw an exception.
 */
func (restContext *RestContext) Verify200Response(resp *http.Response) bool {
	var is200 bool = true
	if resp.StatusCode != 200 {
		is200 = false
		fmt.Sprintf("Response code %d", resp.StatusCode)
		var responseMap map[string]interface{}
		var err error
		responseMap, err = ParseResponseBodyToMap(resp.Body)
		if err == nil { PrintMap(responseMap) }
		//if restContext.stopOnFirstError { os.Exit(1) }
	}
	fmt.Println("Response code ", resp.StatusCode)
	return is200
}

/*******************************************************************************
 * Utility to encode an arbitrary string value, which might contain quotes and other
 * characters, so that it can be safely and securely transported as a JSON string value,
 * delimited by double quotes. Ref. http://json.org/.
 * To do: ....Deal with unicode sequences.
 */
func EncodeStringForJSON(value string) string {
	// Replace each occurrence of double-quote and backslash with backslash double-quote
	// or backslash backslash, respectively.
	
	var encodedValue = value
	encodedValue = strings.Replace(encodedValue, "\"", "\\\"", -1)
	encodedValue = strings.Replace(encodedValue, "\\", "\\\\", -1)
	encodedValue = strings.Replace(encodedValue, "/", "\\/", -1)
	encodedValue = strings.Replace(encodedValue, "\b", "\\b", -1)
	encodedValue = strings.Replace(encodedValue, "\f", "\\f", -1)
	encodedValue = strings.Replace(encodedValue, "\n", "\\n", -1)
	encodedValue = strings.Replace(encodedValue, "\r", "\\r", -1)
	encodedValue = strings.Replace(encodedValue, "\t", "\\t", -1)
	return encodedValue
}

/*******************************************************************************
 * Reverse the encoding that is performed by EncodeStringForJSON.
 */
func DecodeStringFromJSON(encodedValue string) string {
	var decodedValue = encodedValue
	decodedValue = strings.Replace(decodedValue, "\\t", "\t", -1)
	decodedValue = strings.Replace(decodedValue, "\\r", "\r", -1)
	decodedValue = strings.Replace(decodedValue, "\\n", "\n", -1)
	decodedValue = strings.Replace(decodedValue, "\\f", "\f", -1)
	decodedValue = strings.Replace(decodedValue, "\\b", "\b", -1)
	decodedValue = strings.Replace(decodedValue, "\\/", "/", -1)
	decodedValue = strings.Replace(decodedValue, "\\\\", "\\", -1)
	decodedValue = strings.Replace(decodedValue, "\\\"", "\"", -1)
	return decodedValue
}
