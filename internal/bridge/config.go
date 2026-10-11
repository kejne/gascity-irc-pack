package bridge

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// LoadConfig reads the bridge's environment. Secrets are accepted only from
// IRC_PASSWORD; this package never reads or writes a local IRC client config.
func LoadConfig(getenv func(string) string) (Config, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	port, err := envInt(getenv, "IRC_PORT", 6697)
	if err != nil {
		return Config{}, err
	}
	register, err := envBool(getenv, "GC_REGISTER_ADAPTER", true)
	if err != nil {
		return Config{}, err
	}
	nick := getenv("IRC_NICK")
	return Config{
		IRCServer:       envDefault(getenv("IRC_SERVER"), ""),
		IRCPort:         port,
		IRCNick:         nick,
		IRCUsername:     envDefault(getenv("IRC_USERNAME"), nick),
		IRCRealName:     envDefault(getenv("IRC_REALNAME"), "Gas City IRC bridge"),
		IRCSASLUsername: envDefault(getenv("IRC_SASL_USERNAME"), nick),
		IRCPassword:     getenv("IRC_PASSWORD"),
		IRCChannel:      getenv("IRC_CHANNEL"),
		GCAPIBaseURL:    envDefault(getenv("GC_API_BASE_URL"), "http://127.0.0.1:9443"),
		GCCityName:      getenv("GC_CITY_NAME"),
		GCTarget:        getenv("GC_TARGET"),
		GCProvider:      envDefault(getenv("GC_PROVIDER"), "irc"),
		GCAccountID:     envDefault(getenv("GC_ACCOUNT_ID"), "irc"),
		GCScopeID:       getenv("GC_SCOPE_ID"),
		RegisterAdapter: register,
		RetryInitial:    time.Second,
		RetryMax:        30 * time.Second,
	}, nil
}

func envDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func envInt(getenv func(string) string, name string, fallback int) (int, error) {
	value := strings.TrimSpace(getenv(name))
	if value == "" {
		return fallback, nil
	}
	result, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	return result, nil
}

func envBool(getenv func(string) string, name string, fallback bool) (bool, error) {
	value := strings.TrimSpace(getenv(name))
	if value == "" {
		return fallback, nil
	}
	result, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", name)
	}
	return result, nil
}
