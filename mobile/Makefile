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
	@if [ ! -d "$(WORKSPACE)" ]; then \
		echo "Workspace not found. Run 'make generate' first or skip mobile tests."; \
		exit 1; \
	fi
	rm -rf TestResults.xcresult
	set -o pipefail && xcodebuild test \
		-workspace $(WORKSPACE) \
		-scheme $(SCHEME) \
		-destination $(DESTINATION) \
		-enableCodeCoverage YES \
		-resultBundlePath TestResults.xcresult \
		| xcbeautify

clean:
	xcodebuild clean \
		-workspace $(WORKSPACE) \
		-scheme $(SCHEME) \
		-destination $(DESTINATION)
