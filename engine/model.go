package engine

// import (
// 	"crypto/subtle"
// 	"crypto/tls"
// 	"fmt"
// 	"net/http"
// 	"net/url"
// 	"strings"
// 	"time"
//
// 	"github.com/mailgun/oxy/memmetrics"
// 	"github.com/mailgun/oxy/stream"
// 	"github.com/mailgun/route"
// )

type NotFoundError struct {
	Message string
}

func (n *NotFoundError) Error() string {
	if n.Message != "" {
		return n.Message
	} else {
		return "Object not found"
	}
}

type InvalidFormatError struct {
	Message string
}

func (n *InvalidFormatError) Error() string {
	if n.Message != "" {
		return n.Message
	} else {
		return "Invalid format"
	}
}

type AlreadyExistsError struct {
	Message string
}

func (n *AlreadyExistsError) Error() string {
	return n.Message
}
