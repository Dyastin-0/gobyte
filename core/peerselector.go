package core

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
)

const (
	PAGESIZE = 25
)

var SamplePeers = map[string]*peer{
	"test": {
		name:      "test",
		data:      ":8080",
		lastHello: time.Now(),
	},
}

type PeerSelector struct {
	selected string
	peers    map[string]*peer
	Selected map[string]*peer
	filter   string
	page     int
	sortBy   string
}

func NewPeerSelector(peers map[string]*peer) *PeerSelector {
	if peers == nil {
		peers = make(map[string]*peer)
	}
	return &PeerSelector{
		peers:    peers,
		Selected: make(map[string]*peer),
		page:     0,
		sortBy:   "name",
	}
}

func (p *PeerSelector) filteredPeers() []*peer {
	var peerList []*peer

	for _, peer := range p.peers {
		peerList = append(peerList, peer)
	}

	if p.filter != "" {
		filtered := make([]*peer, 0)
		filterLower := strings.ToLower(p.filter)
		for _, peer := range peerList {
			if strings.Contains(strings.ToLower(peer.name), filterLower) ||
				strings.Contains(strings.ToLower(peer.data), filterLower) {
				filtered = append(filtered, peer)
			}
		}
		peerList = filtered
	}

	sort.Slice(peerList, func(i, j int) bool {
		switch p.sortBy {
		case "data":
			return peerList[i].data < peerList[j].data
		case "lastseen":
			return peerList[i].lastHello.After(peerList[j].lastHello)
		default:
			return strings.ToLower(peerList[i].name) < strings.ToLower(peerList[j].name)
		}
	})

	return peerList
}

func (p *PeerSelector) formatPeerOption(peer *peer) string {
	name := peer.name
	if len(name) > 20 {
		name = name[:17] + "..."
	}

	data := peer.data
	if len(data) > 25 {
		data = data[:22] + "..."
	}

	lastSeenStr := ""
	elapsed := 0
	if !peer.lastHello.IsZero() {
		elapsed = int(time.Since(peer.lastHello).Seconds())
		lastSeenStr = fmt.Sprintf("%ds", elapsed)
	}

	peerKey := peer.name
	selectedPrefix := ""
	if _, isSelected := p.Selected[peerKey]; isSelected {
		selectedPrefix = selectedStyle.Render("✓ ")
	}

	text := fmt.Sprintf("%s%-20s %-25s %s",
		selectedPrefix,
		name,
		data,
		lastSeenStr,
	)

	if time.Since(peer.lastHello) > HelloInterval {
		text = warningStyle.Render(text)
	}

	return text
}

func (p *PeerSelector) RunRecur() error {
	// Peers will update automatically, since we are using the map pointer
	// from the broadcaster, beautiful
	peers := p.filteredPeers()

	totalItems := len(peers)
	totalPages := (totalItems + PAGESIZE - 1) / PAGESIZE
	if totalPages == 0 {
		totalPages = 1
	}

	if p.page < 0 {
		p.page = 0
	}

	var options []huh.Option[string]

	filterText := "Filter peers"
	if p.filter != "" {
		filterText = fmt.Sprintf("Filter: '%s'", p.filter)
	}
	options = append(options, huh.NewOption(filterText, "filter"))

	if totalPages > 1 {
		pageInfo := fmt.Sprintf("Page %d of %d (%d peers)", p.page+1, totalPages, totalItems)
		options = append(options, huh.NewOption(pageStyle.Render(pageInfo), "page_info"))

		if p.page > 0 {
			options = append(options, huh.NewOption("<-", "prev_page"))
		}
		if p.page < totalPages-1 {
			options = append(options, huh.NewOption("->", "next_page"))
		}
	}

	start := p.page * PAGESIZE
	end := min(start+PAGESIZE, len(peers))

	for i := start; i < end; i++ {
		peer := peers[i]
		displayText := p.formatPeerOption(peer)
		options = append(options, huh.NewOption(displayText, peer.name))
	}

	options = append(options,
		huh.NewOption("All", "select_all"),
		huh.NewOption("Done", "done"),
		huh.NewOption("Cancel", "cancel"),
	)

	title := fmt.Sprintf("Choose peers (%d selected):", len(p.Selected))
	if p.filter != "" {
		title += fmt.Sprintf(" [Filter: %s]", p.filter)
	}

	form := huh.NewSelect[string]().
		Title(title).
		Options(options...).
		Value(&p.selected).
		Height(20)

	err := form.Run()
	if err != nil {
		return err
	}

	switch p.selected {
	case "cancel":
		return ErrCanceled
	case "done":
		return nil
	case "filter":
		return p.Filter()
	case "prev_page":
		p.page--
		return p.RunRecur()
	case "next_page":
		p.page++
		return p.RunRecur()
	case "select_all":
		return p.SelectAll()
	default:
		p.TogglePeer(p.selected)
		return p.RunRecur()
	}
}

func (p *PeerSelector) Filter() error {
	var newFilter string

	form := huh.NewInput().
		Title("Filter peers (by ID or address):").
		Value(&newFilter).
		Placeholder(p.filter)

	err := form.Run()
	if err != nil {
		return p.RunRecur()
	}

	p.filter = strings.TrimSpace(newFilter)
	p.page = 0
	return p.RunRecur()
}

func (p *PeerSelector) TogglePeer(peerName string) {
	peer, exists := p.peers[peerName]
	if !exists {
		return
	}

	if _, isSelected := p.Selected[peerName]; isSelected {
		delete(p.Selected, peerName)
	} else {
		p.Selected[peerName] = peer
	}
}

func (p *PeerSelector) SelectAll() error {
	filteredPeers := p.filteredPeers()
	for _, peer := range filteredPeers {
		if _, ok := p.Selected[peer.name]; ok {
			delete(p.Selected, peer.name)
		} else {
			p.Selected[peer.name] = peer
		}
	}
	return p.RunRecur()
}

func (p *PeerSelector) ClearSelection() {
	p.Selected = make(map[string]*peer)
}

func (p *PeerSelector) GetSelectedPeers() []*peer {
	peers := make([]*peer, 0, len(p.Selected))
	for _, peer := range p.Selected {
		peers = append(peers, peer)
	}

	sort.Slice(peers, func(i, j int) bool {
		return strings.ToLower(peers[i].name) < strings.ToLower(peers[j].name)
	})

	return peers
}

func (p *PeerSelector) GetSelectedNames() []string {
	names := make([]string, 0, len(p.Selected))
	for name := range p.Selected {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (p *PeerSelector) UpdatePeers(peers map[string]*peer) {
	p.peers = peers
	for name := range p.Selected {
		if _, exists := p.peers[name]; !exists {
			delete(p.Selected, name)
		}
	}
}

func (p *PeerSelector) GetTotalCount() int {
	return len(p.peers)
}
