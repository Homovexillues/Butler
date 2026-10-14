package notify

import "fmt"

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

func AllChannels() []Channel {
	return []Channel{
		ChannelSystem,
		ChannelMQTT,
		ChannelEmail,
		ChannelMessageBox,
	}
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

// ValidateChannels rejects empty and unknown channel configurations.
func ValidateChannels(channels []Channel) error {
	if len(channels) == 0 {
		return fmt.Errorf("notification channels are empty")
	}
	for i, ch := range channels {
		if _, ok := ParseChannel(string(ch)); !ok {
			return fmt.Errorf("channels[%d]: invalid channel %q", i, ch)
		}
	}
	return nil
}
