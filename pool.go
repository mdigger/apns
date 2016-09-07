package apns

// ClientsPool manages a pool of Clients.
type ClientsPool struct {
	*Client       // APNS Client
	notifications chan Notification
}

// Response from sending a notification.
type Response struct {
	Token string // Unique device token for the app.
	ID    string // A canonical UUID that identifies the notification
	Error error  // Error describes the error response from the server
}

// Pool wraps a client with a queue for sending notifications asynchronously.
func (c *Client) Pool(workers uint, responses chan<- Response) *ClientsPool {
	notifications := make(chan Notification)
	// startup workers to send notifications
	for i := uint(0); i < workers; i++ {
		go func() {
			for n := range notifications {
				id, err := c.Push(n)
				if responses != nil {
					responses <- Response{n.Token, id, err}
				}
			}
		}()
	}
	return &ClientsPool{
		Client:        c,
		notifications: notifications,
	}
}

// Push queues a notification to the APN service.
func (p *ClientsPool) Push(n Notification, tokens ...string) {
	for _, token := range tokens {
		n.Token = token
		p.notifications <- n
	}
}

// Close the channels for notifications and Responses and shutdown workers.
// You should only call this after all responses have been received.
func (p *ClientsPool) Close() {
	close(p.notifications)
}
