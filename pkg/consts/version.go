package consts

import "runtime/debug"

// vars below are set by '-X' flag
var (
	Version    = "0.20.3 (Bucaneiros fork - Send MKV as Video)"
	Commit     = "unknown"
	CommitDate = "unknown"
)

func init() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			if Commit == "unknown" && len(setting.Value) >= 7 {
				Commit = setting.Value[:7]
			}
		case "vcs.time":
			if CommitDate == "unknown" {
				CommitDate = setting.Value
			}
		}
	}
}
