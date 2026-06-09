#!/usr/bin/env sh
set -eu

repo="github.com/Freddster16/clibox"
bin_name="clibox"
min_go_minor="25"

load_homebrew_path() {
  if command -v brew >/dev/null 2>&1; then
    return 0
  fi

  for brew_path in /opt/homebrew/bin/brew /usr/local/bin/brew /home/linuxbrew/.linuxbrew/bin/brew; do
    if [ -x "$brew_path" ]; then
      eval "$("$brew_path" shellenv)"
      return 0
    fi
  done

  return 1
}

install_homebrew() {
  if load_homebrew_path; then
    return 0
  fi

  if [ "${CLIBOX_INSTALL_HOMEBREW:-}" != "1" ]; then
    echo "Homebrew is required to install missing clibox dependencies automatically." >&2
    echo "Install Homebrew first: https://brew.sh/" >&2
    echo "Or explicitly allow this installer to run Homebrew's official installer:" >&2
    echo "  curl -fsSL https://raw.githubusercontent.com/Freddster16/clibox/main/install.sh | CLIBOX_INSTALL_HOMEBREW=1 sh" >&2
    exit 1
  fi

  case "$(uname -s)" in
    Darwin|Linux) ;;
    *)
      echo "Homebrew install is supported on macOS and Linux." >&2
      echo "Install the email backend manually: https://github.com/pimalaya/himalaya" >&2
      exit 1
      ;;
  esac

  if ! command -v curl >/dev/null 2>&1; then
    echo "Installing Homebrew requires curl." >&2
    exit 1
  fi
  if [ ! -x /bin/bash ]; then
    echo "Installing Homebrew requires /bin/bash." >&2
    exit 1
  fi

  echo "Homebrew was not found. Installing Homebrew..."
  /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

  if ! load_homebrew_path; then
    echo "Homebrew installed, but brew is still not available in PATH." >&2
    echo "Restart your shell, then run this installer again." >&2
    exit 1
  fi
}

go_version_ok() {
  if ! command -v go >/dev/null 2>&1; then
    return 1
  fi

  version="$(go env GOVERSION 2>/dev/null || true)"
  version="${version#go}"
  major="${version%%.*}"
  rest="${version#*.}"
  minor="${rest%%.*}"
  minor="${minor%%[!0-9]*}"

  case "$major:$minor" in
    *[!0-9:]*|:|*:)
      return 1
      ;;
  esac

  [ "$major" -gt 1 ] || { [ "$major" -eq 1 ] && [ "$minor" -ge "$min_go_minor" ]; }
}

install_go() {
  if go_version_ok; then
    return 0
  fi

  install_homebrew
  echo "Installing Go 1.${min_go_minor}+ with Homebrew..."
  brew install go

  if ! go_version_ok; then
    echo "clibox install requires Go 1.${min_go_minor} or newer." >&2
    echo "Install Go first: https://go.dev/dl/" >&2
    exit 1
  fi
}

install_himalaya() {
  if command -v himalaya >/dev/null 2>&1; then
    return 0
  fi

  install_homebrew
  echo "Installing email backend with Homebrew..."
  brew install himalaya
}

install_himalaya
install_go

echo "Installing ${bin_name} from ${repo} main..."
go install "${repo}@main"

gobin="$(go env GOBIN)"
if [ -z "$gobin" ]; then
  gobin="$(go env GOPATH)/bin"
fi

echo
echo "Installed ${bin_name} to ${gobin}/${bin_name}"
echo "Email backend is available."
echo "Run it with:"
echo "  ${bin_name}"
echo "Check your email setup with:"
echo "  ${bin_name} doctor"
echo "If an account is needed, ${bin_name} will ask for your email once and configure common providers in the TUI."
echo
echo "If your shell cannot find it, add this to PATH:"
echo "  export PATH=\"${gobin}:\$PATH\""
