# Makefile for MusicRoomMobile

WORKSPACE = MusicRoomMobile.xcworkspace
SCHEME = MusicRoomMobile
DESTINATION = 'platform=iOS Simulator,name=iPhone 17'

.PHONY: build test generate clean

generate:
	tuist generate

build:
	xcodebuild build \
		-workspace $(WORKSPACE) \
		-scheme $(SCHEME) \
		-destination $(DESTINATION)

test:
	rm -rf TestResults.xcresult
	xcodebuild test \
		-workspace $(WORKSPACE) \
		-scheme $(SCHEME) \
		-destination $(DESTINATION) \
		-enableCodeCoverage YES \
		-resultBundlePath TestResults.xcresult

clean:
	xcodebuild clean \
		-workspace $(WORKSPACE) \
		-scheme $(SCHEME) \
		-destination $(DESTINATION)
