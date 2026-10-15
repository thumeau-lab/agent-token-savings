#!/bin/sh
# Install the latest ats release for this OS/arch.
#
#   curl -fsSL https://raw.githubusercontent.com/thumeau-lab/agent-token-savings/main/install.sh | sh
#
# Downloads the matching archive from the latest GitHub release, verifies
# its SHA256 against checksums.txt, and installs the binary to
# ~/.local/bin/ats (creating the directory if needed). No root required.
set -eu

REPO="thumeau-lab/agent-token-savings"
INSTALL_DIR="${ATS_INSTALL_DIR:-$HOME/.local/bin}"

os() {
  case "$(uname -s)" in
    Linux) echo "linux" ;;
    Darwin) echo "darwin" ;;
    MINGW* | MSYS* | CYGWIN*) echo "windows" ;;
    *) echo "unsupported OS: $(uname -s)" >&2; exit 1 ;;
  esac
}

arch() {
  case "$(uname -m)" in
    x86_64 | amd64) echo "amd64" ;;
    aarch64 | arm64) echo "arm64" ;;
    *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
  esac
}

main() {
  need_cmd curl
  need_cmd sha256sum || need_cmd shasum

  os_name=$(os)
  arch_name=$(arch)
  ext="tar.gz"
  if [ "$os_name" = "windows" ]; then
    ext="zip"
  fi

  api_url="https://api.github.com/repos/${REPO}/releases/latest"
  tag=$(curl -fsSL "$api_url" | grep '"tag_name"' | head -1 | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
  if [ -z "$tag" ]; then
    echo "could not determine the latest release tag from $api_url" >&2
    exit 1
  fi
  version=$(echo "$tag" | sed 's/^v//')

  archive="ats_${version}_${os_name}_${arch_name}.${ext}"
  base_url="https://github.com/${REPO}/releases/download/${tag}"

  tmp_dir=$(mktemp -d)
  trap 'rm -rf "$tmp_dir"' EXIT

  echo "downloading ${archive} (${tag})..."
  curl -fsSL -o "$tmp_dir/$archive" "$base_url/$archive"
  curl -fsSL -o "$tmp_dir/checksums.txt" "$base_url/checksums.txt"

  echo "verifying checksum..."
  (
    cd "$tmp_dir"
    expected=$(grep " ${archive}\$" checksums.txt | awk '{print $1}')
    if [ -z "$expected" ]; then
      echo "no checksum entry found for ${archive} in checksums.txt" >&2
      exit 1
    fi
    actual=$(sha256_of "$archive")
    if [ "$expected" != "$actual" ]; then
      echo "checksum mismatch for ${archive}: expected $expected, got $actual" >&2
      exit 1
    fi
  )

  echo "extracting..."
  if [ "$ext" = "zip" ]; then
    need_cmd unzip
    unzip -o -q "$tmp_dir/$archive" -d "$tmp_dir/extracted"
  else
    mkdir -p "$tmp_dir/extracted"
    tar -xzf "$tmp_dir/$archive" -C "$tmp_dir/extracted"
  fi

  binary="ats"
  if [ "$os_name" = "windows" ]; then
    binary="ats.exe"
  fi

  mkdir -p "$INSTALL_DIR"
  cp "$tmp_dir/extracted/$binary" "$INSTALL_DIR/$binary"
  chmod +x "$INSTALL_DIR/$binary"

  echo "installed ${binary} ${tag} to ${INSTALL_DIR}/${binary}"
  case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *) echo "note: ${INSTALL_DIR} is not on your PATH; add it to use 'ats' directly" ;;
  esac
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1
}

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

main "$@"
