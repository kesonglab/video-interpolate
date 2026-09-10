package version

import "fmt"

var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

func Full() string {
	return fmt.Sprintf("vif %s (%s, %s)", Version, Commit, Date)
}
