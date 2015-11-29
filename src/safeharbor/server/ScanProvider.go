package server

import (
	
	"safeharbor/apitypes"
)

type ScanProvider interface {
	PingService() *apitypes.Result
	ScanImage(imageName string) *apitypes.Result
}
