package docker

/* Interface for interacting with a Docker Registry version 2.

	What a docker "name" is:

		(From: https://github.com/docker/distribution/blob/master/docs/spec/api.md)
		
		All endpoints will be prefixed by the API version and the repository name:
		
		/v2/<name>/
		
		For example, an API endpoint that will work with the library/ubuntu repository,
		the URI prefix will be:
		
		/v2/library/ubuntu/
		
		This scheme provides rich access control over various operations and methods
		using the URI prefix and http methods that can be controlled in variety of ways.
		
		Classically, repository names have always been two path components where each
		path component is less than 30 characters. The V2 registry API does not enforce this.
		The rules for a repository name are as follows:
		
			1. A repository name is broken up into path components. A component of a
				repository name must be at least one lowercase, alpha-numeric characters,
				optionally separated by periods, dashes or underscores. More strictly,
				it must match the regular expression [a-z0-9]+(?:[._-][a-z0-9]+)*.
				
			2. If a repository name has two or more path components, they must be
				separated by a forward slash ("/").
				
			3. The total length of a repository name, including slashes, must be
				less the 256 characters.
		
		These name requirements only apply to the registry API and should accept
		a superset of what is supported by other docker ecosystem components.
*/



import (
	"fmt"
	"io"
	"os"
	"io/ioutil"
	"net/http"
	"archive/tar"
	"encoding/json"
	"reflect"
	"strings"
	
	"safeharbor/utils"
	"safeharbor/rest"
)

type DockerRegistryImpl struct {
	rest.RestContext
}

/*******************************************************************************
 * 
 */
func OpenDockerRegistryConnection(host string, port int, userId string,
	password string) (DockerRegistry, error) {
	
	fmt.Println(fmt.Sprintf("Opening connection to registry %s:%s@%s:%d",
		userId, password, host, port))
	
	var registry *DockerRegistryImpl = &DockerRegistryImpl{
		RestContext: *rest.CreateTCPRestContext("http", host, port, userId, password, noop),
	}
	
	fmt.Println("Pinging registry...")
	
	var err error = registry.Ping()
	if err != nil {
		return nil, err
	}
	
	fmt.Println("...received response.")
	
	return registry, nil
}

/*******************************************************************************
 * 
 */
func (registry *DockerRegistryImpl) Close() {
}

/*******************************************************************************
 * 
 */
func (registry *DockerRegistryImpl) Ping() error {
	
	var uri = "v2/"
	
	var response *http.Response
	var err error
	response, err = registry.SendBasicGet(uri)
	if err != nil { return err }
	if response.StatusCode != 200 {
		return utils.ConstructServerError(fmt.Sprintf("Ping returned status: %s", response.Status))
	}
	return nil
}

/*******************************************************************************
 * If the specified image exists, return true. The repo name is the image path
 * of the image namespace - if any - and registry repository name, separated by a "/".
 */
func (registry *DockerRegistryImpl) ImageExists(repoName string, tag string) (bool, error) {
	
	// https://github.com/docker/distribution/blob/master/docs/spec/api.md
	// https://docs.docker.com/apidocs/v1.4.0/#!/repositories/GetRepository
	var uri = "v2/" + repoName + "/manifests/" + tag
	//v0: GET /api/v0/repositories/{namespace}/{reponame}
	// Make HEAD request to registry.
	var response *http.Response
	var err error
	response, err = registry.SendBasicHead(uri)
	if err != nil { return false, err }
	if response.StatusCode == 200 {
		return true, nil
	} else if response.StatusCode == 404 { // Not Found
		return false, nil
	} else {
		return false, utils.ConstructServerError(fmt.Sprintf("ImageExists returned status: %s", response.Status))
	}
}

/*******************************************************************************
 * 
 */
func (registry *DockerRegistryImpl) LayerExistsInRepo(repoName, digest string) (bool, error) {
	
	var uri = fmt.Sprintf("v2/%s/blobs/%s", repoName, digest)
	var response *http.Response
	var err error
	response, err = registry.SendBasicHead(uri)
	if err != nil { return false, err }
	if response.StatusCode == 200 {
		return true, nil
	} else if response.StatusCode == 404 { // Not Found
		return false, nil
	} else {
		return false, utils.ConstructServerError(fmt.Sprintf("ImageExists returned status: %s", response.Status))
	}
}

/*******************************************************************************
 * If the specified image exists, return true. The repo name is the image path
 * of the image namespace - if any - and registry repository name, separated by a "/".
 */
