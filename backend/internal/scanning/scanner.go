package scanning

package scanning

import "context"

// Result is the normalized output from a vulnerability scan.
type Result struct {
	Target   string
	Source   string
	Critical int
	High     int
	Medium   int
	Low      int
	Unknown  int
	RawJSON  string
	Scanned  int64
}

// Scanner executes vulnerability scans for a target image.
type Scanner interface {
	Name() string
	Scan(ctx context.Context, target string) (Result, error)
}
