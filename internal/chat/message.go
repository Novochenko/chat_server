package chat

type Message struct {
	Client *Client
	Author string `json:"author"`
	Body   []byte `json:"body"`
}

func NewMessage(client *Client) Message {
	return Message{
		Client: client,
		Body:   make([]byte, 256),
	}
}

func (m *Message) String() string {
	return m.Author + ": " + string(m.Body)
}
