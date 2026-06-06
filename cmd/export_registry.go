package cmd

import "github.com/shapedthought/owlctl/models"

// ResourceExporter defines how to export and snapshot a specific resource type
// within a product. Either callback may be nil if the operation is not supported.
type ResourceExporter struct {
	// Kind matches the resource type string in state.json (e.g. "VBRJob")
	Kind string

	// FolderName is the output subdirectory in the export tree (e.g. "jobs")
	FolderName string

	// ExportAll writes all resources of this type to outDir as YAML files.
	// nil means export is not supported for this resource type.
	ExportAll func(outDir string, profile models.Profile) error

	// SnapshotAll snapshots all resources of this type into state.json.
	// nil means snapshot is not supported for this resource type.
	SnapshotAll func(profile models.Profile) error
}

// ProductExporter groups ResourceExporters by product name (e.g. "vbr").
type ProductExporter struct {
	Product   string
	Resources []ResourceExporter
}

// exportRegistry holds all registered product exporters.
// Populated via RegisterProductExporter() calls in init() functions.
var exportRegistry []ProductExporter

// RegisterProductExporter registers a ProductExporter.
// Called from product-specific init() functions (e.g. cmd/vbr_exports.go).
func RegisterProductExporter(p ProductExporter) {
	exportRegistry = append(exportRegistry, p)
}

// findProductExporter returns the ProductExporter for the given product name, or nil.
func findProductExporter(product string) *ProductExporter {
	for i := range exportRegistry {
		if exportRegistry[i].Product == product {
			return &exportRegistry[i]
		}
	}
	return nil
}
