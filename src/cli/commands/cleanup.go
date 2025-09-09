package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gnostream/src/config"
	"gnostream/src/nostr"
)

// CleanupCommand handles cleanup operations
type CleanupCommand struct {
	config      *config.Config
	nostrClient nostr.Client
}

// NewCleanupCommand creates a new cleanup command
func NewCleanupCommand(cfg *config.Config) *CleanupCommand {
	return &CleanupCommand{
		config: cfg,
	}
}

// Execute runs the cleanup command
func (c *CleanupCommand) Execute(args []string) error {
	if len(args) == 0 {
		c.printUsage()
		return nil
	}

	subcommand := args[0]

	switch subcommand {
	case "stale":
		return c.handleStaleEvents(args[1:])
	case "segments":
		return c.handleOldSegments(args[1:])
	case "archives":
		return c.handleOldArchives(args[1:])
	case "all":
		return c.handleCleanAll(args[1:])
	case "dry-run":
		return c.handleDryRun(args[1:])
	case "--help", "help":
		c.printUsage()
		return nil
	default:
		fmt.Printf("Unknown cleanup subcommand: %s\n\n", subcommand)
		c.printUsage()
		return fmt.Errorf("unknown subcommand: %s", subcommand)
	}
}

// printUsage prints cleanup command usage
func (c *CleanupCommand) printUsage() {
	fmt.Println(`CLEANUP & MAINTENANCE

USAGE:
    gnostream cleanup <SUBCOMMAND> [OPTIONS]

SUBCOMMANDS:
    stale               Clean up stale Nostr live events
    segments            Remove old HLS segments (non-recorded streams)
    archives            Clean old archived streams
    all                 Run all cleanup operations
    dry-run             Show what would be cleaned without doing it

OPTIONS:
    --older-than <days>  Only clean files older than N days (default: 7)
    --confirm            Skip confirmation prompts

EXAMPLES:
    gnostream cleanup stale
    gnostream cleanup segments --older-than 30
    gnostream cleanup archives --older-than 90 --confirm
    gnostream cleanup dry-run`)
}

// handleStaleEvents cleans up stale Nostr live events
func (c *CleanupCommand) handleStaleEvents(args []string) error {
	fmt.Println("🧹 CLEANING STALE NOSTR EVENTS")
	fmt.Println()

	// Initialize Nostr client
	if err := c.initNostrClient(); err != nil {
		return fmt.Errorf("failed to initialize Nostr client: %w", err)
	}

	fmt.Println("🔍 Scanning for stale live events...")
	
	// This is a placeholder - in a real implementation, you would:
	// 1. Query relays for your live events
	// 2. Check which ones are older than a threshold
	// 3. Publish deletion events for stale ones
	
	fmt.Println("⚠️  Note: Stale event cleanup not yet implemented")
	fmt.Println("💡 This feature requires implementing:")
	fmt.Println("   - Relay querying for your events")
	fmt.Println("   - Age-based filtering logic")
	fmt.Println("   - Automated deletion event publishing")

	return nil
}

// handleOldSegments removes old HLS segments
func (c *CleanupCommand) handleOldSegments(args []string) error {
	fmt.Println("🧹 CLEANING OLD HLS SEGMENTS")
	fmt.Println()

	// Parse options
	olderThanDays := 7
	confirm := false
	
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--older-than":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &olderThanDays)
				i++
			}
		case "--confirm":
			confirm = true
		}
	}

	streamDefaults := c.config.GetStreamDefaults()
	outputDir := streamDefaults.OutputDir

	fmt.Printf("📁 Scanning directory: %s\n", outputDir)
	fmt.Printf("⏰ Looking for files older than %d days\n", olderThanDays)
	fmt.Println()

	cutoffTime := time.Now().AddDate(0, 0, -olderThanDays)
	
	// Find old .ts files
	oldFiles, totalSize, err := c.findOldFiles(outputDir, ".ts", cutoffTime)
	if err != nil {
		return fmt.Errorf("failed to scan for old files: %w", err)
	}

	if len(oldFiles) == 0 {
		fmt.Println("✅ No old segment files found")
		return nil
	}

	fmt.Printf("🗑️  Found %d old segment files (%s total)\n", len(oldFiles), formatFileSize(totalSize))

	// Show some examples
	fmt.Println("\nFiles to be deleted:")
	for i, file := range oldFiles {
		if i >= 5 {
			fmt.Printf("   ... and %d more files\n", len(oldFiles)-5)
			break
		}
		fmt.Printf("   🗑️  %s\n", filepath.Base(file.path))
	}

	if !confirm {
		fmt.Print("\nProceed with cleanup? (y/N): ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Cleanup cancelled")
			return nil
		}
	}

	// Delete files
	deletedCount := 0
	for _, file := range oldFiles {
		if err := os.Remove(file.path); err != nil {
			fmt.Printf("❌ Failed to delete %s: %v\n", file.path, err)
		} else {
			deletedCount++
		}
	}

	fmt.Printf("✅ Deleted %d segment files\n", deletedCount)
	return nil
}

