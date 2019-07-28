package aws

import (
	"encoding/xml"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
)

// Interface represents an AWS API compatible interface.
type Interface struct {
	region   string
	account  string
	proto    string
	addr     string
	codePath string
}

// New AWS API compatible interface to serve HTTP requests.
func New(region, account, proto, addr, codePath string) Interface {
	return Interface{
		region:   region,
		account:  account,
		proto:    proto,
		addr:     addr,
		codePath: codePath,
	}
}

func onAccessDenied(w http.ResponseWriter, res, reqID string) {

	msg := fmt.Sprintf("Access to the resource %s is denied.", res)
	err := ResponseErr{
		Details: Details{
			Type:      "Sender",
			Code:      "AccessDenied",
			Message:   msg,
			RequestID: reqID,
		},
	}
	writeErr(w, http.StatusForbidden, err)
}

func onLwsErr(w http.ResponseWriter, r Response) {

	w.WriteHeader(r.Status)
	enc := xml.NewEncoder(w)
	if err := enc.Encode(r.Err); err != nil {
		log.Errorln("Unexpected error encoding error message,", err)
	}
}

func onInvalidParameterValue(w http.ResponseWriter, srv, reqID string) {

	err := ResponseErr{
		Details: Details{
			Type:      "Sender",
			Code:      "InvalidParameterValue",
			Message:   "Invalid or unsupported service " + srv,
			RequestID: reqID,
		},
	}
	writeErr(w, http.StatusBadRequest, err)
}

func writeErr(w http.ResponseWriter, status int, err ResponseErr) {

	w.WriteHeader(status)
	enc := xml.NewEncoder(w)
	if err := enc.Encode(err); err != nil {
		log.Errorln("Unexpected error encoding error message,", err)
	}
}
