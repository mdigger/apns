package apns

// ClientsPool manages a pool of Clients.
//
// The APNs server allows multiple concurrent streams for each connection. The
// exact number of streams is based on the authentication method used (i.e.
// provider certificate or token) and the server load, so do not assume a
// specific number of streams. When you connect to APNs without a provider
// certificate, only one stream is allowed on the connection until you send a
// push message with valid token.
type ClientsPool struct {
	notifications chan Notification
}

// Response from sending a notification.
type Response struct {
	Token string // Unique device token for the app.
	ID    string // A canonical UUID that identifies the notification
	Error error  // Error describes the error response from the server
}

// Pool wraps a client with a queue for sending notifications asynchronously.
//
// You can establish multiple connections to APNs servers to improve
// performance. When you send a large number of remote notifications, distribute
// them across connections to several server endpoints. This improves
// performance, compared to using a single connection, by letting you send
// remote notifications faster and by letting APNs deliver them faster.
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
