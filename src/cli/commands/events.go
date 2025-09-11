package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	nostrTypes "github.com/0ceanslim/grain/server/types"
	
	"gnostream/src/config"
	"gnostream/src/nostr"
)

// EventsCommand handles Nostr event management
type EventsCommand struct {
	config      *config.Config
	nostrClient nostr.Client
}

// NewEventsCommand creates a new events command
func NewEventsCommand(cfg *config.Config) *EventsCommand {
	return &EventsCommand{
		config: cfg,
	}
}

// Execute runs the events command
func (e *EventsCommand) Execute(args []string) error {
	if len(args) == 0 {
		e.printUsage()
		return nil
	}

	// Initialize Nostr client
	if err := e.initNostrClient(); err != nil {
		return fmt.Errorf("failed to initialize Nostr client: %w", err)
	}

	subcommand := args[0]

	switch subcommand {
	case "list":
		return e.handleList(args[1:])
	case "search":
		return e.handleSearch(args[1:])
	case "delete":
		return e.handleDelete(args[1:])
	case "show":
		return e.handleShow(args[1:])
	case "publish":
		return e.handlePublish(args[1:])
	case "deletions":
		return e.handleDeletions(args[1:])
	case "--help", "help":
		e.printUsage()
		return nil
	default:
		fmt.Printf("Unknown events subcommand: %s\n\n", subcommand)
		e.printUsage()
		return fmt.Errorf("unknown subcommand: %s", subcommand)
	}
}

// printUsage prints events command usage
func (e *EventsCommand) printUsage() {
	fmt.Println(`NOSTR EVENTS MANAGEMENT

USAGE:
    gnostream events <SUBCOMMAND> [OPTIONS]

SUBCOMMANDS:
    list                List all your stream events
    search <query>      Search events by title/summary
    delete <id>         Delete specific event by ID
    show <id>           Show detailed event information
    publish <type>      Publish new event (start|end|update)
    deletions           List deletion requests you've sent

OPTIONS:
    --limit <n>         Limit number of results (default: 20)
    --status <status>   Filter by status (live|ended)
    --recent            Show only recent events (last 24h)

EXAMPLES:
    gnostream events list
    gnostream events list --limit 50 --recent
    gnostream events search "gaming"
    gnostream events delete 1234567890abcdef
    gnostream events show 1234567890abcdef
    gnostream events publish update`)
}

// initNostrClient initializes the Nostr client
func (e *EventsCommand) initNostrClient() error {
	if e.nostrClient != nil {
		return nil
	}

	client, err := nostr.NewClient(&e.config.Nostr)
	if err != nil {
		return err
	}

	e.nostrClient = client
	return nil
}

// handleList lists stream events
func (e *EventsCommand) handleList(args []string) error {
	fmt.Println("🔍 Fetching your stream events...")

	// Parse options
	limit := 20
	statusFilter := ""
	recent := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--limit":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &limit)
				i++
			}
		case "--status":
			if i+1 < len(args) {
				statusFilter = args[i+1]
				i++
			}
		case "--recent":
			recent = true
		}
	}

	// Fetch events from relays
	events, err := e.fetchStreamEvents(limit, statusFilter, recent)
	if err != nil {
		return fmt.Errorf("failed to fetch events: %w", err)
	}

	if len(events) == 0 {
		fmt.Println("📭 No stream events found")
		return nil
	}

	fmt.Printf("\n📺 Found %d stream events:\n\n", len(events))

	// Display events in table format  
	fmt.Printf("%-64s %-10s %-20s %-30s\n", "EVENT ID", "STATUS", "CREATED", "TITLE")
	fmt.Println(strings.Repeat("-", 130))

	for _, event := range events {
		status := e.getEventStatus(event)
		created := time.Unix(event.CreatedAt, 0).Format("2006-01-02 15:04")
		title := e.getEventTitle(event)
		if len(title) > 28 {
			title = title[:28] + "..."
		}

		fmt.Printf("%-64s %-10s %-20s %-30s\n", 
			event.ID, status, created, title)
	}

	return nil
}

