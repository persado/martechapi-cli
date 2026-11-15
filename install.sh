#!/usr/bin/env bash
# Installs the latest persadomapicli release binary.
#   curl -fsSL https://raw.githubusercontent.com/persado/martechapi-cli/main/install.sh | bash
# Supported: macOS on Apple silicon, Linux on x86-64. The asset names must match `make dist`.
set -euo pipefail

REPO="persado/martechapi-cli"
BIN="persadomapicli"
BASE_URL="https://github.com/${REPO}/releases/latest/download"
SYSTEM_DIR="/usr/local/bin"
USER_DIR="${HOME}/.local/bin"

say()  { printf '%s\n' "$*"; }
fail() { printf 'error: %s\n' "$*" >&2; exit 1; }

detect_asset() {
  local os arch
  os=$(uname -s)
  arch=$(uname -m)
  case "$os" in
    Darwin)
      # A shell running under Rosetta reports x86_64 on Apple silicon.
      if [ "$arch" = x86_64 ] && [ "$(sysctl -n sysctl.proc_translated 2>/dev/null)" = 1 ]; then
        arch=arm64
      fi
      [ "$arch" = arm64 ] || fail "no release build for macOS on $arch; build from source: https://github.com/${REPO}#build-from-source"
      echo "${BIN}-macos-arm64"
      ;;
    Linux)
      case "$arch" in
        x86_64 | amd64) echo "${BIN}-linux-amd64" ;;
        *) fail "no release build for Linux on $arch; build from source: https://github.com/${REPO}#build-from-source" ;;
      esac
      ;;
    MINGW* | MSYS* | CYGWIN*)
      fail "on Windows, install with PowerShell: https://github.com/${REPO}#windows"
      ;;
    *) fail "unsupported OS: $os" ;;
  esac
}

# Compares hashes directly: the -c flags of GNU sha256sum, BSD sha256sum and shasum differ.
verify_checksum() {
  local dir=$1 asset=$2 expected actual
  expected=$(awk -v a="$asset" '$2 == a {print $1}' "$dir/SHA256SUMS")
  [ -n "$expected" ] || fail "$asset is not listed in SHA256SUMS"
  if command -v sha256sum >/dev/null 2>&1; then
    actual=$(sha256sum "$dir/$asset" | awk '{print $1}')
  elif command -v shasum >/dev/null 2>&1; then
    actual=$(shasum -a 256 "$dir/$asset" | awk '{print $1}')
  else
    fail "neither sha256sum nor shasum is installed; cannot verify the download"
  fi
  [ "$actual" = "$expected" ] || fail "checksum mismatch for $asset"
}

# Asks a yes/no question on the terminal. Stdin is the script itself under `curl | bash`.
confirm() {
  local answer
  printf '%s [y/N] ' "$1" > /dev/tty
  read -r answer < /dev/tty || return 1
  case "$answer" in y | Y | yes | YES) return 0 ;; *) return 1 ;; esac
}

on_path() {
  case ":${PATH}:" in *":$1:"*) return 0 ;; *) return 1 ;; esac
}

main() {
  command -v curl >/dev/null 2>&1 || fail "curl is required"

  local asset tmp dest_dir sudo=""
  asset=$(detect_asset)
  tmp=$(mktemp -d)
  # Expanded now: $tmp is local to main and gone when the EXIT trap fires.
  trap "rm -rf '$tmp'" EXIT

  say "Downloading $asset from the latest release of $REPO ..."
  curl -fsSL -o "$tmp/$asset" "$BASE_URL/$asset" || fail "download failed: $BASE_URL/$asset"
  curl -fsSL -o "$tmp/SHA256SUMS" "$BASE_URL/SHA256SUMS" || fail "download failed: $BASE_URL/SHA256SUMS"
  verify_checksum "$tmp" "$asset"
  say "Checksum verified."

  chmod +x "$tmp/$asset"
  if [ "$(uname -s)" = Darwin ]; then
    # Gatekeeper blocks quarantined unsigned binaries; curl does not normally set the flag.
    xattr -d com.apple.quarantine "$tmp/$asset" 2>/dev/null || true
  fi

  dest_dir=$SYSTEM_DIR
  if [ "$(id -u)" -ne 0 ] && ! { [ -d "$SYSTEM_DIR" ] && [ -w "$SYSTEM_DIR" ]; }; then
    say ""
    say "Installing to $SYSTEM_DIR requires sudo (administrator) access."
    say "Without sudo, $BIN is installed to $USER_DIR instead."
    if ! command -v sudo >/dev/null 2>&1; then
      say "sudo is not installed."
      dest_dir=$USER_DIR
    elif ! (: < /dev/tty) 2>/dev/null; then
      say "No terminal is available to confirm the use of sudo."
      dest_dir=$USER_DIR
    elif confirm "Use sudo to install to $SYSTEM_DIR? sudo may ask for your password."; then
      sudo=sudo
    else
      dest_dir=$USER_DIR
    fi
  fi

  $sudo mkdir -p "$dest_dir"
  $sudo install -m 0755 "$tmp/$asset" "$dest_dir/$BIN"
  say ""
  say "Installed $("$dest_dir/$BIN" version | head -n 1) to $dest_dir/$BIN"

  if ! on_path "$dest_dir"; then
    say ""
    say "$dest_dir is not on your PATH. Add it, then open a new terminal:"
    say "  echo 'export PATH=\"$dest_dir:\$PATH\"' >> ~/.$(basename "${SHELL:-bash}")rc"
  elif [ "$(command -v "$BIN")" != "$dest_dir/$BIN" ]; then
    say ""
    say "warning: another $BIN earlier on your PATH takes precedence: $(command -v "$BIN")"
  fi
}

main "$@"
