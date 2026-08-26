package notify

type Channel string

const (
	ChannelSystem     Channel = "system"
	ChannelMQTT       Channel = "mqtt"
	ChannelEmail      Channel = "email"
	ChannelMessageBox Channel = "messagebox"
)

func (c Channel) String() string {
	return string(c)
}

func ParseChannel(value string) (Channel, bool) {
	channel := Channel(value)
	switch channel {
	case ChannelSystem, ChannelMQTT, ChannelEmail, ChannelMessageBox:
		return channel, true
	default:
		return "", false
	}
}
