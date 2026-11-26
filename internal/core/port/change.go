package port

import (
	"context"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

// ChangeNotifier is the driven port services announce committed changes
// through. Notify must not block: delivery is best effort, and listeners
// that miss changes are expected to reload everything when they resume.
type ChangeNotifier interface {
	Notify(ctx context.Context, change domain.Change)
}

// ChangeFeed is the driving port inbound adapters follow changes with
// (e.g. to push them to browsers).
type ChangeFeed interface {
	// Subscribe returns a channel receiving every change notified after the
	// call. The channel is closed once ctx is done, or early when the
	// subscriber falls too far behind; either way the subscriber has missed
	// changes from then on.
	Subscribe(ctx context.Context) <-chan domain.Change
}
