// Package cdi generates CDI (Container Device Interface) spec files
// for RDMA devices. It is extracted and adapted from the upstream
// k8s-rdma-shared-dev-plugin project with all Kubernetes dependencies removed.
package cdi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	cdiapi "tags.cncf.io/container-device-interface/pkg/cdi"
	cdiparser "tags.cncf.io/container-device-interface/pkg/parser"
	cdiSpecs "tags.cncf.io/container-device-interface/specs-go"

	"github.com/Nativu5/rdma-cdi/pkg/types"

	"sigs.k8s.io/yaml"
)

const (
	// FilePrefix is prepended to all spec files written by this tool
	// to enable safe cleanup without affecting specs from other sources.
	FilePrefix = "rdma-cdi"

	// DefaultOutputDir is the standard CDI spec directory.
	DefaultOutputDir = "/etc/cdi"

	// DefaultPrefix is used when no --prefix is provided.
	DefaultPrefix = "rdma"
)

// SpecFileName returns the deterministic file name for a given prefix, name, and format.
// Format: rdma-cdi_<prefix>_<name>.<ext>
func SpecFileName(prefix, name, format string) string {
	// Normalize: replace '/' in prefix with '_'
	safePrefix := strings.ReplaceAll(prefix, "/", "_")
	return fmt.Sprintf("%s_%s_%s.%s", FilePrefix, safePrefix, name, format)
}

// CreateCDISpec generates a CDI spec file for the given devices and writes it
// to outputDir. The file is named according to SpecFileName().
func CreateCDISpec(resourcePrefix, resourceName string, devices []types.RdmaDevice, outputDir, format string) error {
	log.Infof("creating CDI spec for resource %q (prefix=%s)", resourceName, resourcePrefix)

	cdiDevices := make([]cdiSpecs.Device, 0, len(devices))

	for _, dev := range devices {
		containerEdit := cdiSpecs.ContainerEdits{
			DeviceNodes: make([]*cdiSpecs.DeviceNode, 0, len(dev.DeviceSpecs)),
		}

		for _, spec := range dev.DeviceSpecs {
			deviceNode := cdiSpecs.DeviceNode{
				Path:        spec.ContainerPath,
				HostPath:    spec.HostPath,
				Permissions: spec.Permissions,
			}
			containerEdit.DeviceNodes = append(containerEdit.DeviceNodes, &deviceNode)
		}

		device := cdiSpecs.Device{
			Name:           dev.PciAddress,
			ContainerEdits: containerEdit,
		}
		cdiDevices = append(cdiDevices, device)
	}

	spec := &cdiSpecs.Spec{
		Version: cdiSpecs.CurrentVersion,
		Kind:    resourcePrefix + "/" + resourceName,
		Devices: cdiDevices,
	}

	fileName := SpecFileName(resourcePrefix, resourceName, format)
	filePath := filepath.Join(outputDir, fileName)

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("cannot create output directory %s: %w", outputDir, err)
	}

	// Validate the spec before writing
	if err := validateSpec(spec); err != nil {
		return fmt.Errorf("generated CDI spec is invalid: %w", err)
	}

	data, err := marshalSpec(spec, format)
	if err != nil {
		return fmt.Errorf("cannot marshal CDI spec: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("cannot write CDI spec file %s: %w", filePath, err)
	}

	log.Infof("CDI spec written to %s", filePath)
	return nil
}

// CreateContainerAnnotations generates CDI container annotations for the
// given devices. The returned map can be passed directly to a container runtime.
// Keys are CDI qualified names (vendor/class=deviceName).
func CreateContainerAnnotations(devices []types.RdmaDevice, resourcePrefix, resourceKind string) (map[string]string, error) {
	if len(devices) == 0 {
		return nil, fmt.Errorf("devices list is empty")
	}

	annotations := make(map[string]string)
	for _, dev := range devices {
		qn := cdiparser.QualifiedName(resourcePrefix, resourceKind, dev.PciAddress)
		// Each qualified name is its own annotation key=value pair
		annotations[qn] = qn
	}

	log.Debugf("created CDI annotations: %v", annotations)
	return annotations, nil
}

// CleanupSpecs removes CDI spec files created by this tool from dir.
// If name is empty, all specs matching the given prefix are removed.
// If name is non-empty, only the exact match is removed.
func CleanupSpecs(dir, prefix, name string, dryRun bool) ([]string, error) {
	if dir == "" {
		dir = DefaultOutputDir
	}

	safePrefix := strings.ReplaceAll(prefix, "/", "_")
	var pattern string
	if name != "" {
		// Exact match (both json and yaml)
		patternJSON := filepath.Join(dir, fmt.Sprintf("%s_%s_%s.json", FilePrefix, safePrefix, name))
		patternYAML := filepath.Join(dir, fmt.Sprintf("%s_%s_%s.yaml", FilePrefix, safePrefix, name))
		return cleanupFiles([]string{patternJSON, patternYAML}, dryRun)
	}

	// Match all specs under the given prefix — restrict to known extensions only
	var matches []string
	for _, ext := range []string{"json", "yaml"} {
		pattern = filepath.Join(dir, fmt.Sprintf("%s_%s_*.%s", FilePrefix, safePrefix, ext))
		m, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("glob error for pattern %s: %w", pattern, err)
		}
		matches = append(matches, m...)
	}
	return cleanupFiles(matches, dryRun)
}

func cleanupFiles(paths []string, dryRun bool) ([]string, error) {
	removed := make([]string, 0)
	for _, p := range paths {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			continue
		}
		if dryRun {
			log.Infof("[dry-run] would remove: %s", p)
			removed = append(removed, p)
			continue
		}
		log.Infof("removing CDI spec file: %s", p)
		if err := os.Remove(p); err != nil {
			return removed, fmt.Errorf("cannot remove %s: %w", p, err)
		}
		removed = append(removed, p)
	}
	return removed, nil
}

// validateSpec performs basic validation on a CDI spec.
func validateSpec(spec *cdiSpecs.Spec) error {
	if spec.Kind == "" {
		return fmt.Errorf("spec kind must not be empty")
	}
	if len(spec.Devices) == 0 {
		return fmt.Errorf("spec must contain at least one device")
	}
	return nil
}

// marshalSpec serializes a CDI spec to JSON or YAML bytes.
func marshalSpec(spec *cdiSpecs.Spec, format string) ([]byte, error) {
	_ = cdiapi.GetDefaultCache() // ensure CDI cache is initialized

	switch strings.ToLower(format) {
	case "json":
		return json.MarshalIndent(spec, "", "  ")
	case "yaml":
		jsonData, err := json.Marshal(spec)
		if err != nil {
			return nil, err
		}
		return yaml.JSONToYAML(jsonData)
	default:
		return nil, fmt.Errorf("unsupported format %q: use json or yaml", format)
	}
}
