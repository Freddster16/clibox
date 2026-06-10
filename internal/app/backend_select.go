package app

import "strings"

const (
	backendModeHimalaya = "himalaya"
	backendModeNative   = "native"
)

func newConfiguredBackend(options Options) (inboxBackend, accountSetup) {
	switch normalizeBackendMode(options.BackendMode) {
	case backendModeNative:
		backend := newNativeBackend(options)
		hint, _ := backend.accountHint()
		return backend, hint
	default:
		backend := newHimalayaBackend(options)
		hint, _ := himalayaAccountHint(backend.account)
		return backend, hint
	}
}

func normalizeBackendMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case backendModeNative:
		return backendModeNative
	default:
		return backendModeHimalaya
	}
}
