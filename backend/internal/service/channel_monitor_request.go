package service

import "context"

// ChannelMonitorProbeHeader identifies health-check traffic independently of
// its model, prompt and API format. It only opts out of default Skill injection;
// gateway authentication, billing and model admission still apply.
const ChannelMonitorProbeHeader = "X-Sub2api-Channel-Monitor"
const ChannelMonitorProbeHeaderValue = "probe"

type channelMonitorProbeContextKey struct{}

func WithChannelMonitorProbe(ctx context.Context) context.Context {
	return context.WithValue(ctx, channelMonitorProbeContextKey{}, true)
}

func IsChannelMonitorProbe(ctx context.Context) bool {
	probe, _ := ctx.Value(channelMonitorProbeContextKey{}).(bool)
	return probe
}