// handleSearch searches for events
func (e *EventsCommand) handleSearch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing search query")
	}

	query := strings.Join(args, " ")
	fmt.Printf("🔍 Searching for events matching: %s\n", query)

	events, err := e.fetchStreamEvents(50, "", false)
	if err != nil {
		return err
	}

	matchingEvents := e.filterEventsByQuery(events, query)

	if len(matchingEvents) == 0 {
		fmt.Printf("📭 No events found matching '%s'\n", query)
		return nil
	}

	fmt.Printf("\n📺 Found %d matching events:\n\n", len(matchingEvents))

	for _, event := range matchingEvents {
		status := e.getEventStatus(event)
		title := e.getEventTitle(event)
		summary := e.getEventSummary(event)

		fmt.Printf("ID: %s\n", event.ID)
		fmt.Printf("Status: %s\n", status)
		fmt.Printf("Title: %s\n", title)
		fmt.Printf("Summary: %s\n", summary)
		fmt.Printf("Created: %s\n", time.Unix(event.CreatedAt, 0).Format("2006-01-02 15:04:05"))
		fmt.Println(strings.Repeat("-", 50))
	}

	return nil
}

// handleDelete deletes a specific event
func (e *EventsCommand) handleDelete(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing event ID")
	}

	eventID := args[0]
	fmt.Printf("🗑️  Deleting event: %s\n", eventID)

	// First verify the event exists
	fmt.Println("🔍 Verifying event exists...")
	event, err := e.fetchEventByID(eventID)
	if err != nil {
		return fmt.Errorf("❌ Cannot delete - event not found: %v", err)
	}
	
	fmt.Printf("✅ Found event: %s\n", e.getEventTitle(*event))

	// Create and publish deletion event with detailed response
	deletionJSON, successfulRelays := e.nostrClient.BroadcastDeletionEventWithResponse(eventID, "Deleted via gnostream CLI")
	
	if len(successfulRelays) == 0 {
		return fmt.Errorf("❌ Deletion request failed - no relays accepted")
	}
	
	fmt.Println("📡 Relay responses:")
	allRelays := []string{"wss://relay.damus.io", "wss://nos.lol", "wss://relay.nostr.band", "wss://wheat.happytavern.co"}
	
	for _, relay := range allRelays {
		accepted := false
		for _, successRelay := range successfulRelays {
			if successRelay == relay {
				accepted = true
				break
			}
		}
		
		if accepted {
			fmt.Printf("   ✅ ACCEPTED %s\n", relay)
		} else {
			fmt.Printf("   ❌ REJECTED %s\n", relay)
		}
	}
	
	// Show deletion event ID
	if len(deletionJSON) > 0 {
		var deletionEvent map[string]interface{}
		if err := json.Unmarshal([]byte(deletionJSON), &deletionEvent); err == nil {
			if id, ok := deletionEvent["id"].(string); ok {
				fmt.Printf("\n🗑️ Deletion event ID: %s\n", id)
			}
		}
	}

	// Check for associated recordings and prompt for deletion
	if err := e.checkAndDeleteRecordings(event, eventID); err != nil {
		fmt.Printf("⚠️ Error checking recordings: %v\n", err)
	}

	return nil
}

