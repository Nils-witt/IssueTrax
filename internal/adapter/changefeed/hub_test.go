package changefeed_test

import (
	"context"
	"testing"

	"nilswitt.dev/issuetrax/internal/adapter/changefeed"
	"nilswitt.dev/issuetrax/internal/core/domain"
)

func TestHubDeliversToEverySubscriber(t *testing.T) {
	t.Parallel()

	hub := changefeed.New()
	a := hub.Subscribe(t.Context())
	b := hub.Subscribe(t.Context())

	want := domain.Change{Kind: domain.ChangeIssue, Project: "TRAX", Issue: 7}
	hub.Notify(t.Context(), want)

	for name, ch := range map[string]<-chan domain.Change{"a": a, "b": b} {
		if got := <-ch; got != want {
			t.Errorf("subscriber %s got %+v, want %+v", name, got, want)
		}
	}
}

func TestHubClosesOnContextDone(t *testing.T) {
	t.Parallel()

	hub := changefeed.New()
	ctx, cancel := context.WithCancel(t.Context())
	ch := hub.Subscribe(ctx)

	cancel()

	if _, ok := <-ch; ok {
		t.Fatal("received a change, want closed channel")
	}

	// Notifying after the subscriber left must not panic on a closed channel.
	hub.Notify(t.Context(), domain.Change{Kind: domain.ChangeProject, Project: "TRAX"})
}

func TestHubDropsSlowSubscriber(t *testing.T) {
	t.Parallel()

	hub := changefeed.New()
	slow := hub.Subscribe(t.Context())

	change := domain.Change{Kind: domain.ChangeProject, Project: "TRAX"}
	for range 1000 {
		hub.Notify(t.Context(), change)
	}

	n := 0
	for range slow {
		n++
	}

	if n == 0 || n >= 1000 {
		t.Fatalf("slow subscriber got %d changes before being dropped, want some but not all", n)
	}
}
