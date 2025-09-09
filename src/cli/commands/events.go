package commands

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gnostream/src/config"
	"gnostream/src/nostr"
)

// EventsCommand handles Nostr event management
type EventsCommand struct {
	config      *config.Config
	nostrClient *nostr.Client
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
	fmt.Printf("%-16s %-10s %-20s %-30s\n", "EVENT ID", "STATUS", "CREATED", "TITLE")
	fmt.Println(strings.Repeat("-", 80))

	for _, event := range events {
		status := e.getEventStatus(event)
		created := time.Unix(event.CreatedAt, 0).Format("2006-01-02 15:04")
		title := e.getEventTitle(event)
		if len(title) > 28 {
			title = title[:28] + "..."
		}

		fmt.Printf("%-16s %-10s %-20s %-30s\n", 
			event.ID[:16]+"...", status, created, title)
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

	// Create and publish deletion event
	e.nostrClient.BroadcastDeletionEvent(eventID, "Deleted via gnostream CLI")

	fmt.Printf("✅ Event %s marked for deletion\n", eventID)
	fmt.Println("📡 Deletion request broadcast to all relays")

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
	// This is a simplified mock - in a real implementation, you'd query Nostr relays
	// For now, return empty slice as this requires implementing relay queries
	fmt.Println("⚠️  Note: Event fetching from relays not yet implemented")
	fmt.Println("💡 This feature requires implementing Nostr relay query functionality")
	
	return []NostrEvent{}, nil
}

// fetchEventByID fetches a specific event by ID
func (e *EventsCommand) fetchEventByID(eventID string) (*NostrEvent, error) {
	// This is a simplified mock - in a real implementation, you'd query Nostr relays
	fmt.Println("⚠️  Note: Event fetching from relays not yet implemented")
	return nil, fmt.Errorf("event fetching not yet implemented")
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

// NostrEvent represents a Nostr event (simplified structure)
type NostrEvent struct {
	ID        string     `json:"id"`
	PubKey    string     `json:"pubkey"`
	CreatedAt int64      `json:"created_at"`
	Kind      int        `json:"kind"`
	Tags      [][]string `json:"tags"`
	Content   string     `json:"content"`
	Sig       string     `json:"sig"`
}