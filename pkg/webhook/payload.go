package webhook

import "time"

// Payload represents the data sent as the payload of an event.
type Payload struct {
	Type         string      `json:"type"`
	Token        string      `json:"token"`
	TokenExpires time.Time   `json:"tokenExpires"`
	Body         interface{} `json:"body"`
	AppID        int         `json:"-"`
	InstID       int         `json:"-"`
}