func (registry *DockerRegistryImpl) GetImageInfo(repoName string, tag string) (digest string,
	layerAr []map[string]interface{}, err error) {
	
	// Retrieve manifest.
	var uri = "v2/" + repoName + "/manifests/" + tag
	var resp *http.Response
	resp, err = registry.SendBasicGet(uri)
	if err != nil { return "", nil, err }
	if resp.StatusCode == 404 {
		return "", nil, utils.ConstructUserError("Not found")
	} else if resp.StatusCode != 200 {
		return "", nil, utils.ConstructServerError(fmt.Sprintf("ImageExists returned status: %s", resp.Status))
	}
	
	// Parse description of each layer.
	layerAr, err = parseManifest(resp.Body)
	resp.Body.Close()
	if err != nil { return "", nil, err }
	
	// Retrieve image digest header.
	var headers map[string][]string = resp.Header
	digest = headers["Docker-Content-Digest"][0]
	
	return digest, layerAr, nil
}

/*******************************************************************************
 * 
 */
func (registry *DockerRegistryImpl) GetImage(repoName string, tag string, filepath string) error {
	
	// GET /v2/<name>/manifests/<reference>
	// GET /v2/<name>/blobs/<digest>
	
	// Retrieve manifest.
	var uri = "v2/" + repoName + "/manifests/" + tag
	var resp *http.Response
	var err error
	resp, err = registry.SendBasicGet(uri)
	if err != nil { return err }
	if resp.StatusCode == 404 {
		return utils.ConstructUserError("Not found")
	} else if resp.StatusCode != 200 {
		return utils.ConstructServerError(fmt.Sprintf("ImageExists returned status: %s", resp.Status))
	}
	
	// Parse description of each layer.
	var layerAr []map[string]interface{}
	layerAr, err = parseManifest(resp.Body)
	resp.Body.Close()
	if err != nil { return err }
	
	// Retrieve layers, and add each to a tar archive.
	var tarFile *os.File
	tarFile, err = os.Create(filepath)
	if err != nil { return utils.ConstructServerError(fmt.Sprintf(
		"When creating image file '%s': %s", filepath, err.Error()))
	}
	var tarWriter = tar.NewWriter(tarFile)
	var tempDirPath string
	tempDirPath, err = utils.MakeTempDir()
	if err != nil { return utils.ConstructServerError(fmt.Sprintf(
		"When creating temp directory for writing layer files: %s", err.Error()))
	}
	defer os.RemoveAll(tempDirPath)
	for _, layerDesc := range layerAr {
		
		var layerDigest = layerDesc["blobSum"]
		if layerDigest == nil {
			return utils.ConstructServerError("Did not find blobSum field in response for layer")
		}
		var digest string
		var isType bool
		digest, isType = layerDigest.(string)
		if ! isType { return utils.ConstructServerError("blogSum field is not a string - it is a " +
			reflect.TypeOf(layerDigest).String())
		}
		uri = "v2/" + repoName + "/blobs/" + digest
		resp, err = registry.SendBasicGet(uri)
		if err != nil { return err }
		defer resp.Body.Close()
		if resp.StatusCode >= 500 {
			return utils.ConstructServerError(fmt.Sprintf(
				"Response code %d, when requesting uri: '%s'", resp.StatusCode, uri))
		} else if resp.StatusCode >= 400 {
			return utils.ConstructUserError(fmt.Sprintf(
				"Response code %d, when requesting uri: '%s'", resp.StatusCode, uri))
		} else  if resp.StatusCode >= 300 {
			return utils.ConstructServerError(fmt.Sprintf(
				"Response code %d, when requesting uri: '%s'", resp.StatusCode, uri))
		}

		// Create temporary file in which to write layer.
		var layerFile *os.File
		layerFile, err = utils.MakeTempFile(tempDirPath, digest)
		if err != nil { return utils.ConstructServerError(fmt.Sprintf(
			"When creating layer file: %s", err.Error()))
		}
		
		var reader io.ReadCloser = resp.Body
		layerFile, err = os.OpenFile(layerFile.Name(), os.O_WRONLY, 0600)
		if err != nil { return utils.ConstructServerError(fmt.Sprintf(
			"When opening layer file '%s': %s", layerFile.Name(), err.Error()))
		}
		_, err = io.Copy(layerFile, reader)
		if err != nil { return utils.ConstructServerError(fmt.Sprintf(
			"When writing layer file '%s': %s", layerFile.Name(), err.Error()))
		}
		var fileInfo os.FileInfo
		fileInfo, err = layerFile.Stat()
		if err != nil { return utils.ConstructServerError(fmt.Sprintf(
			"When getting status of layer file '%s': %s", layerFile.Name(), err.Error()))
		}
		if fileInfo.Size() == 0 { return utils.ConstructServerError(fmt.Sprintf(
			"Layer file that was written, '%s', has zero size", layerFile.Name()))
		}
		
		// Add file to tar archive.
		var tarHeader = &tar.Header{
			Name: fileInfo.Name(),
			Mode: 0600,
			Size: int64(fileInfo.Size()),
		}
		err = tarWriter.WriteHeader(tarHeader)
		if err != nil {	return utils.ConstructServerError(fmt.Sprintf(
			"While writing layer header to tar archive: , %s", err.Error()))
		}
		
		layerFile, err = os.Open(layerFile.Name())
		if err != nil {	return utils.ConstructServerError(fmt.Sprintf(
			"While opening layer file '%s': , %s", layerFile.Name(), err.Error()))
		}
		_, err := io.Copy(tarWriter, layerFile)
		if err != nil {	return utils.ConstructServerError(fmt.Sprintf(
			"While writing layer content to tar archive: , %s", err.Error()))
		}
	}
	
	err = tarWriter.Close()
	if err != nil {	return utils.ConstructServerError(fmt.Sprintf(
		"While closing tar archive: , %s", err.Error()))
	}
	
	return nil
}

