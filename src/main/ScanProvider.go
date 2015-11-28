package main

import (
	
	"apitypes"
)

type ScanProvider interface {
	PingService() *apitypes.Result
	ScanImage(imageName string) *apitypes.Result
}
