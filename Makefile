.SILENT:
.EXPORT_ALL_VARIABLES:

.PHONY: b
b:
	rm -rf /tmp/e2e-test-landslide/nodes/
	./scripts/build.sh /tmp/e2e-test-landslide/avalanchego/plugins/pjSL9ksard4YE96omaiTkGL5H6XX2W5VEo3ZgWC9S2P6gzs9A


ba:
	BASEDIR=/tmp/e2e-test-landslide AVALANCHEGO_BUILD_PATH=/tmp/e2e-test-landslide/avalanchego ./scripts/install_avalanchego_release.sh
.PHONY: ba

.PHONY: w
w:
	rm -rf /tmp/e2e-test-landslide/nodes/
	./scripts/build_wasm.sh /tmp/e2e-test-landslide/avalanchego/plugins/pjSL9ksard4YE96omaiTkGL5H6XX2W5VEo3ZgWC9S2P6gzs9A
