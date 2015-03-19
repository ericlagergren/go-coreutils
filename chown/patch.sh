#!/bin/bash
set -euf -o pipefail


ORIG=$PWD

# You don't need wget the tarball unless you don't trust mine...
# MD5: 8cb8c7de1776a5badf4f0070eb009363  patch101310044_20001.tar.bz2

#PATCH=https://codereview.appspot.com/tarball/101310044/20001
FNAME=patch101310044_20001.tar.bz2
#wget -O $FNAME $PATCH

# If this fails then either the downloaded tarball was downloaded incorrectly
# or something else weird happened. In any case, go to
# https://codereview.appspot.com/101310044/ and find the correct patch,
# download the tarball, and place it inside this directory under the name
# patch101310044_20001.tar.bz2 and then run this script
# Or just do it yourself
md5sum -c checksum
if [[ $? != 0 ]]; then
	echo "checksum did not match."
	exit 1
fi

tar -xjf $FNAME

if [ ! $GOROOT ]; then
	GOROOT="$(dirname "$(which go)")"
fi

cd $GOROOT && cd ../
cd src/pkg/os/
rm --preserve-root -r a/
cp -r $ORIG/b/src/pkg/os/user/* user/
if [[ $? != 0 ]]; then
	if [[ $EUID -ne 0 ]]; then
   		echo "If 'permission denied', then this script must be run as root" 1>&2
   		exit 1
	fi
fi
cd ../../
./all.bash
if [[ $? != 0 ]]; then
	if [[ $EUID -ne 0 ]]; then
   		echo "If 'permission denied', then this script must be run as root" 1>&2
   		exit 1
	fi
fi
cd $ORIG
rm --preserve-root -r b/

exit 0