// checkAndDeleteRecordings checks for recordings associated with the event and prompts for deletion
func (e *EventsCommand) checkAndDeleteRecordings(event *NostrEvent, eventID string) error {
	// Extract dtag from event tags
	dtag := ""
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "d" {
			dtag = tag[1]
			break
		}
	}
	
	if dtag == "" {
		fmt.Println("\n📁 No dtag found in event - cannot match recordings")
		return nil
	}
	
	eventTime := time.Unix(event.CreatedAt, 0)
	
	// Archive path where recordings are stored
	archivePath := "www/live/archive"
	
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		fmt.Println("\n📁 No archive directory found")
		return nil
	}
	
	var foundRecordings []string
	
	// Look for directories with pattern: date-dtag (e.g., "9-8-2025-315523")
	datePattern := eventTime.Format("1-2-2006") // e.g., "9-8-2025"
	expectedFolderPattern := datePattern + "-" + dtag
	
	err := filepath.Walk(archivePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue on errors
		}
		
		if info.IsDir() && info.Name() != "archive" { // Skip the root archive dir itself
			dirname := info.Name()
			// Check if directory name matches the expected pattern
			if dirname == expectedFolderPattern || strings.Contains(dirname, dtag) {
				foundRecordings = append(foundRecordings, path)
			}
		}
		return nil
	})
	
	if err != nil {
		return fmt.Errorf("error searching archive directory: %w", err)
	}
	
	if len(foundRecordings) == 0 {
		fmt.Println("\n📁 No recordings found for this stream")
		return nil
	}
	
	// Calculate total size of recordings
	var totalSize int64
	var recordingInfos []struct {
		path string
		size int64
	}
	
	for _, recording := range foundRecordings {
		size, err := calculateDirSize(recording)
		if err != nil {
			fmt.Printf("⚠️ Could not calculate size for %s: %v\n", recording, err)
			size = 0
		}
		totalSize += size
		recordingInfos = append(recordingInfos, struct {
			path string
			size int64
		}{recording, size})
	}

	fmt.Printf("\n📁 Found %d potential recording(s) (Total: %s):\n", len(foundRecordings), formatBytes(totalSize))
	for i, info := range recordingInfos {
		fmt.Printf("   %d. %s (%s)\n", i+1, info.path, formatBytes(info.size))
	}
	
	// Prompt user for deletion
	fmt.Print("\n🗑️  Delete these recordings too? (y/N): ")
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}
	
	response = strings.TrimSpace(strings.ToLower(response))
	if response == "y" || response == "yes" {
		fmt.Println("\n🗑️ Deleting recordings...")
		deleted := 0
		failed := 0
		
		for _, recording := range foundRecordings {
			if err := os.RemoveAll(recording); err != nil {
				fmt.Printf("   ❌ Failed to delete %s: %v\n", recording, err)
				failed++
			} else {
				fmt.Printf("   ✅ Deleted %s\n", recording)
				deleted++
			}
		}
		
		fmt.Printf("\n📊 Summary: %d deleted, %d failed\n", deleted, failed)
	} else {
		fmt.Println("📁 Recordings preserved")
	}
	
	return nil
}

// calculateDirSize calculates the total size of a directory
func calculateDirSize(dirPath string) (int64, error) {
	var size int64
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// formatBytes formats byte size into human readable format
func formatBytes(bytes int64) string {
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

// handleDeletions lists deletion requests sent
func (e *EventsCommand) handleDeletions(args []string) error {
	fmt.Println("🔍 Fetching your deletion requests...")
	
	grainClient, ok := e.nostrClient.(*nostr.GrainClient)
	if !ok || !grainClient.IsEnabled() {
		return fmt.Errorf("grain client not available or not enabled")
	}

	// Create filter for deletion events (kind 5)
	limit := 20
	limitPtr := &limit
	filter := nostrTypes.Filter{
		Kinds:   []int{5}, // NIP-09: Event Deletion
		Authors: []string{grainClient.GetUserSession().PublicKey},
		Limit:   limitPtr,
	}

	// Subscribe to get deletion events
	subscription, err := grainClient.Subscribe([]nostrTypes.Filter{filter}, nil)
	if err != nil {
		return fmt.Errorf("failed to create subscription: %w", err)
	}
	
	timeout := time.NewTimer(3 * time.Second)
	defer timeout.Stop()
	
	// Graceful cleanup - close subscription after collection is done
	defer func() {
		if subscription != nil {
			// Give a small delay for any pending messages to be processed
			time.Sleep(100 * time.Millisecond)
			subscription.Close()
		}
	}()

	var deletions []NostrEvent
	collecting := true
	
	for collecting {
		select {
		case event, ok := <-subscription.Events:
			if !ok {
				collecting = false
				break
			}
			
			deletions = append(deletions, NostrEvent{
				ID:        event.ID,
				PubKey:    event.PubKey,
				CreatedAt: event.CreatedAt,
				Kind:      event.Kind,
				Tags:      event.Tags,
				Content:   event.Content,
				Sig:       event.Sig,
			})
			
		case <-subscription.Done:
			collecting = false
			
		case <-timeout.C:
			collecting = false
		}
	}

	if len(deletions) == 0 {
		fmt.Println("📭 No deletion requests found")
		return nil
	}

	fmt.Printf("\n🗑️  Found %d deletion requests:\n\n", len(deletions))
	fmt.Printf("%-64s %-20s %-30s\n", "DELETION EVENT ID", "CREATED", "TARGET EVENT ID")
	fmt.Println(strings.Repeat("-", 120))

	for _, deletion := range deletions {
		created := time.Unix(deletion.CreatedAt, 0).Format("2006-01-02 15:04")
		
		// Extract target event ID from e tags
		targetID := ""
		for _, tag := range deletion.Tags {
			if len(tag) >= 2 && tag[0] == "e" {
				targetID = tag[1]
				break
			}
		}
		
		fmt.Printf("%-64s %-20s %-30s\n", deletion.ID, created, targetID)
	}

	return nil
}

// handleShow shows detailed event information
func (e *EventsCommand) handleShow(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing event ID")
	}

	eventID := args[0]
	fmt.Printf("🔍 Fetching event details: %s\n", eventID)

	event, err := e.fetchEventByID(eventID)
	if err != nil {
		return err
	}

	if event == nil {
		return fmt.Errorf("event not found: %s", eventID)
	}

	// Display detailed event information
	fmt.Printf("\n📺 EVENT DETAILS:\n\n")
	fmt.Printf("ID:          %s\n", event.ID)
	fmt.Printf("Kind:        %d\n", event.Kind)
	fmt.Printf("PubKey:      %s\n", event.PubKey)
	fmt.Printf("Created:     %s\n", time.Unix(event.CreatedAt, 0).Format("2006-01-02 15:04:05 MST"))
	fmt.Printf("Content:     %s\n", event.Content)

	fmt.Printf("\nTags:\n")
	for _, tag := range event.Tags {
		if len(tag) >= 2 {
			fmt.Printf("  %s: %s\n", tag[0], tag[1])
		}
	}

	fmt.Printf("\nRAW JSON:\n")
	jsonData, _ := json.MarshalIndent(event, "", "  ")
	fmt.Println(string(jsonData))

	return nil
}

// handlePublish publishes a new event
func (e *EventsCommand) handlePublish(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing event type (start|end|update)")
	}

	eventType := args[0]
	fmt.Printf("📡 Publishing %s event...\n", eventType)

	metadata := e.config.GetStreamMetadata()

	switch eventType {
	case "start":
		e.nostrClient.BroadcastStartEvent(metadata)
	case "end":
		e.nostrClient.BroadcastEndEvent(metadata)
	case "update":
		e.nostrClient.BroadcastUpdateEvent(metadata)
	default:
		return fmt.Errorf("unknown event type: %s (use: start|end|update)", eventType)
	}

	fmt.Printf("✅ %s event published successfully\n", strings.ToUpper(eventType))
	return nil
}

