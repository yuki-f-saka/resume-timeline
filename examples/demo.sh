#!/usr/bin/env bash
# Builds a throwaway git repo with four versions of the sample resume and
# renders the timeline for it. Run from the repository root:
#   ./examples/demo.sh
set -euo pipefail

cd "$(dirname "$0")/.."
demo=$(mktemp -d)
trap 'rm -rf "$demo"' EXIT

git -C "$demo" init -q
git -C "$demo" -c user.name=demo -c user.email=demo@example.com commit -q --allow-empty -m init

commit() { # $1=version file, $2=message
  cp "examples/versions/$1" "$demo/resume.pdf"
  git -C "$demo" add resume.pdf
  git -C "$demo" -c user.name=demo -c user.email=demo@example.com commit -q -m "$2"
}

commit v1.pdf "Initial resume"
commit v2.pdf "Add audit pipeline bullet, mention high availability in summary"
commit v3.pdf "Update peak traffic metric, drop legacy PHP bullet"
commit v4.pdf "Add DynamoDB and Kubernetes to skills"

go run . -file "$demo/resume.pdf" -out demo-timeline.html
echo "open demo-timeline.html"
