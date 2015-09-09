#!/bin/bash

set -ex

echo "$1" > VERSION
git commit -am "bumped version to v$1"
git tag "v$1"
git push --tags

# update homebrew/gondor.rb
sed -i '' "s/[0-9a-f]\{64\}/$(curl -sL --retry-delay 1 --retry-max-time 60 https://github.com/eldarion-gondor/cli/archive/v$1.tar.gz | shasum -a 256 | grep -o '[0-9a-f]\{64\}')/" homebrew/gondor.rb
sed -i '' "s/v[0-9]\{1,2\}\.[0-9]\{1,2\}\.[0-9]\{1,2\}/v$1/" homebrew/gondor.rb
git commit -am "updated homebrew for v$1"

# set everything back to dev
echo "dev" > VERSION
git commit -am "set version to dev"

git push
