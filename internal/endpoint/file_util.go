package endpoint

import "os"

// readFileBytesFromDisk reads a file from disk. Separated for clarity.
func readFileBytesFromDisk(path string) ([]byte, error) {
	return os.ReadFile(path)
}
