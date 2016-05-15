/*******************************************************************************
 * General purpose utility functions.
 */

package utils

import (
	"fmt"
	"os"
	"hash"
	//"runtime/debug"
	
	// SafeHarbor packages:
)

/*******************************************************************************
 * Return the SHA-512 hash of the content of the specified file. Should not be salted
 * because the hash is intended to be reproducible by third parties, given the
 * original file.
 */
func ComputeFileSignature(hash hash.Hash, filepath string) ([]byte, error) {
	
	var file *os.File
	var err error
	file, err = os.Open(filepath)
	if err != nil { return nil, err }
	var buf = make([]byte, 10000)
	var totalBytesRead int = 0
	for {
		var numBytesRead int
		numBytesRead, err = file.Read(buf)
		totalBytesRead += numBytesRead
		if numBytesRead == 0 { break }
		if numBytesRead < 10000 {
			hash.Write(buf[0:numBytesRead])
			break
		}
		hash.Write(buf)
	}
	
	fmt.Println("Total bytes read:", totalBytesRead)
	
	var empty = []byte{}
	var sig = hash.Sum(empty)
	
	var fileInfo os.FileInfo
	fileInfo, _ = file.Stat()
	fmt.Println(fmt.Sprintf("Signature of file %s, size %d:", filepath, fileInfo.Size()))
	for _, b := range sig {
		fmt.Print(b, ", ")
	}
	fmt.Println()
	
	return sig, nil
}
