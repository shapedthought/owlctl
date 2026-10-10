#!/usr/bin/env bash
# Fail if any owlctl release download URL referenced in docs/ or examples/ returns an HTTP error.
# Also checks the URLs the install template builds for 'latest' and for its default tag.
# Migration guides are skipped: they intentionally reference old releases.
set -euo pipefail

cd "$(dirname "$0")/.."

TEMPLATE=examples/pipelines/templates/install-owlctl.yml
BASE=https://github.com/shapedthought/owlctl/releases
ASSET=owlctl-linux-amd64.tar.gz

default_tag=$(sed -n "/name: owlctlVersion/,/default:/ s/.*default: *'\{0,1\}\([^' ]*\)'\{0,1\}.*/\1/p" "$TEMPLATE" | tail -n1)
if [ -z "$default_tag" ]; then
  echo "could not read default owlctlVersion from $TEMPLATE" >&2
  exit 1
fi

urls=$(
  {
    grep -rhoE "https://github\.com/shapedthought/owlctl/releases/[^ \"'\`)>|]+" docs examples \
      --exclude='migration-*.md' || true
    echo "$BASE/latest/download/$ASSET"
    echo "$BASE/latest/download/$ASSET.sha256"
    echo "$BASE/download/$default_tag/$ASSET"
    echo "$BASE/download/$default_tag/$ASSET.sha256"
  } | grep -vE '[$\{]' | sort -u
)

fail=0
for url in $urls; do
  if curl -fsIL -o /dev/null "$url"; then
    echo "ok    $url"
  else
    echo "FAIL  $url" >&2
    fail=1
  fi
done
exit $fail
