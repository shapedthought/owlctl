package cmd

// vbr_exports.go registers all VBR resource types with the export/snapshot registry.
// Adding a new VBR resource type only requires adding an entry here — no changes
// to the global export or snapshot commands are needed.

func init() {
	RegisterProductExporter(ProductExporter{
		Product: "vbr",
		Resources: []ResourceExporter{
			{
				Kind:        "VBRJob",
				FolderName:  "jobs",
				ExportAll:   exportAllJobsToDir,
				SnapshotAll: nil, // Jobs are managed declaratively via 'job apply', not snapshot
			},
			{
				Kind:        "VBRRepository",
				FolderName:  "repos",
				ExportAll:   exportAllReposToDir,
				SnapshotAll: snapshotAllReposForRegistry,
			},
			{
				Kind:        "VBRScaleOutRepository",
				FolderName:  "sobrs",
				ExportAll:   exportAllSobrsToDir,
				SnapshotAll: snapshotAllSobrsForRegistry,
			},
			{
				Kind:        "VBREncryptionPassword",
				FolderName:  "encryption",
				ExportAll:   exportAllEncryptionToDir,
				SnapshotAll: snapshotAllEncryptionForRegistry,
			},
			{
				Kind:        "VBRKmsServer",
				FolderName:  "kms",
				ExportAll:   exportAllKmsToDir,
				SnapshotAll: snapshotAllKmsForRegistry,
			},
		},
	})
}
