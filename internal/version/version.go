package version

import "runtime"

var (
	Version = "0.92"
	Commit  = "dev"
	Date    = ""
)

type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date,omitempty"`
	GoOS    string `json:"goos"`
	GoArch  string `json:"goarch"`
}

func Get() Info {
	return Info{
		Version: Version,
		Commit:  Commit,
		Date:    Date,
		GoOS:    runtime.GOOS,
		GoArch:  runtime.GOARCH,
	}
}
