package delivery

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/shikanon/cookies/internal/systems/delivery/calibrationmanifest"
)

// currentOceanEngineCalibrationManifest validates the one frozen source before
// a domain contract accepts its binding. The parser lives below both Delivery
// and Platform Skills to prevent a second field map or an import cycle.
func currentOceanEngineCalibrationManifest() (calibrationmanifest.Manifest, error) {
	return calibrationmanifest.Current()
}

func manifestAllowsMarketingPurpose(value string) bool {
	manifest, err := currentOceanEngineCalibrationManifest()
	if err != nil {
		return false
	}
	for _, entry := range manifest.ConditionVocabulary {
		if entry.Key != "marketing_purpose" {
			continue
		}
		for _, allowed := range entry.KnownValues {
			if value == allowed {
				return true
			}
		}
	}
	return false
}

// validateManifestContractOwnership proves that every non-evidence Manifest
// mapping reaches a real field in its declared domain contract. It follows the
// Manifest contract_path, rather than maintaining a second field-key map.
func validateManifestContractOwnership(manifest calibrationmanifest.Manifest) error {
	for _, mapping := range manifest.ConsumerMappings {
		if mapping.Treatment == calibrationmanifest.EvidenceOnly || mapping.Destination == calibrationmanifest.PlatformSkill {
			continue
		}
		root, prefix := manifestContractRoot(mapping.Destination)
		if root == nil || !strings.HasPrefix(mapping.ContractPath, prefix+".") {
			return fmt.Errorf("%w: unsupported contract path %q", calibrationmanifest.ErrInvalid, mapping.ContractPath)
		}
		if err := manifestPathExists(root, strings.TrimPrefix(mapping.ContractPath, prefix+".")); err != nil {
			return fmt.Errorf("%w: %s", calibrationmanifest.ErrInvalid, err)
		}
	}
	return nil
}

func manifestContractRoot(destination calibrationmanifest.Consumer) (reflect.Type, string) {
	switch destination {
	case calibrationmanifest.DeliveryIntent:
		return reflect.TypeOf(DeliveryIntent{}), "DeliveryIntent"
	case calibrationmanifest.OceanEngineConfiguration:
		return reflect.TypeOf(OceanEngineConfiguration{}), "OceanEngineConfiguration"
	case calibrationmanifest.DeliveryDecisionCandidate:
		return reflect.TypeOf(DeliveryDecisionCandidate{}), "DeliveryDecisionCandidate"
	case calibrationmanifest.CompiledDeliveryWorkflow:
		return reflect.TypeOf(CompiledDeliveryWorkflow{}), "CompiledDeliveryWorkflow"
	default:
		return nil, ""
	}
}

func manifestPathExists(root reflect.Type, path string) error {
	current := root
	for _, rawSegment := range strings.Split(path, ".") {
		segment := strings.TrimSuffix(rawSegment, "[]")
		for current.Kind() == reflect.Pointer || current.Kind() == reflect.Slice || current.Kind() == reflect.Array {
			current = current.Elem()
		}
		if current.Kind() != reflect.Struct {
			return fmt.Errorf("contract path %q reaches non-struct before %q", path, rawSegment)
		}
		field, found := current.FieldByName(segment)
		if !found {
			return fmt.Errorf("contract path %q has no field %q", path, segment)
		}
		current = field.Type
	}
	return nil
}
