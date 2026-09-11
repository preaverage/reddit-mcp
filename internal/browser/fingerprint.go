package browser

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	envConfig      = "CAMOU_CONFIG"
	configKeyUA    = "navigator.userAgent"
	appIni         = "application.ini"
	versionKey     = "Version="
	fallbackMajor  = "148"
	userAgentFmt   = "Mozilla/5.0 (%s; rv:%s.0) Gecko/20100101 Firefox/%s.0"
	platformLinux  = "X11; Linux x86_64"
	platformWin    = "Windows NT 10.0; Win64; x64"
	platformDarwin = "Macintosh; Intel Mac OS X 10.15"
)

func platform() string {
	switch runtime.GOOS {
	case "windows":
		return platformWin
	case "darwin":
		return platformDarwin
	default:
		return platformLinux
	}
}

func userAgent(major string) string {
	return fmt.Sprintf(userAgentFmt, platform(), major, major)
}

func firefoxMajor(executable string) string {
	b, err := os.ReadFile(filepath.Join(filepath.Dir(executable), appIni))
	if err != nil {
		return fallbackMajor
	}

	for _, line := range strings.Split(string(b), "\n") {
		v, ok := strings.CutPrefix(strings.TrimSpace(line), versionKey)
		if !ok {
			continue
		}

		if major, _, found := strings.Cut(v, "."); found && major != "" {
			return major
		}
	}

	return fallbackMajor
}

func environ() map[string]string {
	pairs := os.Environ()
	env := make(map[string]string, len(pairs)+1)

	for _, kv := range pairs {
		if k, v, ok := strings.Cut(kv, "="); ok {
			env[k] = v
		}
	}

	return env
}

func fingerprintEnv(executable string) map[string]string {
	env := environ()
	if _, set := env[envConfig]; set {
		return env
	}

	cfg, _ := json.Marshal(map[string]string{
		configKeyUA: userAgent(firefoxMajor(executable)),
	})
	env[envConfig] = string(cfg)

	return env
}