/*******************************************************************************
 * 
 */
func (registry *DockerRegistryImpl) DeleteImage(repoName, tag string) error {
	
	//v2: DELETE /v2/<name>/blobs/<digest>
	//	DELETE /v2/<name>/manifests/<reference>
	//v1: DELETE /api/v0/repositories/{namespace}/{reponame}
	
	// Retrieve manifest.
	var uri = "v2/" + repoName + "/manifests/" + tag
	var resp *http.Response
	var err error
	resp, err = registry.SendBasicGet(uri)
	if err != nil { return err }
	resp.Body.Close()
	if resp.StatusCode == 404 {
		return utils.ConstructUserError("Not found")
	} else if resp.StatusCode != 200 {
		return utils.ConstructServerError(fmt.Sprintf("DeleteImage returned status: %s", resp.Status))
	}
	
	// Parse description of each layer.
	var layerAr []map[string]interface{}
	layerAr, err = parseManifest(resp.Body)
	if err != nil { return err }
	
	// Delete each layer.
	for _, layerDesc := range layerAr {
		
		var layerDigest = layerDesc["blobSum"]
		if layerDigest == nil {
			return utils.ConstructServerError("Did not find blobSum field in response for layer")
		}
		var digest string
		var isType bool
		digest, isType = layerDigest.(string)
		if ! isType { return utils.ConstructServerError("blogSum field is not a string - it is a " +
			reflect.TypeOf(layerDigest).String())
		}
		
		uri = fmt.Sprintf("v2/%s/blobs/%s", repoName, digest)
		var response *http.Response
		var err error
		response, err = registry.SendBasicDelete(uri)
		if err != nil { return err }
		if response.StatusCode == 200 {
			return nil
		} else if response.StatusCode == 404 { // Not Found
			return utils.ConstructUserError(fmt.Sprintf("DeleteImage - image not found: %s", response.Status))
		} else {
			return utils.ConstructServerError(fmt.Sprintf("DeleteImage returned status: %s", response.Status))
		}
	}
	
	// Delete manifest.
	uri = "v2/" + repoName + "/manifests/" + tag
	resp, err = registry.SendBasicDelete(uri)
	if err != nil { return err }
	
	return nil
}

/*******************************************************************************
 * 
 */
