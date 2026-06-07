#!/usr/bin/env sh
set -eu

repo="github.com/Freddster16/clibox"
bin_name="clibox"

if ! command -v go >/dev/null 2>&1; then
  echo "clibox install requires Go 1.24 or newer." >&2
  echo "Install Go first: https://go.dev/dl/" >&2
  exit 1
fi

echo "Installing ${bin_name} from ${repo}..."
go install "${repo}@latest"

gobin="$(go env GOBIN)"
if [ -z "$gobin" ]; then
  gobin="$(go env GOPATH)/bin"
fi

echo
echo "Installed ${bin_name} to ${gobin}/${bin_name}"
echo "Run it with:"
echo "  ${bin_name}"
echo
echo "If your shell cannot find it, add this to PATH:"
echo "  export PATH=\"${gobin}:\$PATH\""