// handleOldArchives cleans old archived streams
func (c *CleanupCommand) handleOldArchives(args []string) error {
	fmt.Println("🧹 CLEANING OLD ARCHIVES")
	fmt.Println()

	// Parse options
	olderThanDays := 90 // Default to 90 days for archives
	confirm := false
	
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--older-than":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &olderThanDays)
				i++
			}
		case "--confirm":
			confirm = true
		}
	}

	streamDefaults := c.config.GetStreamDefaults()
	archiveDir := streamDefaults.ArchiveDir

	fmt.Printf("📁 Scanning archive directory: %s\n", archiveDir)
	fmt.Printf("⏰ Looking for archives older than %d days\n", olderThanDays)
	fmt.Println()

	cutoffTime := time.Now().AddDate(0, 0, -olderThanDays)
	
	// Find old archive directories
	oldArchives, err := c.findOldArchives(archiveDir, cutoffTime)
	if err != nil {
		return fmt.Errorf("failed to scan for old archives: %w", err)
	}

	if len(oldArchives) == 0 {
		fmt.Println("✅ No old archives found")
		return nil
	}

	fmt.Printf("🗑️  Found %d old archives\n", len(oldArchives))

	// Show examples
	fmt.Println("\nArchives to be deleted:")
	for i, archive := range oldArchives {
		if i >= 5 {
			fmt.Printf("   ... and %d more archives\n", len(oldArchives)-5)
			break
		}
		fmt.Printf("   🗑️  %s\n", filepath.Base(archive.path))
	}

	if !confirm {
		fmt.Print("\nProceed with archive cleanup? (y/N): ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Cleanup cancelled")
			return nil
		}
	}

	// Delete archive directories
	deletedCount := 0
	for _, archive := range oldArchives {
		if err := os.RemoveAll(archive.path); err != nil {
			fmt.Printf("❌ Failed to delete %s: %v\n", archive.path, err)
		} else {
			deletedCount++
		}
	}

	fmt.Printf("✅ Deleted %d archives\n", deletedCount)
	return nil
}

// handleCleanAll runs all cleanup operations
func (c *CleanupCommand) handleCleanAll(args []string) error {
	fmt.Println("🧹 RUNNING ALL CLEANUP OPERATIONS")
	fmt.Println()

	// Run each cleanup operation
	operations := []struct {
		name string
		fn   func([]string) error
	}{
		{"Stale Events", c.handleStaleEvents},
		{"Old Segments", c.handleOldSegments},
		{"Old Archives", c.handleOldArchives},
	}

	for _, op := range operations {
		fmt.Printf("🔄 Running %s cleanup...\n", op.name)
		if err := op.fn(args); err != nil {
			fmt.Printf("❌ %s cleanup failed: %v\n", op.name, err)
		}
		fmt.Println()
	}

	fmt.Println("✅ All cleanup operations completed")
	return nil
}

// handleDryRun shows what would be cleaned without doing it
func (c *CleanupCommand) handleDryRun(args []string) error {
	fmt.Println("🔍 DRY RUN - SHOWING WHAT WOULD BE CLEANED")
	fmt.Println()

	olderThanDays := 7
	for i := 0; i < len(args); i++ {
		if args[i] == "--older-than" && i+1 < len(args) {
			fmt.Sscanf(args[i+1], "%d", &olderThanDays)
			i++
		}
	}

	cutoffTime := time.Now().AddDate(0, 0, -olderThanDays)
	streamDefaults := c.config.GetStreamDefaults()

	// Check segments
	fmt.Println("📁 HLS SEGMENTS:")
	oldFiles, totalSize, err := c.findOldFiles(streamDefaults.OutputDir, ".ts", cutoffTime)
	if err != nil {
		fmt.Printf("   ❌ Error scanning: %v\n", err)
	} else if len(oldFiles) == 0 {
		fmt.Println("   ✅ No old segments found")
	} else {
		fmt.Printf("   🗑️  Would delete %d files (%s)\n", len(oldFiles), formatFileSize(totalSize))
	}

	// Check archives
	fmt.Println("\n📦 ARCHIVES:")
	oldArchives, err := c.findOldArchives(streamDefaults.ArchiveDir, cutoffTime)
	if err != nil {
		fmt.Printf("   ❌ Error scanning: %v\n", err)
	} else if len(oldArchives) == 0 {
		fmt.Println("   ✅ No old archives found")
	} else {
		fmt.Printf("   🗑️  Would delete %d archives\n", len(oldArchives))
	}

	fmt.Println("\n💡 Run without dry-run to actually perform cleanup")
	return nil
}

// initNostrClient initializes the Nostr client
func (c *CleanupCommand) initNostrClient() error {
	if c.nostrClient != nil {
		return nil
	}

	client, err := nostr.NewClient(&c.config.Nostr)
	if err != nil {
		return err
	}

	c.nostrClient = client
	return nil
}

// FileInfo represents a file with metadata
type FileInfo struct {
	path    string
	modTime time.Time
	size    int64
}

// findOldFiles finds files with specific extension older than cutoff time
func (c *CleanupCommand) findOldFiles(dir, ext string, cutoff time.Time) ([]FileInfo, int64, error) {
	var oldFiles []FileInfo
	var totalSize int64

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue walking, ignore errors
		}

		if info.IsDir() {
			return nil
		}

		if strings.ToLower(filepath.Ext(path)) == ext {
			if info.ModTime().Before(cutoff) {
				oldFiles = append(oldFiles, FileInfo{
					path:    path,
					modTime: info.ModTime(),
					size:    info.Size(),
				})
				totalSize += info.Size()
			}
		}

		return nil
	})

	return oldFiles, totalSize, err
}

// findOldArchives finds archive directories older than cutoff time
func (c *CleanupCommand) findOldArchives(dir string, cutoff time.Time) ([]FileInfo, error) {
	var oldArchives []FileInfo

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			path := filepath.Join(dir, entry.Name())
			if info, err := entry.Info(); err == nil {
				if info.ModTime().Before(cutoff) {
					oldArchives = append(oldArchives, FileInfo{
						path:    path,
						modTime: info.ModTime(),
						size:    0, // Directory size calculation would be complex
					})
				}
			}
		}
	}

	return oldArchives, nil
}

// formatFileSize formats byte size into human readable format
func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}