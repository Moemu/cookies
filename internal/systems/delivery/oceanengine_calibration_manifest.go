package delivery

import "github.com/shikanon/cookies/internal/systems/delivery/calibrationmanifest"

// currentOceanEngineCalibrationManifest validates the one frozen source before
// a domain contract accepts its binding. The parser lives below both Delivery
// and Platform Skills to prevent a second field map or an import cycle.
func currentOceanEngineCalibrationManifest() (calibrationmanifest.Manifest, error) {
	return calibrationmanifest.Current()
}