// fetchStreamEvents fetches stream events from Nostr relays
func (e *EventsCommand) fetchStreamEvents(limit int, statusFilter string, recent bool) ([]NostrEvent, error) {
	grainClient, ok := e.nostrClient.(*nostr.GrainClient)
	if !ok || !grainClient.IsEnabled() {
		return nil, fmt.Errorf("grain client not available or not enabled")
	}

	// Create filter for live streaming events (kind 30311)
	limitPtr := &limit
	filter := nostrTypes.Filter{
		Kinds:   []int{30311}, // NIP-53: Live Activities
		Authors: []string{grainClient.GetUserSession().PublicKey},
		Limit:   limitPtr,
	}
	
	// Add time filter if recent is requested
	if recent {
		since := time.Now().Add(-24 * time.Hour)
		filter.Since = &since
	}

	// Subscribe to get events
	subscription, err := grainClient.Subscribe([]nostrTypes.Filter{filter}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscription: %w", err)
	}
	
	// Use a shorter timeout to prevent blocking
	var events []NostrEvent
	timeout := time.NewTimer(3 * time.Second) // Reduced timeout
	defer timeout.Stop()

	// Start collecting events with timeout and proper cleanup
	eventCount := 0
	collecting := true
	
	// Graceful cleanup - close subscription after collection is done
	defer func() {
		if subscription != nil {
			// Give a small delay for any pending messages to be processed
			time.Sleep(100 * time.Millisecond)
			subscription.Close()
		}
	}()
	
	for collecting {
		select {
		case event, ok := <-subscription.Events:
			if !ok {
				// Channel closed, we're done
				collecting = false
				break
			}
			
			// Filter by status if specified
			if statusFilter != "" {
				status := ""
				for _, tag := range event.Tags {
					if len(tag) >= 2 && tag[0] == "status" {
						status = tag[1]
						break
					}
				}
				if status != statusFilter {
					continue
				}
			}
			
			// Convert grain event to our NostrEvent type
			nostrEvent := NostrEvent{
				ID:        event.ID,
				PubKey:    event.PubKey,
				CreatedAt: event.CreatedAt,
				Kind:      event.Kind,
				Tags:      event.Tags,
				Content:   event.Content,
				Sig:       event.Sig,
			}
			events = append(events, nostrEvent)
			eventCount++
			
			// If we've collected enough events, stop collecting
			if eventCount >= limit {
				collecting = false
			}
			
		case <-subscription.Done:
			// Subscription finished
			collecting = false
			
		case <-timeout.C:
			// Timeout reached
			collecting = false
		}
	}
	
	// Deduplicate by event ID
	seen := make(map[string]bool)
	var deduped []NostrEvent
	for _, event := range events {
		if !seen[event.ID] {
			seen[event.ID] = true
			deduped = append(deduped, event)
		}
	}
	
	// Sort events by date (newest first)
	sort.Slice(deduped, func(i, j int) bool {
		return deduped[i].CreatedAt > deduped[j].CreatedAt
	})
	
	return deduped, nil
}

