#!/bin/sh

st=0
for pkg in $(go list ./...); do
    echo "$pkg"

    go vet "$pkg"
    [ $? -ne 0 ] && st=1

    golint "$pkg"
    [ $? -ne 0 ] && st=1

    # gofmt works on files, not packages
    go fmt "$pkg"
    [ $? -ne 0 ] && st=1
done
exit $st