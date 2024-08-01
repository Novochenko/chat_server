package chat

type Message struct {
	Author string `json:"author"`
	Body   []byte `json:"body"`
}

func (m *Message) String() string {
	return m.Author + ": " + string(m.Body)
}
