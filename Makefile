APP := roll

all: clean build compress checksums

include github.com/codekoala/make/golang
include github.com/codekoala/make/upx
