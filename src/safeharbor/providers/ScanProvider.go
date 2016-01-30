package providers

import (
	
	"safeharbor/apitypes"
)

type ScanService interface {
	GetName() string
	GetEndpoint() string
	GetParameterDescriptions() map[string]string
	GetParameterDescription(string) (string, error)
	CreateScanContext(map[string]string) (ScanContext, error)  // params may be nil
	AsScanProviderDesc() *apitypes.ScanProviderDesc
}

type ScanContext interface {
	PingService() *apitypes.Result
	ScanImage(string) (*ScanResult, error)
}

type ScanResult struct {
	Vulnerabilities []*apitypes.VulnerabilityDesc
}

type Vulnerability struct {
	ID, Link, Priority, Description string
}



	/*
	Lynis:
		// Lynis scan:
		// https://cisofy.com/lynis/
		// https://cisofy.com/lynis/plugins/docker-containers/
		// /usr/local/lynis/lynis -c --checkupdate --quiet --auditor "SafeHarbor" > ....
	Baude:
		// OpenScap using RedHat/Baude image scanner:
		// https://github.com/baude/image-scanner
		// https://github.com/baude
		// https://developerblog.redhat.com/2015/04/21/introducing-the-atomic-command/
		// https://access.redhat.com/articles/881893#get
		// https://aws.amazon.com/partners/redhat/
		// https://aws.amazon.com/marketplace/pp/B00VIMU19E
		// https://aws.amazon.com/marketplace/library/ref=mrc_prm_manage_subscriptions
		// RHEL7.1 ami at Amazon: ami-4dbf9e7d
		
		//var cmd *exec.Cmd = exec.Command("image-scanner-remote.py",
		//	"--profile", "localhost", "-s", dockerImage.getDockerImageTag())
	openscap:
		// http://www.open-scap.org/resources/documentation/security-compliance-of-rhel7-docker-containers/
		
	*/
