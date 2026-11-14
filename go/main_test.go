package main

import "testing"

func TestGetDealIDReturnsLastIDInLink(t *testing.T) {
	got, ok := getDealID("https://example.test/deal-123/category-456/")
	if !ok {
		t.Fatal("expected a deal ID")
	}
	if got != 456 {
		t.Fatalf("getDealID() = %d, want 456", got)
	}
}

func TestParseRFDDealsFiltersSourcesAndPreviouslySeenIDs(t *testing.T) {
	html := `
		<ul>
			<li class="topic-card topic">
				<a class="topic-card-info" href="/deals/new-200/">
					<h3 class="thread_title"> New offer </h3>
					<div class="dealer_name">Amazon Canada</div>
				</a>
			</li>
			<li class="topic-card topic">
				<a class="topic-card-info" href="/deals/seen-100/">
					<h3 class="thread_title">Already seen</h3>
					<div class="dealer_name">Amazon</div>
				</a>
			</li>
			<li class="topic-card topic">
				<a class="topic-card-info" href="/deals/ignored-300/">
					<h3 class="thread_title">Other store</h3>
					<div class="dealer_name">Unknown store</div>
				</a>
			</li>
		</ul>`
	pushed := map[int64]struct{}{100: {}}
	settings := config{
		RFDSources:     []string{"Amazon"},
		DealLinkPrefix: "https://forums.example.test",
	}

	got, err := parseRFDDeals(html, settings, pushed)
	if err != nil {
		t.Fatalf("parseRFDDeals() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("parseRFDDeals() returned %d deals, want 1", len(got))
	}
	if got[0].Title != "New offer" || got[0].Link != "https://forums.example.test/deals/new-200/" || got[0].Source != "Amazon Canada" {
		t.Fatalf("parseRFDDeals() returned unexpected deal: %+v", got[0])
	}
	if _, exists := pushed[200]; !exists {
		t.Fatal("new deal ID was not recorded")
	}
}

func TestKeepLatestDealIDsRetainsHighestIDs(t *testing.T) {
	deals := map[int64]struct{}{
		1: {}, 2: {}, 3: {}, 4: {},
	}

	keepLatestDealIDs(deals, 2)

	if len(deals) != 2 {
		t.Fatalf("kept %d deals, want 2", len(deals))
	}
	if _, exists := deals[4]; !exists {
		t.Fatal("highest ID 4 was removed")
	}
	if _, exists := deals[3]; !exists {
		t.Fatal("second-highest ID 3 was removed")
	}
}
