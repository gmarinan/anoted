package level

// PCMFeed receives PCM captured by the recorder so level meters do not need a
// second WASAPI client on the same device while recording.
type PCMFeed interface {
	FeedSystemPCM(pcm []byte)
	FeedMicPCM(pcm []byte)
}

// PCMFeedConfig extends PCMFeed with capture format hints for level analysis.
type PCMFeedConfig interface {
	PCMFeed
	SetFeedChannels(int)
}
