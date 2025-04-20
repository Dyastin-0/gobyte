package tofu

import "fmt"

var (
	ErrorConnectionDenied      error = fmt.Errorf("connection denied")
	ErrorNoCertificateProvided error = fmt.Errorf("no certificate provided")
	ErrorNoCertificateFound    error = fmt.Errorf("no certificate found")
	ErrorMustSpecifyCertPaths  error = fmt.Errorf("no or missing cert path")
)
