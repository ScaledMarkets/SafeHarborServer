package providers

import (
	
	"safeharbor/apitypes"
)

type ScanService interface {
	GetEndpoint() string
	GetParameterDescriptions() map[string]string
	CreateScanContext(map[string]string) (ScanContext, error)  // params may be nil
}

type ScanContext interface {
	PingService() *apitypes.Result
	ScanImage(string) (*ScanResult, error)
}

type ScanResult struct {
	Vulnerabilities []Vulnerability
}

type Vulnerability struct {
	ID, Link, Priority, Description string
}