// fetchEventByID fetches a specific event by ID
func (e *EventsCommand) fetchEventByID(eventID string) (*NostrEvent, error) {
	grainClient, ok := e.nostrClient.(*nostr.GrainClient)
	if !ok || !grainClient.IsEnabled() {
		return nil, fmt.Errorf("grain client not available or not enabled")
	}

	// Create filter for the specific event ID
	limitPtr := 1
	filter := nostrTypes.Filter{
		IDs:     []string{eventID},
		Kinds:   []int{30311}, // NIP-53: Live Activities
		Authors: []string{grainClient.GetUserSession().PublicKey},
		Limit:   &limitPtr,
	}

	// Subscribe to get the event
	subscription, err := grainClient.Subscribe([]nostrTypes.Filter{filter}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscription: %w", err)
	}
	
	timeout := time.NewTimer(3 * time.Second) // Reduced timeout
	defer timeout.Stop()
	
	// Graceful cleanup - close subscription after collection is done
	defer func() {
		if subscription != nil {
			// Give a small delay for any pending messages to be processed
			time.Sleep(100 * time.Millisecond)
			subscription.Close()
		}
	}()

	searching := true
	for searching {
		select {
		case event, ok := <-subscription.Events:
			if !ok {
				// Channel closed, event not found
				return nil, fmt.Errorf("event not found: %s", eventID)
			}
			
			// Convert grain event to our NostrEvent type
			nostrEvent := &NostrEvent{
				ID:        event.ID,
				PubKey:    event.PubKey,
				CreatedAt: event.CreatedAt,
				Kind:      event.Kind,
				Tags:      event.Tags,
				Content:   event.Content,
				Sig:       event.Sig,
			}
			return nostrEvent, nil
			
		case <-subscription.Done:
			// Subscription finished without finding event
			searching = false
			
		case <-timeout.C:
			// Timeout reached
			searching = false
		}
	}
	
	return nil, fmt.Errorf("event not found: %s", eventID)
}

// filterEventsByQuery filters events by search query
func (e *EventsCommand) filterEventsByQuery(events []NostrEvent, query string) []NostrEvent {
	var filtered []NostrEvent
	query = strings.ToLower(query)

	for _, event := range events {
		title := strings.ToLower(e.getEventTitle(event))
		summary := strings.ToLower(e.getEventSummary(event))
		content := strings.ToLower(event.Content)

		if strings.Contains(title, query) || 
		   strings.Contains(summary, query) || 
		   strings.Contains(content, query) {
			filtered = append(filtered, event)
		}
	}

	return filtered
}

// Helper functions to extract event information
func (e *EventsCommand) getEventStatus(event NostrEvent) string {
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "status" {
			return tag[1]
		}
	}
	return "unknown"
}

func (e *EventsCommand) getEventTitle(event NostrEvent) string {
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "title" {
			return tag[1]
		}
	}
	return "Untitled Stream"
}

func (e *EventsCommand) getEventSummary(event NostrEvent) string {
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "summary" {
			return tag[1]
		}
	}
	return ""
}

// NostrEvent represents a Nostr event - using the same structure as the main client
type NostrEvent = nostr.Event