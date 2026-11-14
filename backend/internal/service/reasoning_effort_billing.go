package service

import "context"

// reasoningEffortForBilling selects the effort used for customer cost and quota
// multipliers. It does not change the forwarded effort recorded in usage logs or
// the effort used by independent account-cost rules.
func (r *ModelPricingResolver) reasoningEffortForBilling(ctx context.Context, apiKey *APIKey, requested, forwarded *string) string {
	if r == nil || r.channelService == nil || apiKey == nil {
		return optionalStringValue(forwarded)
	}
	groupID := apiKey.GroupID
	if apiKey.Group != nil {
		groupID = &apiKey.Group.ID
	}
	if groupID == nil {
		return optionalStringValue(forwarded)
	}
	cache, err := r.channelService.loadCache(ctx)
	if err != nil {
		return optionalStringValue(forwarded)
	}
	// Only read the immutable cache snapshot; billing needs no full channel clone.
	channel := cache.channelByGroupID[*groupID]
	if channel == nil || !channel.IsActive() || channel.ReasoningEffortSource != ReasoningEffortSourceRequested {
		return optionalStringValue(forwarded)
	}
	if optionalStringValue(requested) == "" {
		requested = RequestedReasoningEffortFromContext(ctx)
	}
	// Older adapters may not capture the requested effort; retain their existing
	// behavior rather than silently dropping an otherwise configured multiplier.
	return optionalStringValue(coalesceRequestedReasoningEffort(requested, forwarded))
}
