actions_app_shell, func setDeviceMode linia 67 obecnie (zacznin sprawdzanie tam)

```go
type PreviewMode string

const (
	PreviewDesktop PreviewMode = "desktop"
	PreviewTablet  PreviewMode = "tablet"
	PreviewPhone   PreviewMode = "phone"
)

func ParsePreviewMode(raw string) (PreviewMode, bool) {
	switch m := PreviewMode(strings.ToLower(strings.TrimSpace(raw))); m {
	case PreviewDesktop, PreviewTablet, PreviewPhone:
		return m, true
	default:
		return "", false
	}
}

func setDeviceMode(s *state.AppState, r *http.Request) string {
	if s.Config.DevMode {
		if mode, ok := ParsePreviewMode(r.FormValue("mode")); ok {
			s.Settings.PreviewMode = mode
			_ = s.SaveSettings()
		}
	}
	return ""
}
```