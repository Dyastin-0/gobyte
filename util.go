package gobyte

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
)

func resolveFiles(filePaths []string) ([]FileInfo, error) {
	var files []FileInfo

	for _, path := range filePaths {
		fileInfo, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("error accessing file %s: %v", path, err)
		}

		if fileInfo.IsDir() {
			return nil, fmt.Errorf("%s is a directory, not a file", path)
		}

		files = append(files, FileInfo{
			Name: filepath.Base(path),
			Size: fileInfo.Size(),
			Path: path,
		})
	}

	return files, nil
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}

	return "127.0.0.1"
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}