func (registry *DockerRegistryImpl) PushImage(repoName, tag, imageFilePath string) error {
	
	// Create a scratch directory.
	var tempDirPath string
	var err error
	tempDirPath, err = utils.MakeTempDir()
	if err != nil { return err }
	//defer os.RemoveAll(tempDirPath)
	
	// Expand tar file.
	var tarFile *os.File
	tarFile, err = os.Open(imageFilePath)
	if err != nil { return err }
	var tarReader *tar.Reader = tar.NewReader(tarFile)
	
	for { // each tar file entry
		var header *tar.Header
		header, err = tarReader.Next()
		if err == io.EOF { break }
		if err != nil { return err }
		
		if strings.HasSuffix(header.Name, "/") {  // a directory
			
			var dirname = tempDirPath + "/" + header.Name
			err = os.Mkdir(dirname, 0770)
			if err != nil { return err }
			
		} else if (header.Name == "repositories") ||
				strings.HasSuffix(header.Name, "/layer.tar") {
			
			// Write entry to a file.
			var nWritten int64
			var outfile *os.File
			var filename = tempDirPath + "/" + header.Name
			outfile, err = os.OpenFile(filename, os.O_CREATE | os.O_RDWR, 0770)
			if err != nil { return err }
			nWritten, err = io.Copy(outfile, tarReader)
			if err != nil { return err }
			if nWritten == 0 { return utils.ConstructServerError(
				"No data written to " + filename)
			}
			outfile.Close()
		}
	}
	
	// Parse the 'repositories' file.
	// We are expecting a format as,
	//	{"<repo-name>":{"<tag>":"<digest>"}}
	// E.g.,
	//	{"realm4/repo1":{"myimage2":"d2cf21381ce5a17243ec11062b5..."}}
	var repositoriesFile *os.File
	repositoriesFile, err = os.Open(tempDirPath + "/" + "repositories")
	if err != nil { return err }
	var bytes []byte
	bytes, err = ioutil.ReadAll(repositoriesFile)
	if err != nil { return err }
	var obj interface{}
	err = json.Unmarshal(bytes, &obj)
	if err != nil { return err }
	var repositoriesMap map[string]interface{}
	var isType bool
	repositoriesMap, isType = obj.(map[string]interface{})
	if ! isType { return utils.ConstructServerError(
		"repositories file json does not translate to a map[string]interface")
	}
	if len(repositoriesMap) == 0 { return utils.ConstructServerError(
		"No entries found in repository map for image")
	}
	if len(repositoriesMap) > 1 { return utils.ConstructServerError(
		"More than one entry found in repository map for image")
	}
	
	//var oldRepoName string
	//var oldTag string
	var imageDigest string
	for _, tagObj := range repositoriesMap {
		//oldRepoName = rName
		var tagMap map[string]interface{}
		tagMap, isType = tagObj.(map[string]interface{})
		if ! isType { return utils.ConstructServerError(
			"repository json does not translate to a map[string]interface")
		}
		if len(tagMap) == 0 { return utils.ConstructServerError(
			"No entries found in tag map for repo")
		}
		if len(tagMap) > 1 { return utils.ConstructServerError(
			"More than one entry found in tag map for repo")
		}
		for _, tagDigestObj := range tagMap {
			//oldTag = t
			var tagDigest string
			tagDigest, isType = tagDigestObj.(string)
			if ! isType { return utils.ConstructServerError(
				"Digest is not a string")
			}
			imageDigest = tagDigest
		}
	}
	
	// Obtain digest strings and layer paths.
	var scratchDir *os.File
	scratchDir, err = os.Open(tempDirPath)
	if err != nil { return err }
	var layerFilenames []string
	layerFilenames, err = scratchDir.Readdirnames(0)
	if err != nil { return err }

	// Send each layer to the registry.
	for _, layerDigest := range layerFilenames {  // layer files are named by their digest
		var exists bool
		exists, err = registry.LayerExistsInRepo(repoName, layerDigest)
		if err != nil { return err }
		if exists { continue }
		
		var layerFilePath = tempDirPath + "/" + layerDigest + "/layer.tar"
		err = registry.PushLayer(layerFilePath, repoName, layerDigest)
		if err != nil { return err }
	}
	
	// Send a manifest to the registry.
	err = registry.PushManifest(repoName, tag, imageDigest, layerFilenames)
	if err != nil { return err }
	
	os.RemoveAll(tempDirPath)

	return nil
}

/*******************************************************************************
 * Push a layer, using the "chunked" upload registry protocol.
 */
func (registry *DockerRegistryImpl) PushLayer(layerFilePath, repoName, digestString string) error {

	var uri = fmt.Sprintf("v2/%s/blobs/uploads/", repoName)
	
	var response *http.Response
	var err error
	response, err = registry.SendBasicFormPost(uri, []string{}, []string{})
	if err != nil { return err }
	if response.StatusCode != 202 {
		return utils.ConstructServerError(fmt.Sprintf(
			"Posting to start upload of layer returned status: %s", response.Status))
	}
	
	// Get Location header.
	var locations []string = response.Header["Location"]
	if locations == nil { return utils.ConstructServerError("No Location header") }
	if len(locations) != 1 { return utils.ConstructServerError("Unexpected Location header") }
	var location string = locations[0]
	
	var layerFile *os.File
	layerFile, err = os.Open(layerFilePath)
	if err != nil { return err }
	var fileInfo os.FileInfo
	fileInfo, err = layerFile.Stat()
	if err != nil { return err }
	
	var fileSize int64 = fileInfo.Size()
	var headers = map[string]string{
		"Content-Length": fmt.Sprintf("%d", fileSize),
		"Content-Type": "application/octet-stream",
	}
	
	uri = fmt.Sprintf("v2/%s/blobs/uploads/%s?digest=%s", repoName, location, digestString)
	response, err = registry.SendBasicStreamPut(uri, headers, layerFile)
	if err != nil { return err }
	if response.StatusCode != 201 {
		return utils.ConstructServerError(fmt.Sprintf(
			"Posting layer returned status: %s", response.Status))
	}
	
	return nil
}

/*******************************************************************************
 * Push a layer, using a single POST.
 * (The Registry does not yet support this.)
 */
func (registry *DockerRegistryImpl) PushLayerSinglePost(layerFilePath, repoName, digestString string) error {
	
	var layerFile *os.File
	var err error
	layerFile, err = os.Open(layerFilePath)
	if err != nil { return err }
	var fileInfo os.FileInfo
	fileInfo, err = layerFile.Stat()
	if err != nil { return err }
	
	var fileSize int64 = fileInfo.Size()
	var response *http.Response
	var headers = map[string]string{
		"Content-Length": fmt.Sprintf("%d", fileSize),
		"Content-Type": "application/octet-stream",
	}
	
	var uri = fmt.Sprintf("v2/%s/blobs/uploads/?digest=%s", repoName, digestString)
	
	response, err = registry.SendBasicStreamPost(uri, headers, layerFile)
	if err != nil { return err }
	if response.StatusCode != 202 {
		return utils.ConstructServerError(fmt.Sprintf("Posting layer returned status: %s", response.Status))
	}
	
	return nil
}

/*******************************************************************************
 * 
 */
func (registry *DockerRegistryImpl) PushManifest(repoName, tag, imageDigestString string,
	layerDigestStrings []string) error {
	
	var uri = fmt.Sprintf("v2/%s/manifests/%s", repoName + ":" + tag, imageDigestString)
	
	var manifest = fmt.Sprintf("{" +
		"\"name\": \"%s\", \"tag\": \"%s\", \"fsLayers\": [", repoName, tag)
	
	for i, layerDigestString := range layerDigestStrings {
		if i > 0 { manifest = manifest + ",\n" }
		manifest = manifest + fmt.Sprintf("{\"blobSum\": \"%s\"}", layerDigestString)
	}
	
	manifest = manifest + "]}"
	
	var stringReader *strings.Reader = strings.NewReader(manifest)
	
	var headers = map[string]string{
		"Content-Length": fmt.Sprintf("%d", len(manifest)),
		"Content-Type": "application/json",
	}
	
	var response *http.Response
	var err error
	response, err = registry.SendBasicStreamPut(uri, headers, stringReader)
	if err != nil { return err }
	if response.StatusCode != 201 {
		return utils.ConstructServerError(fmt.Sprintf("Putting manifest returned status: %s", response.Status))
	}
	
	return nil
}

/*******************************************************************************
 * Return an array of maps, one for each layer, and each containing the attributes
 * of the layer.
 */
func parseManifest(body io.ReadCloser) ([]map[string]interface{}, error) {
	
	var responseMap map[string]interface{}
	var err error
	responseMap, err = rest.ParseResponseBodyToMap(body)
	if err != nil { return nil, err }
	body.Close()
	var layersObj = responseMap["fsLayers"]
	if layersObj == nil {
		return nil, utils.ConstructServerError("Did not find fsLayers field in body")
	}
	var isType bool
	var layerArObj []interface{}
	layerArObj, isType = layersObj.([]interface{})
	if ! isType { return nil, utils.ConstructServerError(
		"Type of layer description is " + reflect.TypeOf(layersObj).String())
	}
	var layerAr = make([]map[string]interface{}, 0)
	for _, obj := range layerArObj {
		var m map[string]interface{}
		m, isType = obj.(map[string]interface{})
		if ! isType { return nil, utils.ConstructServerError(
			"Type of layer object is " + reflect.TypeOf(obj).String())
		}
		layerAr = append(layerAr, m)
	}
	
	return layerAr, nil
}

/*******************************************************************************
 * 
 */
func noop(req *http.Request, s string) {
}
